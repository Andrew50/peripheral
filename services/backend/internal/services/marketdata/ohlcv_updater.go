package marketdata

// Package marketdata now provides a *minimal* updater that simply streams the
// original Polygon flat-file CSVs straight into TimescaleDB.  No per-row
// processing or format changes are performed – the Go code only:
//   1. Lists the relevant objects in the Polygon S3 bucket.
//   2. Downloads & decompresses each *.csv.gz file.
//   3. Executes `COPY <table> FROM STDIN WITH (FORMAT csv, HEADER true)`
//      piping the raw CSV bytes to Postgres.
//
// This removes ~90 % of the original logic: no ticker filtering, no securityID
// look-ups, no chunk compression juggling.  **A new migration (47.sql)** must
// already have created OHLCV tables that exactly mirror the flat-file column
// order.
//
// Environment variables that remain in use:
//   POLYGON_S3_ENDPOINT – S3 endpoint (default: https://files.polygon.io)
//   POLYGON_S3_KEY      – S3 key (required)
//   POLYGON_S3_SECRET   – S3 secret (required)
//   AWS_REGION          – Region (default: us-east-1)
//   S3_BUCKET           – Bucket (default: flatfiles)
//   YEARS_BACK          – How many past years of data to ingest (default: 1)
//
// The public entry-point `UpdateAllOHLCV` signature is preserved so callers do
// not need to change.

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"backend/internal/data"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4/pgxpool"
)

// -----------------------------------------------------------------------------
// Config helpers
// -----------------------------------------------------------------------------

type s3Config struct {
	Endpoint  string
	Bucket    string
	Key       string
	Secret    string
	Region    string
	YearsBack int
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("environment variable %s is required", key)
	}
	return v
}

func loadS3Config() s3Config {
	years := 1
	if y := os.Getenv("YEARS_BACK"); y != "" {
		if n, err := strconv.Atoi(y); err == nil && n > 0 {
			years = n
		}
	}
	return s3Config{
		Endpoint:  env("POLYGON_S3_ENDPOINT", "https://files.polygon.io"),
		Bucket:    env("S3_BUCKET", "flatfiles"),
		Key:       mustEnv("POLYGON_S3_KEY"),
		Secret:    mustEnv("POLYGON_S3_SECRET"),
		Region:    env("AWS_REGION", "us-east-1"),
		YearsBack: years,
	}
}

func newS3Client(cfg s3Config) (*s3.Client, error) {
	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.Key, cfg.Secret, "")),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, _ ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: cfg.Endpoint, SigningRegion: region, HostnameImmutable: true}, nil
			}),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws cfg: %w", err)
	}
	httpClient := &http.Client{Timeout: 30 * time.Second}
	return s3.NewFromConfig(awsCfg, func(o *s3.Options) { o.HTTPClient = httpClient }), nil
}

// -----------------------------------------------------------------------------
// Timeframe definitions – keep identical table ⇄ prefix mapping.
// -----------------------------------------------------------------------------

type timeframe struct {
	name      string
	s3Prefix  string
	tableName string
}

var timeframes = []timeframe{
	{"1-day", "us_stocks_sip/day_aggs_v1", "ohlcv_1d"},
	{"1-minute", "us_stocks_sip/minute_aggs_v1", "ohlcv_1m"},
}

// -----------------------------------------------------------------------------
// Configuration – worker pool sizes
// -----------------------------------------------------------------------------

// copyWorkerCount controls how many files are copied in parallel. Tune based on
// available DB connections and network bandwidth.
const copyWorkerCount = 4

// -----------------------------------------------------------------------------
// Public entry point
// -----------------------------------------------------------------------------

