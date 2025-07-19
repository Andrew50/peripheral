package marketdata

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"backend/internal/data"

	"errors"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// isFirstFullLoad returns true if this is the first time loading OHLCV data (determined via freshness check)
func isFirstFullLoad(conn *data.Conn) bool {
	_, err := CheckOHLCVDataFreshness(conn)
	if err != nil {
		// missing update state implies first load
		return true
	}
	// data exists (fresh or stale), not first load
	return false
}

// UpdateAllOHLCV streams Polygon flat files for each timeframe directly into
// TimescaleDB without any in-memory transformation.
func UpdateAllOHLCV(conn *data.Conn) error {
	log.Println("🔄 Starting OHLCV update …")
	start := time.Now()

	// Determine if this is the first full load
	isFirst := isFirstFullLoad(conn)
	cfg := loadS3Config()
	s3c, err := newS3Client(cfg)
	if err != nil {
		return err
	}

	ctx := context.Background()
	for _, tf := range timeframes {
		fromDate, err := getLastLoadedAt(ctx, conn.DB, tf.name)
		if err != nil {
			return err
		}
		if err := runTimeframe(ctx, conn.DB, s3c, cfg.Bucket, fromDate, tf, isFirst); err != nil {
			return err
		}
	}

	log.Printf("✅ Update finished in %v", time.Since(start))
	return nil
}

// runTimeframe orchestrates pre-load setup → pipeline → post-load cleanup for a single timeframe.
func runTimeframe(ctx context.Context, db *pgxpool.Pool, s3c *s3.Client, bucket string, fromDate time.Time, tf timeframe, isFirstRun bool) error {
	log.Printf("🗂 Processing %s …", tf.name)

	if err := PreLoadSetup(ctx, db, tf.tableName, isFirstRun); err != nil {
		return err
	}
	defer func() {
		if err := PostLoadCleanup(context.Background(), db, tf.tableName); err != nil {
			log.Printf("post-load cleanup failed for %s: %v", tf.tableName, err)
		}
	}()

	to := time.Now()
	prefixes := monthlyPrefixes(tf.s3Prefix, fromDate, to)

	var keysToProcess []string
	for _, p := range prefixes {
		keys, err := listCSVObjects(ctx, s3c, bucket, p)
		if err != nil {
			return err
		}
		keysToProcess = append(keysToProcess, keys...)
	}

	if len(keysToProcess) == 0 {
		log.Printf("ℹ️  No files to process for %s starting from %s", tf.name, fromDate.Format("2006-01-02"))
		return nil
	}

	fc := &failedCollector{}
	bulkConnPool, err := newBulkLoadPoolWithRetry(ctx, db)
	if err != nil {
		return fmt.Errorf("create bulk load pool: %w", err)
	}
	defer bulkConnPool.Close()

	total := int64(len(keysToProcess))
	var processed int64
	var skipped int64
	start := time.Now()

	_, err = processFilesWithPipeline(ctx, s3c, bucket, keysToProcess, tf.tableName, tf.name, bulkConnPool, fc, &processed, &skipped, total, 10, db, fromDate)
	if err != nil {
		return err
	}

	log.Printf("✅ Completed %s: %d files in %v (skipped %d)", tf.name, total, time.Since(start).Truncate(time.Second), atomic.LoadInt64(&skipped))
	return nil
}

// -----------------------------------------------------------------------------
// Maintenance helpers (unchanged from original, but local to orchestrator)
// -----------------------------------------------------------------------------

