package marketdata

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"bytes"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/klauspost/compress/gzip"
)

// -----------------------------------------------------------------------------
// Performance-optimised connection pool for bulk COPY
//
// Note: Connection pool creation includes retry logic to handle transient
// database connectivity issues (e.g., during database restarts, maintenance
// windows, or Kubernetes pod rescheduling). The retry wrapper will attempt
// up to 5 times with exponential backoff for connection timeouts and dial
// errors before failing the job.
// -----------------------------------------------------------------------------

type bulkLoadPool struct {
	pool *pgxpool.Pool
}

func newBulkLoadPool(ctx context.Context, db *pgxpool.Pool) (*bulkLoadPool, error) {
	cfg := db.Config()
	cfg.MinConns = int32(copyWorkerCount)
	cfg.MaxConns = int32(copyWorkerCount)

	// Apply tuning parameters to every connection via AfterConnect hook.
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		memMB := 4096 / copyWorkerCount
		if memMB < 64 {
			memMB = 64
		}
		settings := []string{
			"SET synchronous_commit = off",
			fmt.Sprintf("SET maintenance_work_mem = '%dMB'", memMB),
			"SET work_mem = '256MB'",
			"SET temp_buffers = '256MB'",
		}
		for _, s := range settings {
			if _, err := conn.Exec(ctx, s); err != nil {
				return fmt.Errorf("set %s: %w", s, err)
			}
		}

		// Disable or raise the tuple-decompression guardrail for this session so large UPSERTs
		// that touch compressed chunks do not error with:
		//   ERROR: tuple decompression limit exceeded by operation (SQLSTATE 53400)
		// If the running TimescaleDB version does not expose the GUC the command will error
		// with "unrecognized configuration parameter". In that case we log the event and
		// continue; for any other error we still abort pool creation.
		if _, err := conn.Exec(ctx, "SET timescaledb.max_tuples_decompressed_per_dml_transaction = 0"); err != nil {
			if strings.Contains(err.Error(), "unrecognized configuration parameter") {
				log.Printf("ℹ️  Parameter timescaledb.max_tuples_decompressed_per_dml_transaction not available: %v", err)
			} else {
				return fmt.Errorf("set timescaledb.max_tuples_decompressed_per_dml_transaction: %w", err)
			}
		}
		return nil
	}

	pool, err := pgxpool.ConnectConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create bulk load pool: %w", err)
	}

	return &bulkLoadPool{pool: pool}, nil
}

// newBulkLoadPoolWithRetry wraps newBulkLoadPool with retry logic for transient connection failures.
// This handles cases where the database is temporarily unavailable (restarts, maintenance, etc.).
func newBulkLoadPoolWithRetry(ctx context.Context, db *pgxpool.Pool) (*bulkLoadPool, error) {
	const maxAttempts = 5
	const baseDelay = 2 * time.Second
	const maxDelay = 30 * time.Second

	var lastErr error
	delay := baseDelay

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Check if context was cancelled before attempting
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		blp, err := newBulkLoadPool(ctx, db)
		if err == nil {
			if attempt > 1 {
				log.Printf("✅ Bulk load pool created successfully on attempt %d/%d", attempt, maxAttempts)
			}
			return blp, nil
		}

		lastErr = err

		// Check if this looks like a transient connection error
		isTransientError := strings.Contains(err.Error(), "dial error") ||
			strings.Contains(err.Error(), "i/o timeout") ||
			strings.Contains(err.Error(), "connection refused") ||
			strings.Contains(err.Error(), "timeout")

		// If it's not a transient error, fail immediately
		if !isTransientError {
			return nil, fmt.Errorf("create bulk load pool: %w", err)
		}

		// Don't retry on the last attempt
		if attempt == maxAttempts {
			break
		}

		log.Printf("⚠️ Bulk load pool creation failed (attempt %d/%d): %v - retrying in %v",
			attempt, maxAttempts, err, delay)

		// Sleep with context cancellation check
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}

		// Exponential backoff with cap
		delay *= 2
		if delay > maxDelay {
			delay = maxDelay
		}
	}

	return nil, fmt.Errorf("create bulk load pool failed after %d attempts: %w", maxAttempts, lastErr)
}