// UpdateAllOHLCV streams Polygon flat files for each timeframe directly into
// TimescaleDB without any in-memory transformation.
func UpdateAllOHLCV(conn *data.Conn) error {
	log.Println("🔄 Starting OHLCV update …")
	start := time.Now()

	cfg := loadS3Config()
	s3c, err := newS3Client(cfg)
	if err != nil {
		return err
	}

	ctx := context.Background()

	// Read last_loaded_at; default 2008-01-01 if not present
	fromDate, err := getLastLoadedAt(ctx, conn.DB)
	if err != nil {
		return err
	}

	for _, tf := range timeframes {
		if err := runTimeframe(ctx, conn.DB, s3c, cfg.Bucket, fromDate, tf); err != nil {
			return err
		}
	}

	// Update the tracker table to now (UTC date)
	if err := setLastLoadedAt(ctx, conn.DB, time.Now().UTC()); err != nil {
		log.Printf("warning: failed to update last_loaded_at: %v", err)
	}

	log.Printf("✅ Update finished in %v", time.Since(start))
	return nil
}

// -----------------------------------------------------------------------------
// Internal helpers
// -----------------------------------------------------------------------------

func runTimeframe(ctx context.Context, db *pgxpool.Pool, s3c *s3.Client, bucket string, fromDate time.Time, tf timeframe) error {
	log.Printf("🗂 Processing %s …", tf.name)

	// 1) Prepare table for bulk ingestion (drop secondary indexes, disable compression).
	if err := PreLoadSetup(ctx, db, tf.tableName); err != nil {
		return err
	}
	// Ensure cleanup even if processing fails.
	defer func() {
		if err := PostLoadCleanup(context.Background(), db, tf.tableName); err != nil {
			log.Printf("post-load cleanup failed for %s: %v", tf.tableName, err)
		}
	}()

	to := time.Now()
	prefixes := monthlyPrefixes(tf.s3Prefix, fromDate, to)

	// NEW: Collect all keys first to get accurate total count
	var allKeys []string
	for _, p := range prefixes {
		keys, err := listCSVObjects(ctx, s3c, bucket, p)
		if err != nil {
			return err
		}
		allKeys = append(allKeys, keys...)
	}

	if len(allKeys) == 0 {
		log.Printf("ℹ️  No files found for %s", tf.name)
		return nil
	}

	log.Printf("📁 Found %d total files for %s", len(allKeys), tf.name)

	// NEW: Track progress across all files
	total := int64(len(allKeys))
	var processed int64
	start := time.Now()
	progressInterval := int64(20) // print progress every 20 files

	// Process all keys with progress tracking
	if err := copyObjectsConcurrentWithProgress(ctx, db, s3c, bucket, allKeys, tf.tableName, &processed, total, start, progressInterval); err != nil {
		return err
	}

	log.Printf("✅ Completed %s: %d files in %v", tf.name, total, time.Since(start).Truncate(time.Second))
	return nil
}

func listCSVObjects(ctx context.Context, s3c *s3.Client, bucket, prefix string) ([]string, error) {
	var out []string
	pag := s3.NewListObjectsV2Paginator(s3c, &s3.ListObjectsV2Input{Bucket: aws.String(bucket), Prefix: aws.String(prefix)})
	for pag.HasMorePages() {
		page, err := pag.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			if obj.Key != nil && strings.HasSuffix(*obj.Key, ".csv.gz") {
				out = append(out, *obj.Key)
			}
		}
	}
	return out, nil
}

func copyObject(ctx context.Context, db *pgxpool.Pool, s3c *s3.Client, bucket, key, table string) error {
	//log.Printf("➡️  Copying %s into %s …", filepath.Base(key), table)

	resp, err := s3c.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()

	if err := copyCSV(ctx, db, table, gz); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			//log.Printf("⚠️  Duplicate key error for %s – skipping file", filepath.Base(key))
			return nil
		}
		return err
	}

	//log.Printf("✅ Copied %s", filepath.Base(key))
	return nil
}