func PreLoadSetup(ctx context.Context, db *pgxpool.Pool, tbl string, isFirstRun bool) error {
	log.Printf("🔧 Pre-load setup for %s", tbl)

	// Remove any existing compression policy
	// no reason to drop as no compression poclies are used, it is just done manually.
	/*
		removeSQL := fmt.Sprintf(`SELECT remove_compression_policy('%s', TRUE);`, tbl)
		if _, err := data.ExecWithRetry(ctx, db, removeSQL); err != nil {

			log.Printf("warning: remove compression policy for %s: %v", tbl, err)
		}
	*/

	if isFirstRun {
		dropSQL := fmt.Sprintf(`DO $$
DECLARE idx record;
BEGIN
  FOR idx IN
    SELECT ci.relname AS indexname
    FROM pg_index i
    JOIN pg_class ci ON ci.oid = i.indexrelid
    JOIN pg_class ct ON ct.oid = i.indrelid
    JOIN pg_namespace n ON n.oid = ct.relnamespace
    WHERE n.nspname = 'public'
      AND ct.relname = '%s'
      AND NOT i.indisprimary
      AND NOT i.indisunique
  LOOP
    EXECUTE format('DROP INDEX IF EXISTS %%I', idx.indexname);
  END LOOP;
END$$;`, tbl)
		if _, err := data.ExecWithRetry(ctx, db, dropSQL); err != nil {
			return fmt.Errorf("drop indexes: %w", err)
		}
	}

	// TimescaleDB automatically creates chunks on demand; explicit pre-creation is no longer necessary.

	// -----------------------------------------------------------------
	// NOTE: Previously we created a shared staging table `<tbl>_stage` here.
	// That approach was prone to race conditions between concurrent sessions
	// that still held cached references to the table's composite *row type*.
	// Dropping the type in one session could invalidate another session's
	// cache and trigger «cache lookup failed for relation …» (SQLSTATE XX000).
	//
	// The load pipeline has since moved to per-connection staging tables
	// (`<tbl>_stage_<pid>`), so the shared table is no longer required.
	// We therefore skip this step entirely to avoid the invalidation hazard.

	return nil
}

func PostLoadCleanup(ctx context.Context, db *pgxpool.Pool, tbl string) error {
	log.Printf("🔧 Post-load cleanup for %s", tbl)

	//autovaccum is always on so not more multixfact overflows
	/*
		if _, err := data.ExecWithRetry(ctx, db, fmt.Sprintf(`ALTER TABLE %s RESET (autovacuum_enabled)`, tbl)); err != nil {
			return fmt.Errorf("re-enable autovacuum for %s: %w", tbl, err)
		}
	*/

	// Recreate helpful covering indexes on source tables
	var indexSQLs []string
	switch tbl {
	case "ohlcv_1m":
		indexSQLs = Ohlcv1mIndexSQLs()
	case "ohlcv_1d":
		indexSQLs = Ohlcv1dIndexSQLs()
	default:
		indexSQLs = nil
	}
	for _, q := range indexSQLs {
		if _, err := data.ExecWithRetry(ctx, db, q); err != nil {
			return fmt.Errorf("recreate index %s: %w", q, err)
		}
	}
	if _, err := data.ExecWithRetry(ctx, db, fmt.Sprintf(`ANALYZE %s`, tbl)); err != nil {
		log.Printf("analyze warning for %s: %v", tbl, err)
	}

	// Final compression
	var timeframe string
	switch tbl {
	case "ohlcv_1m":
		timeframe = "1-minute"
	case "ohlcv_1d":
		timeframe = "1-day"
	}
	lastLoaded, err := getLastLoadedAt(ctx, db, timeframe)
	if err != nil {
		log.Printf("warning: get last loaded for %s: %v", tbl, err)
		return nil // Continue without compression
	}
	var minDuration time.Duration
	switch timeframe {
	case "1-minute":
		minDuration = 7 * 24 * time.Hour
	case "1-day":
		minDuration = 60 * 24 * time.Hour
	}
	now := time.Now().UTC()
	recentSafe := now.Add(-minDuration)
	eff := lastLoaded
	if lastLoaded.After(recentSafe) {
		eff = recentSafe
	}
	if err := compressOldChunks(ctx, db, tbl, eff); err != nil {
		log.Printf("final compression failed for %s: %v", tbl, err)
	}

	stageTbl := tbl + "_stage"
	if _, err := data.ExecWithRetry(ctx, db, fmt.Sprintf(`DROP TABLE IF EXISTS %s`, stageTbl)); err != nil {
		log.Printf("warning: drop staging table %s: %v", stageTbl, err)
	}
	// Also remove the composite type in case the table was dropped elsewhere
	// and only the type remains (prevents "type ... already exists" on the
	// next run).
	if _, err := data.ExecWithRetry(ctx, db, fmt.Sprintf(`DROP TYPE IF EXISTS %s CASCADE`, stageTbl)); err != nil {
		log.Printf("warning: drop staging type %s: %v", stageTbl, err)
	}

	// -------------------------------------------------------------------
	// Cleanup any per-connection staging tables left behind by workers.
	// These tables follow the naming pattern <tbl>_stage_<pid> and are
	// created in ohlcv_pipeline.go for each database connection. They are
	// truncated after use but not dropped, which leads to clutter over
	// repeated runs. Remove them now to keep the schema tidy.
	// -------------------------------------------------------------------

	dropWorkerStagesSQL := fmt.Sprintf(`DO $$
DECLARE r record;
BEGIN
    FOR r IN
        SELECT tablename FROM pg_tables
        WHERE schemaname = 'public' AND tablename LIKE '%s_stage_%%'
    LOOP
        EXECUTE format('DROP TABLE IF EXISTS %%I', r.tablename);
    END LOOP;
END$$;`, tbl)

	if _, err := data.ExecWithRetry(ctx, db, dropWorkerStagesSQL); err != nil {
		log.Printf("warning: drop worker stage tables for %s: %v", tbl, err)
	}

	return nil
}