func (blp *bulkLoadPool) Close() { blp.pool.Close() }

func (blp *bulkLoadPool) AcquireFunc(ctx context.Context, fn func(*pgxpool.Conn) error) error {
	return blp.pool.AcquireFunc(ctx, fn)
}

// -----------------------------------------------------------------------------
// Streaming CSV reader that concatenates multiple gzip files
// -----------------------------------------------------------------------------

type batchedCSVReader struct {
	ctx    context.Context
	files  []string
	s3c    *s3.Client
	bucket string

	current io.Reader
	fileIdx int
	header  []byte
	hasRead bool

	gz   *gzip.Reader
	body io.ReadCloser

	cancel context.CancelFunc // cancels the request context once the body is closed

	// Accumulated time spent fetching objects from S3. This allows callers to
	// break down wall-clock time between network I/O (download) and database
	// insertion work.
	fetchDur time.Duration
}

func newBatchedCSVReader(ctx context.Context, s3c *s3.Client, bucket string, files []string) *batchedCSVReader {
	return &batchedCSVReader{ctx: ctx, files: files, s3c: s3c, bucket: bucket}
}

func (b *batchedCSVReader) Read(p []byte) (int, error) {
	for {
		// If we have an active reader, consume from it first.
		if b.current != nil {
			n, err := b.current.Read(p)
			if err == nil || err != io.EOF {
				// Successful read or real error (not EOF).
				return n, err
			}

			// Clean up after hitting EOF on current file.
			if b.gz != nil {
				_ = b.gz.Close()
				b.gz = nil
			}
			if b.body != nil {
				_ = b.body.Close()
				b.body = nil
			}
			if b.cancel != nil {
				b.cancel()
				b.cancel = nil
			}
			b.current = nil
			// Loop to open next file.
		}

		// No more files left.
		if b.fileIdx >= len(b.files) {
			return 0, io.EOF
		}

		file := b.files[b.fileIdx]
		b.fileIdx++

		// Measure the time spent establishing the object stream from S3.
		fetchStart := time.Now()
		resp, cancelFunc, err := getS3ObjectWithRetry(b.ctx, b.s3c, b.bucket, file, 3)
		b.fetchDur += time.Since(fetchStart)
		if err != nil {
			return 0, fmt.Errorf("get object %s: %w", file, err)
		}

		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
			return 0, fmt.Errorf("gzip reader %s: %w", file, err)
		}

		b.gz = gz
		b.body = resp.Body
		b.cancel = cancelFunc
		reader := bufio.NewReader(gz)

		if !b.hasRead {
			headerLine, err := reader.ReadString('\n')
			if err != nil {
				gz.Close()
				resp.Body.Close()
				return 0, fmt.Errorf("read header: %w", err)
			}
			mapped := headerLine
			if strings.Contains(headerLine, "window_start") {
				mapped = strings.Replace(headerLine, "window_start", "timestamp", 1)
			}
			b.header = []byte(mapped)
			b.hasRead = true
			b.current = io.MultiReader(bytes.NewReader(b.header), reader)
		} else {
			if _, err = reader.ReadString('\n'); err != nil {
				gz.Close()
				resp.Body.Close()
				return 0, fmt.Errorf("skip header: %w", err)
			}
			b.current = reader
		}
		// Loop will now read from the new current reader.
	}
}

// FetchDuration returns the total time spent fetching objects from S3 so far.
func (b *batchedCSVReader) FetchDuration() time.Duration {
	return b.fetchDur
}

// -----------------------------------------------------------------------------
// Pipeline worker & dispatcher
// -----------------------------------------------------------------------------

// Result status used by workers to report back to the aggregator.
type resultStatus int

const (
	resultLoaded resultStatus = iota
	resultFailed
)

type workerResult struct {
	day    time.Time
	status resultStatus
	reason string
}

type pipelineWorker struct {
	s3c       *s3.Client
	bucket    string
	table     string
	timeframe string
	pool      *bulkLoadPool
	collector *failedCollector
	resultCh  chan<- workerResult // channel to report per-file outcomes
	skipped   *int64
}