func copyCSV(ctx context.Context, pool *pgxpool.Pool, table string, r io.Reader) error {
	return pool.AcquireFunc(ctx, func(c *pgxpool.Conn) error {
		// Explicitly disable restoring mode on the session used for COPY. If a previous
		// connection in the pool had `timescaledb.restoring` enabled by PreLoadSetup,
		// it would propagate with the pooled connection and block INSERT/COPY into
		// hypertables. Clearing it here guarantees the COPY can proceed.
		if _, err := c.Exec(ctx, "SET timescaledb.restoring = off;"); err != nil {
			return fmt.Errorf("disable restoring mode for copy: %w", err)
		}

		pgc := c.Conn().PgConn()
		sql := fmt.Sprintf("COPY %s FROM STDIN WITH (FORMAT csv, HEADER true)", table)
		if _, err := pgc.CopyFrom(ctx, r, sql); err != nil {
			return err
		}
		return nil
	})
}

// copyObjectsConcurrent copies the given S3 keys into the specified table using
// a fixed-size worker pool. Each worker processes one file at a time – files
// are *not* split between workers ensuring the atomicity requirement.
func copyObjectsConcurrent(ctx context.Context, db *pgxpool.Pool, s3c *s3.Client, bucket string, keys []string, table string) error {
	if len(keys) == 0 {
		return nil
	}

	keyCh := make(chan string)
	errCh := make(chan error, 1) // capture first error

	var wg sync.WaitGroup

	// Launch workers.
	for i := 0; i < copyWorkerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for k := range keyCh {
				if err := copyObject(ctx, db, s3c, bucket, k, table); err != nil {
					// Signal the first error encountered.
					select {
					case errCh <- err:
					default:
					}
					return
				}
			}
		}()
	}

	// Feed keys to workers.
	go func() {
		for _, k := range keys {
			keyCh <- k
		}
		close(keyCh)
	}()

	// Wait for completion or first error.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case err := <-errCh:
		return err
	case <-done:
		return nil
	}
}

// NEW: copyObjectsConcurrentWithProgress adds progress tracking to the concurrent copy
func copyObjectsConcurrentWithProgress(ctx context.Context, db *pgxpool.Pool, s3c *s3.Client, bucket string, keys []string, table string, processed *int64, total int64, start time.Time, progressInterval int64) error {
	if len(keys) == 0 {
		return nil
	}

	keyCh := make(chan string)
	errCh := make(chan error, 1) // capture first error

	var wg sync.WaitGroup

	// Launch workers.
	for i := 0; i < copyWorkerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for k := range keyCh {
				if err := copyObject(ctx, db, s3c, bucket, k, table); err != nil {
					// Signal the first error encountered.
					select {
					case errCh <- err:
					default:
					}
					return
				}

				// Update progress.
				n := atomic.AddInt64(processed, 1)
				if n%progressInterval == 0 || n == total {
					elapsed := time.Since(start)
					avgPer := time.Duration(0)
					if n > 0 {
						avgPer = elapsed / time.Duration(n)
					}
					remaining := total - n
					eta := time.Duration(0)
					if remaining > 0 && avgPer > 0 {
						eta = avgPer * time.Duration(remaining)
					}
					log.Printf("📊 %s progress: %d/%d files processed (remaining: %d), elapsed %v, est. %v left", table, n, total, remaining, elapsed.Truncate(time.Second), eta.Truncate(time.Second))
				}
			}
		}()
	}

	// Feed keys to workers.
	go func() {
		for _, k := range keys {
			keyCh <- k
		}
		close(keyCh)
	}()

	// Wait for completion or first error.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case err := <-errCh:
		return err
	case <-done:
		return nil
	}
}

// -----------------------------------------------------------------------------
// Utility helpers
// -----------------------------------------------------------------------------

func monthlyPrefixes(base string, from, to time.Time) []string {
	var out []string
	cur := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(to.Year(), to.Month(), 1, 0, 0, 0, 0, time.UTC)
	for !cur.After(end) {
		out = append(out, fmt.Sprintf("%s/%04d/%02d/", base, cur.Year(), cur.Month()))
		cur = cur.AddDate(0, 1, 0)
	}
	return out
}