// -----------------------------------------------------------------------------
// Tracker helpers (unchanged)
// -----------------------------------------------------------------------------

func getLastLoadedAt(ctx context.Context, db *pgxpool.Pool, timeframe string) (time.Time, error) {
	var d time.Time
	err := db.QueryRow(ctx, `SELECT last_loaded_at FROM ohlcv_update_state WHERE timeframe = $1 LIMIT 1`, timeframe).Scan(&d)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// First run – start from earliest date.
			return time.Date(2003, 1, 1, 0, 0, 0, 0, time.UTC), nil
		}
		// Any other error should propagate so the updater fails fast.
		return time.Time{}, err
	}
	return d, nil
}

func setLastLoadedAt(ctx context.Context, db *pgxpool.Pool, timeframe string, t time.Time) error {
	// Use the robust retry helper so a brief database restart does not abort the
	// entire ingestion run.
	_, err := data.ExecWithRetry(ctx, db, `INSERT INTO ohlcv_update_state(timeframe, last_loaded_at)
                                         VALUES($1, $2)
                                         ON CONFLICT (timeframe) DO UPDATE
                                         SET last_loaded_at = EXCLUDED.last_loaded_at`, timeframe, t)
	return err
}

func storeFailedFiles(ctx context.Context, db *pgxpool.Pool, files []failedFile) error {
	if len(files) == 0 {
		return nil
	}
	for _, f := range files {
		if _, err := data.ExecWithRetry(ctx, db, `INSERT INTO ohlcv_failed_files(day, timeframe, reason)
                                                VALUES($1,$2,$3)
                                                ON CONFLICT DO NOTHING`, f.Day, f.Timeframe, f.Reason); err != nil {
			return err
		}
	}
	return nil
}

// CheckOHLCVDataFreshness verifies that both 1-minute and 1-day OHLCV data
// have been updated within the last 7 days. Returns true if data is fresh,
// false if either timeframe is stale or missing.
func CheckOHLCVDataFreshness(conn *data.Conn) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cutoffTime := time.Now().UTC().AddDate(0, 0, -7) // 7 days ago

	// Check both 1-minute and 1-day timeframes
	timeframes := []string{"1-minute", "1-day"}

	for _, tf := range timeframes {
		var lastLoaded time.Time
		err := conn.DB.QueryRow(ctx, `SELECT last_loaded_at FROM ohlcv_update_state WHERE timeframe = $1 LIMIT 1`, tf).Scan(&lastLoaded)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				log.Printf("⚠️  No update state found for timeframe %s", tf)
				return false, fmt.Errorf("no update state found for timeframe %s", tf)
			}
			return false, fmt.Errorf("failed to check update state for %s: %w", tf, err)
		}

		if lastLoaded.Before(cutoffTime) {
			log.Printf("⚠️  OHLCV data for %s is stale (last updated: %v, cutoff: %v)", tf, lastLoaded, cutoffTime)
			return false, nil
		}
	}

	log.Printf("✅ OHLCV data is fresh for both 1-minute and 1-day timeframes")
	return true, nil
}