func newPipelineWorker(s3c *s3.Client, bucket, table, timeframe string, pool *bulkLoadPool, fc *failedCollector, resCh chan<- workerResult, skipped *int64) *pipelineWorker {
	return &pipelineWorker{s3c: s3c, bucket: bucket, table: table, timeframe: timeframe, pool: pool, collector: fc, resultCh: resCh, skipped: skipped}
}

func (pw *pipelineWorker) processFiles(ctx context.Context, files []string) error {
	if len(files) == 0 {
		return nil
	}

	reader := newBatchedCSVReader(ctx, pw.s3c, pw.bucket, files)

	batchErr := pw.pool.AcquireFunc(ctx, func(c *pgxpool.Conn) error {
		pgc := c.Conn().PgConn()

		// -----------------------------------------------------------------
		// Per-connection staging table setup
		// -----------------------------------------------------------------
		stageTable := fmt.Sprintf("%s_stage_%d", pw.table, pgc.PID())

		// Ensure the temporary staging table is removed once this connection finishes.
		// We drop it *before* the function returns so it is cleaned up even if the
		// job crashes between batches.
		defer func() {
			if _, err := c.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", stageTable)); err != nil {
				log.Printf("warning: drop staging table %s: %v", stageTable, err)
			}
		}()

		// Staging table needs a bigint timestamp column because Polygon CSV files
		// contain Unix *nanosecond* epoch values. The main hypertables now store
		// a proper timestamptz, so we cannot use LIKE <main> anymore – instead
		// define the minimal schema explicitly with a bigint timestamp column
		// and no constraints.
		createStageSQL := fmt.Sprintf(`CREATE UNLOGGED TABLE IF NOT EXISTS %s (
			ticker        text,
			volume        numeric,
			open          numeric,
			close         numeric,
			high          numeric,
			low           numeric,
			"timestamp"  bigint     NOT NULL,
			transactions  integer
		)`, stageTable)
		if _, err := c.Exec(ctx, createStageSQL); err != nil {
			return fmt.Errorf("create staging table: %w", err)
		}

		// Step 1: COPY into staging table.
		copySQL := fmt.Sprintf("COPY %s(ticker, volume, open, close, high, low, \"timestamp\", transactions) FROM STDIN WITH (FORMAT csv, HEADER true)", stageTable)
		if _, err := pgc.CopyFrom(ctx, reader, copySQL); err != nil {
			return err
		}

		// Count and log records with null/empty tickers for monitoring
		var nullTickerCount int64
		countSQL := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE ticker IS NULL OR ticker = ''`, stageTable)
		if err := c.QueryRow(ctx, countSQL).Scan(&nullTickerCount); err != nil {
			log.Printf("warning: failed to count null tickers in %s: %v", stageTable, err)
		} else if nullTickerCount > 0 {
			log.Printf("⚠️  Filtering out %d records with null/empty tickers from batch", nullTickerCount)
		}

		// Step 2: Upsert into main table.
		upsertSQL := fmt.Sprintf(`INSERT INTO %s (ticker, volume, open, close, high, low, "timestamp", transactions)
SELECT ticker, volume, open, close, high, low,
       to_timestamp("timestamp"::double precision / 1000000000) AT TIME ZONE 'UTC',
       transactions FROM %s
WHERE ticker IS NOT NULL AND ticker != ''
ON CONFLICT (ticker, "timestamp") DO UPDATE SET
    volume        = EXCLUDED.volume,
    open          = EXCLUDED.open,
    close         = EXCLUDED.close,
    high          = EXCLUDED.high,
    low           = EXCLUDED.low,
    transactions  = EXCLUDED.transactions`, pw.table, stageTable)

		if _, err := c.Exec(ctx, upsertSQL); err != nil {
			return fmt.Errorf("upsert into %s: %w", pw.table, err)
		}

		// Step 3: Clear staging table for next batch.
		if _, err := c.Exec(ctx, fmt.Sprintf("TRUNCATE %s", stageTable)); err != nil {
			return fmt.Errorf("truncate stage: %w", err)
		}

		return nil
	})

	if batchErr == nil {
		for _, f := range files {
			if day, err := parseDayFromKey(f); err == nil {
				pw.resultCh <- workerResult{day: day, status: resultLoaded}
			}
		}
		return nil
	}

	var pgErr *pgconn.PgError
	if errors.As(batchErr, &pgErr) && pgErr.Code == "23505" {
		for _, f := range files {
			if day, err := parseDayFromKey(f); err == nil {
				pw.resultCh <- workerResult{day: day, status: resultLoaded}
			}
		}
		return nil
	}

	var finalErr error
	for _, file := range files {
		if err := copyObject(ctx, pw.pool.pool, pw.s3c, pw.bucket, file, pw.table); err != nil {
			var fe *pgconn.PgError
			if errors.As(err, &fe) {
				code := fe.Code
				if code == "22P04" || code == "57014" {
					atomic.AddInt64(pw.skipped, 1)
					if pw.collector != nil {
						if day, errDay := parseDayFromKey(file); errDay == nil {
							pw.collector.Add(failedFile{Day: day, Timeframe: pw.timeframe, Reason: fe.Message})
							pw.resultCh <- workerResult{day: day, status: resultFailed, reason: fe.Message}
						}
					}
					continue
				}
			}
			// Gracefully handle truncated or corrupted gzip files (unexpected EOF).
			if errors.Is(err, io.ErrUnexpectedEOF) || strings.Contains(err.Error(), "unexpected EOF") {
				log.Printf("warning: skipping %s due to gzip error: %v", file, err)
				atomic.AddInt64(pw.skipped, 1)
				if pw.collector != nil {
					if day, errDay := parseDayFromKey(file); errDay == nil {
						pw.collector.Add(failedFile{Day: day, Timeframe: pw.timeframe, Reason: err.Error()})
						pw.resultCh <- workerResult{day: day, status: resultFailed, reason: err.Error()}
					}
				}
				continue
			}
			finalErr = err
			if pw.collector != nil {
				if day, errDay := parseDayFromKey(file); errDay == nil {
					pw.collector.Add(failedFile{Day: day, Timeframe: pw.timeframe, Reason: err.Error()})
					pw.resultCh <- workerResult{day: day, status: resultFailed, reason: err.Error()}
				}
			}
		} else {
			if day, err := parseDayFromKey(file); err == nil {
				pw.resultCh <- workerResult{day: day, status: resultLoaded}
			}
		}
	}
	return finalErr
}

// processFilesWithPipeline is the public dispatcher that fans out work to COPY workers.
func processFilesWithPipeline(ctx context.Context, s3c *s3.Client, bucket string, keys []string, table string, timeframe string, pool *bulkLoadPool, fc *failedCollector, processed *int64, skipped *int64, total int64, progressInterval int64, db *pgxpool.Pool, initialCutoff time.Time) (time.Time, error) {
	if len(keys) == 0 {
		return time.Time{}, nil
	}

	startTime := time.Now()

	// Build list of unique days for the tracker.
	uniq := make(map[time.Time]struct{})
	for _, k := range keys {
		if d, err := parseDayFromKey(k); err == nil {
			uniq[d] = struct{}{}
		}
	}
	var days []time.Time
	for d := range uniq {
		days = append(days, d)
	}

	tracker := newDayStatusTracker(days, initialCutoff)

	// Channel for per-file results.
	resultCh := make(chan workerResult, 100)

	// Error channel to receive first fatal error.
	errCh := make(chan error, 1)

	// Context to allow cancellation on first error.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Aggregator goroutine – persists progress and failed files periodically.
	aggDone := make(chan struct{})
	var finalCutoff time.Time
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		persisted := initialCutoff

		flush := func() {
			cut := tracker.CurrentCutoff()
			// Persist failed files collected so far.
			if failed := fc.PopAll(); len(failed) > 0 {
				if err := storeFailedFiles(ctx, db, failed); err != nil {
					select {
					case errCh <- err:
					default:
					}
					cancel()
					return
				}
			}

			// Persist last_loaded_at if we can move the cut-off.
			if cut.After(persisted) {
				if err := setLastLoadedAt(ctx, db, timeframe, cut); err != nil {
					select {
					case errCh <- err:
					default:
					}
					cancel()
					return
				}
				persisted = cut

				// Manually compress chunks in background
				now := time.Now().UTC()
				var minDuration time.Duration
				if timeframe == "1-minute" {
					minDuration = 7 * 24 * time.Hour
				} else if timeframe == "1-day" {
					minDuration = 60 * 24 * time.Hour
				}
				recentSafe := now.Add(-minDuration)
				eff := cut
				if cut.After(recentSafe) {
					eff = recentSafe
				}
				go func(e time.Time) {
					if err := compressOldChunks(context.Background(), db, table, e); err != nil {
						log.Printf("periodic compression failed for %s: %v", table, err)
					}
				}(eff)
			}
		}

		for {
			select {
			case r, ok := <-resultCh:
				if !ok {
					flush()
					finalCutoff = persisted
					close(aggDone)
					return
				}
				before := tracker.CurrentCutoff()
				if r.status == resultLoaded {
					tracker.MarkLoaded(r.day)
				} else {
					tracker.MarkFailed(r.day)
				}
				// If the guaranteed cutoff advanced, flush immediately.
				if tracker.CurrentCutoff().After(before) {
					flush()
				}
			case <-ticker.C:
				flush()
			case <-ctx.Done():
				flush()
				finalCutoff = persisted
				close(aggDone)
				return
			}
		}
	}()

	// Prepare batches of keys.
	batchSize := getBatchSize(table)
	batches := make([][]string, 0, (len(keys)+batchSize-1)/batchSize)
	for i := 0; i < len(keys); i += batchSize {
		end := i + batchSize
		if end > len(keys) {
			end = len(keys)
		}
		batches = append(batches, keys[i:end])
	}

	batchCh := make(chan []string)
	var wg sync.WaitGroup

	// Worker goroutines.
	for i := 0; i < copyWorkerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker := newPipelineWorker(s3c, bucket, table, timeframe, pool, fc, resultCh, skipped)
			for {
				select {
				case <-ctx.Done():
					return
				case batch, ok := <-batchCh:
					if !ok {
						return
					}
					if err := worker.processFiles(ctx, batch); err != nil {
						select {
						case errCh <- err:
						default:
						}
						cancel()
						return
					}
					n := atomic.AddInt64(processed, int64(len(batch)))
					if n%progressInterval == 0 || n == total {
						elapsed := time.Since(startTime)
						remainingFiles := total - n
						var estRemaining time.Duration
						if n > 0 {
							estRemaining = time.Duration(float64(elapsed) * float64(remainingFiles) / float64(n))
						}
						log.Printf("%s progress: %d/%d processed | elapsed %v | est remaining %v", table, n, total, elapsed.Truncate(time.Second), estRemaining.Truncate(time.Second))
					}
				}
			}
		}()
	}

	// Producer goroutine to feed batches.
	go func() {
		for _, b := range batches {
			select {
			case <-ctx.Done():
				return
			case batchCh <- b:
			}
		}
		close(batchCh)
	}()

	// Wait for workers then close result channel.
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	select {
	case err := <-errCh:
		return time.Time{}, err
	case <-aggDone:
		return finalCutoff, nil
	}
}

// -----------------------------------------------------------------------------
// S3 retry helpers
// -----------------------------------------------------------------------------

// getS3ObjectWithRetry downloads an S3 object with exponential backoff retry on rate limits.
// It returns both the object output and the CancelFunc belonging to the per-request timeout
// context. Callers **must** invoke the returned cancel function after they are done reading
// resp.Body (ideally in the same defer that closes the body) to release resources associated
// with the timer.
func getS3ObjectWithRetry(ctx context.Context, s3c *s3.Client, bucket, key string, maxRetries int) (*s3.GetObjectOutput, context.CancelFunc, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 2^attempt seconds, capped at 30 seconds
			backoffDuration := time.Duration(1<<attempt) * time.Second
			if backoffDuration > 30*time.Second {
				backoffDuration = 30 * time.Second
			}

			log.Printf("S3 GetObject rate limited for %s (attempt %d/%d), backing off for %v...", key, attempt, maxRetries, backoffDuration)

			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			case <-time.After(backoffDuration):
			}
		}

		ctxObj, cancel := context.WithTimeout(ctx, 15*time.Minute)
		resp, err := s3c.GetObject(ctxObj, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})

		if err != nil {
			// If the request failed, release resources tied to the timeout context
			cancel() // cancel context for the failed attempt

			lastErr = err

			// Check if this is a rate limit error
			if isS3RateLimitError(err) {
				continue // Retry on rate limit
			}

			// For non-rate-limit errors, fail immediately
			return nil, nil, err
		}

		// On success, *do not* call cancel() – keeping the context alive ensures
		// the underlying HTTP stream remains readable for the lifetime of
		// resp.Body. The 15-minute timeout will still automatically cancel if
		// the download stalls for too long.

		// Success – return both the response and the cancel so the caller can
		// defer their cleanup.
		return resp, cancel, nil
	}

	return nil, nil, fmt.Errorf("S3 GetObject failed after %d retries for %s, last error: %w", maxRetries, key, lastErr)
}

// -----------------------------------------------------------------------------
// COPY helpers & failed-file tracking structs
// -----------------------------------------------------------------------------

func copyObject(ctx context.Context, db *pgxpool.Pool, s3c *s3.Client, bucket, key, table string) error {
	// Download object with retry logic for rate limiting
	resp, cancel, err := getS3ObjectWithRetry(ctx, s3c, bucket, key, 3)
	if err != nil {
		return err
	}
	defer func() {
		resp.Body.Close()
		if cancel != nil {
			cancel()
		}
	}()

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()

	// Two-step load: COPY into per-connection staging (bigint ts) then upsert converting to timestamptz.
	return db.AcquireFunc(ctx, func(c *pgxpool.Conn) error {
		pgc := c.Conn().PgConn()

		stageTable := fmt.Sprintf("%s_stage_%d", table, pgc.PID())

		createStageSQL := fmt.Sprintf(`CREATE UNLOGGED TABLE IF NOT EXISTS %s (
			ticker        text,
			volume        numeric,
			open          numeric,
			close         numeric,
			high          numeric,
			low           numeric,
			"timestamp"  bigint     NOT NULL,
			transactions  integer
		)`, stageTable)
		if _, err := c.Exec(ctx, createStageSQL); err != nil {
			return fmt.Errorf("create staging table: %w", err)
		}

		copySQL := fmt.Sprintf(`COPY %s(ticker, volume, open, close, high, low, "timestamp", transactions)
 FROM STDIN WITH (FORMAT csv, HEADER true)`, stageTable)
		if _, err := pgc.CopyFrom(ctx, gz, copySQL); err != nil {
			// UNIQUE violations are handled by the upsert step, so COPY should not hit them.
			return err
		}

		// Count and log records with null/empty tickers for monitoring
		var nullTickerCount int64
		countSQL := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE ticker IS NULL OR ticker = ''`, stageTable)
		if err := c.QueryRow(ctx, countSQL).Scan(&nullTickerCount); err != nil {
			log.Printf("warning: failed to count null tickers in %s: %v", stageTable, err)
		} else if nullTickerCount > 0 {
			log.Printf("⚠️  Filtering out %d records with null/empty tickers from %s", nullTickerCount, key)
		}

		upsertSQL := fmt.Sprintf(`INSERT INTO %s (ticker, volume, open, close, high, low, "timestamp", transactions)
SELECT ticker, volume, open, close, high, low,
       to_timestamp("timestamp"::double precision / 1000000000) AT TIME ZONE 'UTC',
       transactions FROM %s
WHERE ticker IS NOT NULL AND ticker != ''
ON CONFLICT (ticker, "timestamp") DO UPDATE SET
    volume        = EXCLUDED.volume,
    open          = EXCLUDED.open,
    close         = EXCLUDED.close,
    high          = EXCLUDED.high,
    low           = EXCLUDED.low,
    transactions  = EXCLUDED.transactions`, table, stageTable)

		if _, err := c.Exec(ctx, upsertSQL); err != nil {
			return err
		}

		if _, err := c.Exec(ctx, fmt.Sprintf("TRUNCATE %s", stageTable)); err != nil {
			return err
		}

		return nil
	})
}