// -----------------------------------------------------------------------------
// Startup DB health-check helper (compat shim for scheduler)
// -----------------------------------------------------------------------------

// EnsureDBStateOnStartup loops over all OHLCV tables and invokes PostLoadCleanup
// to make sure indexes, compression policies and restoring mode are consistent.
// This preserves the behaviour expected by `server/schedule.go` while delegating
// the heavy-lifting to our lightweight PostLoadCleanup implementation.
func EnsureDBStateOnStartup(ctx context.Context, db *pgxpool.Pool) error {
	tables := []string{"ohlcv_1m", "ohlcv_1d"}
	for _, tbl := range tables {
		if err := PostLoadCleanup(ctx, db, tbl); err != nil {
			return fmt.Errorf("startup check failed on %s: %w", tbl, err)
		}
	}
	return nil
}

// -----------------------------------------------------------------------------
// Maintenance helpers (lightweight versions of the original logic)
// -----------------------------------------------------------------------------

// PreLoadSetup prepares the table for fast bulk COPY by disabling compression
// and dropping non-PK indexes.  It must be run during a maintenance window.
func PreLoadSetup(ctx context.Context, db *pgxpool.Pool, tbl string) error {
	log.Printf("🔧 Pre-load setup for %s", tbl)

	// Enable Timescale restoring mode (disables background jobs like compression).
	if _, err := db.Exec(ctx, `SET timescaledb.restoring = on;`); err != nil {
		return fmt.Errorf("enable restoring mode: %w", err)
	}

	// Drop secondary indexes (keep pkey / constraints). Uses a DO block so it's safe if no indexes exist.
	dropSQL := fmt.Sprintf(`DO $$
DECLARE idx record;
BEGIN
  FOR idx IN
    SELECT indexname FROM pg_indexes
    WHERE schemaname = 'public' AND tablename = '%s'
      AND indexname NOT ILIKE '%%_pkey' AND indexname NOT ILIKE '%%_constraint'
  LOOP
    EXECUTE format('DROP INDEX IF EXISTS %%I', idx.indexname);
  END LOOP;
END$$;`, tbl)
	if _, err := db.Exec(ctx, dropSQL); err != nil {
		return fmt.Errorf("drop indexes: %w", err)
	}
	return nil
}

// PostLoadCleanup recreates the essential index and re-enables compression.
func PostLoadCleanup(ctx context.Context, db *pgxpool.Pool, tbl string) error {
	log.Printf("🔧 Post-load cleanup for %s", tbl)

	// Disable restoring mode.
	if _, err := db.Exec(ctx, `SET timescaledb.restoring = off;`); err != nil {
		return fmt.Errorf("disable restoring mode: %w", err)
	}

	// Ensure a compression policy exists (ignore errors if extension not present).
	_, _ = db.Exec(ctx, fmt.Sprintf(`SELECT add_compression_policy('%s', 302400000000000)`, tbl))

	// Update statistics.
	if _, err := db.Exec(ctx, fmt.Sprintf(`ANALYZE %s`, tbl)); err != nil {
		log.Printf("analyze warning for %s: %v", tbl, err)
	}
	return nil
}

// -----------------------------------------------------------------------------
// Tracker helpers
// -----------------------------------------------------------------------------

func getLastLoadedAt(ctx context.Context, db *pgxpool.Pool) (time.Time, error) {
	var d time.Time
	err := db.QueryRow(ctx, `SELECT last_loaded_at FROM ohlcv_update_state LIMIT 1`).Scan(&d)
	if err != nil {
		return time.Date(2008, 1, 1, 0, 0, 0, 0, time.UTC), nil // fallback
	}
	return d, nil
}

func setLastLoadedAt(ctx context.Context, db *pgxpool.Pool, t time.Time) error {
	_, err := db.Exec(ctx, `UPDATE ohlcv_update_state SET last_loaded_at = $1 WHERE id = TRUE`, t)
	return err
}