// copyCSV is no longer used but kept for reference; mark deprecated.
// Deprecated: use the two-step staging load path instead.
/*func copyCSV(ctx context.Context, pool *pgxpool.Pool, table string, r io.Reader) error {
	pgErr := pool.AcquireFunc(ctx, func(c *pgxpool.Conn) error {
		pgc := c.Conn().PgConn()
		sql := fmt.Sprintf("COPY %s(ticker, volume, open, close, high, low, \"timestamp\", transactions) FROM STDIN WITH (FORMAT csv, HEADER true)", table)
		_, err := pgc.CopyFrom(ctx, r, sql)
		return err
	})
	return pgErr
}*/

// Failure bookkeeping --------------------------------------------------------

type failedFile struct {
	Day       time.Time
	Timeframe string
	Reason    string
}

type failedCollector struct {
	mu   sync.Mutex
	list []failedFile
}

func (fc *failedCollector) Add(f failedFile) {
	fc.mu.Lock()
	fc.list = append(fc.list, f)
	fc.mu.Unlock()
}

func (fc *failedCollector) List() []failedFile {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	out := make([]failedFile, len(fc.list))
	copy(out, fc.list)
	return out
}

func (fc *failedCollector) PopAll() []failedFile {
	fc.mu.Lock()
	out := fc.list
	fc.list = nil
	fc.mu.Unlock()
	return out
}

// Processed-date tracking -----------------------------------------------------

/*
type processedDateTracker struct {
	mu             sync.Mutex
	processedDates map[time.Time]bool
}

func newProcessedDateTracker() *processedDateTracker {
	return &processedDateTracker{processedDates: make(map[time.Time]bool)}
}

func (p *processedDateTracker) AddProcessedDate(d time.Time) {
	p.mu.Lock()
	p.processedDates[d] = true
	p.mu.Unlock()
}

func (p *processedDateTracker) GetConservativeUpdateDate() time.Time {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.processedDates) == 0 {
		return time.Time{}
	}
	var earliest time.Time
	for d := range p.processedDates {
		if earliest.IsZero() || d.Before(earliest) {
			earliest = d
		}
	}
	return earliest.AddDate(0, 0, -1)
}
*/

// compressOldChunks compresses chunks older than the given effective timestamp in batches of 5.
func compressOldChunks(ctx context.Context, db *pgxpool.Pool, table string, effective time.Time) error {
	var lastChunk string
	for {
		var query string
		params := []interface{}{table, effective}
		if lastChunk == "" {
			query = `SELECT c FROM show_chunks($1, older_than => $2) AS c ORDER BY c`
		} else {
			query = `SELECT c FROM show_chunks($1, older_than => $2) AS c WHERE c > $3 ORDER BY c`
			params = append(params, lastChunk)
		}

		rows, err := db.Query(ctx, query, params...)
		if err != nil {
			return fmt.Errorf("query chunks: %w", err)
		}
		defer rows.Close()

		count := 0
		for rows.Next() {
			var chunk string
			if err := rows.Scan(&chunk); err != nil {
				log.Printf("scan chunk: %v", err)
				continue
			}
			if _, err := db.Exec(ctx, `SELECT compress_chunk($1, true)`, chunk); err != nil {
				log.Printf("compress %s failed: %v", chunk, err)
			}
			lastChunk = chunk
			count++
		}
		if rows.Err() != nil {
			return fmt.Errorf("rows error: %w", rows.Err())
		}
		if count == 0 {
			break
		}
	}
	return nil
}

// parseDayFromKey extracts YYYY-MM-DD from an S3 key that ends with .csv.gz.
func parseDayFromKey(key string) (time.Time, error) {
	parts := strings.Split(key, "/")
	if len(parts) == 0 {
		return time.Time{}, fmt.Errorf("invalid key")
	}
	fname := strings.TrimSuffix(parts[len(parts)-1], ".csv.gz")
	return time.Parse("2006-01-02", fname)
}
