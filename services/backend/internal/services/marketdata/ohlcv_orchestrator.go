package marketdata

import (
	"context"
	"fmt"
	"log"
	"time"

	"backend/internal/data"

	"errors"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

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
	for _, tf := range timeframes {
		if err := runTimeframe(ctx, conn.DB, s3c, cfg.Bucket, tf); err != nil {
			return err
		}
	}

	log.Printf("✅ Update finished in %v", time.Since(start))
	return nil
}

// runTimeframe orchestrates pre-load setup → pipeline → post-load cleanup for a single timeframe.
func runTimeframe(ctx context.Context, db *pgxpool.Pool, s3c *s3.Client, bucket string, tf timeframe) error {
	log.Printf("🗂 Processing %s …", tf.name)

	if err := PreLoadSetup(ctx, db, tf.tableName); err != nil {
		return err
	}
	defer func() {
		if err := PostLoadCleanup(context.Background(), db, tf.tableName); err != nil {
			log.Printf("post-load cleanup failed for %s: %v", tf.tableName, err)
		}
	}()

	// Get current load state
	state, err := getLoadState(ctx, db, tf.name)
	if err != nil {
		return err
	}

	// log.Printf("🔍 Initial load state for %s: earliest=%s, latest=%s", tf.name,
	//	func() string {
	//		if state.Earliest.IsZero() {
	//			return "null"
	//		} else {
	//			return state.Earliest.Format("2006-01-02")
	//		}
	//	}(),
	//	func() string {
	//		if state.Latest.IsZero() {
	//			return "null"
	//		} else {
	//			return state.Latest.Format("2006-01-02")
	//		}
	//	}())

	upper := time.Now().UTC()
	lower := time.Date(2003, 9, 10, 0, 0, 0, 0, time.UTC)

	// ---- Forward pass ----
	if !state.Latest.IsZero() {
		log.Printf("➡️  Running forward pass for %s from %s", tf.name, state.Latest.Format("2006-01-02"))
		if err := runForward(ctx, db, s3c, bucket, state.Latest, upper, tf, state); err != nil {
			return err
		}
		// Refresh state after forward pass
		state, err = getLoadState(ctx, db, tf.name)
		if err != nil {
			return err
		}
	}

	// ---- Backward pass ----
	var startBack time.Time
	if state.Earliest.IsZero() {
		startBack = upper // nothing old loaded yet
		// log.Printf("🔍 Backward pass: state.Earliest is zero, starting from upper=%s", upper.Format("2006-01-02"))
	} else {
		startBack = state.Earliest // inclusive start
		// log.Printf("🔍 Backward pass: using state.Earliest=%s as startBack", state.Earliest.Format("2006-01-02"))
	}

	// CRITICAL FIX: If latest_loaded_at is NULL and we're doing backward loading,
	// we need to initialize it to the starting point, otherwise it never gets set
	if state.Latest.IsZero() && startBack.After(lower) {
		// log.Printf("🔍 Backward pass: latest_loaded_at is null, initializing to startBack=%s", startBack.Format("2006-01-02"))
		state.Latest = startBack
		if err := setLoadState(ctx, db, tf.name, state); err != nil {
			log.Printf("❌ Failed to initialize latest_loaded_at: %v", err)
			return err
		}
		// log.Printf("✅ Initialized latest_loaded_at to %s for backward loading", startBack.Format("2006-01-02"))
	}

	// log.Printf("🔍 Backward pass decision: startBack=%s, lower=%s, will run=%v",
	//	startBack.Format("2006-01-02"), lower.Format("2006-01-02"), startBack.After(lower))

	if startBack.After(lower) {
		log.Printf("⬅️  Running backward pass for %s from %s down to %s", tf.name, startBack.Format("2006-01-02"), lower.Format("2006-01-02"))
		if err := runBackward(ctx, db, s3c, bucket, startBack, lower, tf, state); err != nil {
			return err
		}
	}

	log.Printf("✅ Completed %s", tf.name)
	return nil
}

// runForward processes files from the latest loaded date forward to now
func runForward(ctx context.Context, db *pgxpool.Pool, s3c *s3.Client, bucket string, from, to time.Time, tf timeframe, state LoadState) error {
	prefixes := monthlyPrefixes(tf.s3Prefix, from, to)

	var keysToProcess []string
	for _, p := range prefixes {
		keys, err := listCSVObjects(ctx, s3c, bucket, p)
		if err != nil {
			return err
		}

		// Filter keys to only include files AFTER the 'from' date
		// If latest_loaded_at is 2025-07-21, we should only load 2025-07-22 and newer
		for _, key := range keys {
			if fileDate, err := parseDayFromKey(key); err == nil {
				if fileDate.After(from) {
					keysToProcess = append(keysToProcess, key)
				}
			}
		}
	}

	if len(keysToProcess) == 0 {
		// log.Printf("ℹ️  No files to process for %s forward pass from %s", tf.name, from.Format("2006-01-02"))
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
	// start := time.Now()

	_, err = processFilesWithPipeline(ctx, s3c, bucket, keysToProcess, tf.tableName, tf.name, bulkConnPool, fc, &processed, &skipped, total, 10, db, from, true, state)
	if err != nil {
		return err
	}

	// log.Printf("✅ Forward pass %s: %d files in %v (skipped %d)", tf.name, total, time.Since(start).Truncate(time.Second), skipped)
	return nil
}

// runBackward processes files from the earliest loaded date backward to the lower bound
func runBackward(ctx context.Context, db *pgxpool.Pool, s3c *s3.Client, bucket string, from, to time.Time, tf timeframe, state LoadState) error {
	// Get prefixes in reverse order (from newest to oldest)
	prefixes := monthlyPrefixes(tf.s3Prefix, to, from)

	// Reverse the prefixes to process newest first when going backward
	for i, j := 0, len(prefixes)-1; i < j; i, j = i+1, j-1 {
		prefixes[i], prefixes[j] = prefixes[j], prefixes[i]
	}

	var keysToProcess []string
	for _, p := range prefixes {
		keys, err := listCSVObjects(ctx, s3c, bucket, p)
		if err != nil {
			return err
		}

		// Filter keys to only include files BEFORE the 'from' date
		// If earliest_loaded_at is 2025-07-21, we should only load 2025-07-20 and older
		var filteredKeys []string
		for _, key := range keys {
			if fileDate, err := parseDayFromKey(key); err == nil {
				if fileDate.Before(from) {
					filteredKeys = append(filteredKeys, key)
				}
			}
		}

		// Reverse keys within each prefix to maintain descending order
		for i, j := 0, len(filteredKeys)-1; i < j; i, j = i+1, j-1 {
			filteredKeys[i], filteredKeys[j] = filteredKeys[j], filteredKeys[i]
		}
		keysToProcess = append(keysToProcess, filteredKeys...)
	}

	if len(keysToProcess) == 0 {
		// log.Printf("ℹ️  No files to process for %s backward pass from %s", tf.name, from.Format("2006-01-02"))
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
	// start := time.Now()

	_, err = processFilesWithPipeline(ctx, s3c, bucket, keysToProcess, tf.tableName, tf.name, bulkConnPool, fc, &processed, &skipped, total, 10, db, from, false, state)
	if err != nil {
		return err
	}

	// log.Printf("✅ Backward pass %s: %d files in %v (skipped %d)", tf.name, total, time.Since(start).Truncate(time.Second), skipped)
	return nil
}

// -----------------------------------------------------------------------------
// Maintenance helpers (unchanged from original, but local to orchestrator)
// -----------------------------------------------------------------------------

func PreLoadSetup(ctx context.Context, db *pgxpool.Pool, tbl string) error {
	// log.Printf("🔧 Pre-load setup for %s", tbl)

	// Remove any existing compression policy
	// no reason to drop as no compression poclies are used, it is just done manually.
	/*
		removeSQL := fmt.Sprintf(`SELECT remove_compression_policy('%s', TRUE);`, tbl)
		if _, err := data.ExecWithRetry(ctx, db, removeSQL); err != nil {

			log.Printf("warning: remove compression policy for %s: %v", tbl, err)
		}
	*/

	// Note: Indexes are now managed by the database on startup and should not be dropped here

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
	// log.Printf("🔧 Post-load cleanup for %s", tbl)

	//autovaccum is always on so not more multixfact overflows
	/*
		if _, err := data.ExecWithRetry(ctx, db, fmt.Sprintf(`ALTER TABLE %s RESET (autovacuum_enabled)`, tbl)); err != nil {
			return fmt.Errorf("re-enable autovacuum for %s: %w", tbl, err)
		}
	*/

	// Note: Indexes are now managed by the database on startup and should not be recreated here

	if _, err := data.ExecWithRetry(ctx, db, fmt.Sprintf(`ANALYZE %s`, tbl)); err != nil {
		log.Printf("analyze warning for %s: %v", tbl, err)
	}

	// Final compression using bidirectional bounds
	var timeframe string
	var chunkOffset time.Duration
	var minDuration time.Duration
	switch tbl {
	case "ohlcv_1m":
		timeframe = "1-minute"
		minDuration = 7 * 24 * time.Hour
		chunkOffset = 4 * 30 * 24 * time.Hour // 4 months
	case "ohlcv_1d":
		timeframe = "1-day"
		minDuration = 60 * 24 * time.Hour
		chunkOffset = 8 * 365 * 24 * time.Hour // 8 years
	}

	state, err := getLoadState(ctx, db, timeframe)
	if err != nil {
		log.Printf("warning: get load state for %s: %v", tbl, err)
		return nil // Continue without compression
	}

	// Only compress if we have both bounds established
	if !state.Latest.IsZero() && !state.Earliest.IsZero() {
		now := time.Now().UTC()
		lower := time.Date(2003, 9, 10, 0, 0, 0, 0, time.UTC)

		// Determine safe compression range
		upperBound := state.Latest.Add(-chunkOffset)
		lowerBound := state.Earliest.Add(chunkOffset)

		// Apply recent-data safety limit
		recentSafe := now.Add(-minDuration)
		if upperBound.After(recentSafe) {
			upperBound = recentSafe
		}

		// Skip lower bound check if we've reached the absolute minimum
		if state.Earliest.Equal(lower) || state.Earliest.Before(lower) {
			lowerBound = time.Time{} // No lower bound restriction
		}

		// Validate compression bounds before attempting compression
		if !lowerBound.IsZero() && !upperBound.After(lowerBound) {
			// log.Printf("ℹ️  Skipping final compression for %s: insufficient data range (span less than %v)",
			//	tbl, chunkOffset*2)
		} else {
			if err := compressChunksInRange(ctx, db, tbl, upperBound, lowerBound); err != nil {
				// log.Printf("final compression failed for %s: %v", tbl, err)
			}
		}
	} else if !state.Latest.IsZero() {
		// Only forward loading has been done - use old logic
		now := time.Now().UTC()
		recentSafe := now.Add(-minDuration)
		eff := state.Latest
		if state.Latest.After(recentSafe) {
			eff = recentSafe
		}
		if err := compressOldChunks(ctx, db, tbl, eff); err != nil {
			// log.Printf("final compression failed for %s: %v", tbl, err)
		}
	}

	stageTbl := tbl + "_stage"
	if _, err := data.ExecWithRetry(ctx, db, fmt.Sprintf(`DROP TABLE IF EXISTS %s`, stageTbl)); err != nil {
		// log.Printf("warning: drop staging table %s: %v", stageTbl, err)
	}
	// Also remove the composite type in case the table was dropped elsewhere
	// and only the type remains (prevents "type ... already exists" on the
	// next run).
	if _, err := data.ExecWithRetry(ctx, db, fmt.Sprintf(`DROP TYPE IF EXISTS %s CASCADE`, stageTbl)); err != nil {
		// log.Printf("warning: drop staging type %s: %v", stageTbl, err)
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
		// log.Printf("warning: drop worker stage tables for %s: %v", tbl, err)
	}

	return nil
}

// -----------------------------------------------------------------------------
// Tracker helpers (unchanged)
// -----------------------------------------------------------------------------

// LoadState represents the bidirectional loading state with earliest and latest loaded dates
type LoadState struct {
	Earliest time.Time // zero => NULL
	Latest   time.Time // zero => NULL
}

// getLoadState retrieves the current load state for a timeframe
func getLoadState(ctx context.Context, db *pgxpool.Pool, tf string) (LoadState, error) {
	var state LoadState
	var earliestPtr, latestPtr *time.Time

	err := db.QueryRow(ctx, `SELECT earliest_loaded_at, latest_loaded_at FROM ohlcv_update_state WHERE timeframe = $1 LIMIT 1`, tf).Scan(&earliestPtr, &latestPtr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// First run – return zero state
			return LoadState{}, nil
		}
		return LoadState{}, err
	}

	if earliestPtr != nil {
		state.Earliest = *earliestPtr
	}
	if latestPtr != nil {
		state.Latest = *latestPtr
	}

	return state, nil
}

// setLoadState updates the load state for a timeframe
func setLoadState(ctx context.Context, db *pgxpool.Pool, tf string, s LoadState) error {
	var earliestPtr, latestPtr *time.Time

	if !s.Earliest.IsZero() {
		earliestPtr = &s.Earliest
	}
	if !s.Latest.IsZero() {
		latestPtr = &s.Latest
	}

	// log.Printf("🔍 setLoadState: timeframe=%s, earliest=%s, latest=%s", tf,
	//	func() string {
	//		if earliestPtr == nil {
	//			return "null"
	//		} else {
	//			return earliestPtr.Format("2006-01-02")
	//		}
	//	}(),
	//	func() string {
	//		if latestPtr == nil {
	//			return "null"
	//		} else {
	//			return latestPtr.Format("2006-01-02")
	//		}
	//	}())

	_, err := data.ExecWithRetry(ctx, db, `INSERT INTO ohlcv_update_state(timeframe, earliest_loaded_at, latest_loaded_at)
                                         VALUES($1, $2, $3)
                                         ON CONFLICT (timeframe) DO UPDATE
                                         SET earliest_loaded_at = EXCLUDED.earliest_loaded_at,
                                             latest_loaded_at = EXCLUDED.latest_loaded_at`, tf, earliestPtr, latestPtr)

	if err != nil {
		// log.Printf("🔍 setLoadState: ERROR writing to database: %v", err)
	} else {
		// log.Printf("🔍 setLoadState: successfully wrote to database")
	}

	return err
}

// Deprecated: Use getLoadState instead
func getLastLoadedAt(ctx context.Context, db *pgxpool.Pool, timeframe string) (time.Time, error) {
	state, err := getLoadState(ctx, db, timeframe)
	if err != nil {
		return time.Time{}, err
	}
	if state.Latest.IsZero() {
		// First run – start from earliest date.
		return time.Date(2003, 1, 1, 0, 0, 0, 0, time.UTC), nil
	}
	return state.Latest, nil
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

// CheckOHLCVPartialCoverage verifies that both 1-minute and 1-day OHLCV data
// have earliest_loaded_at at least 2 months before now. This allows services
// to start once sufficient historical data is available, without waiting for
// the complete multi-year backfill to finish.
func CheckOHLCVPartialCoverage(conn *data.Conn) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Require data going back at least 2 months
	requiredEarliest := time.Now().UTC().AddDate(0, -2, 0) // 2 months ago

	// Check both 1-minute and 1-day timeframes
	timeframes := []string{"1-minute", "1-day"}

	for _, tf := range timeframes {
		var earliestLoaded *time.Time
		err := conn.DB.QueryRow(ctx, `SELECT earliest_loaded_at FROM ohlcv_update_state WHERE timeframe = $1 LIMIT 1`, tf).Scan(&earliestLoaded)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// log.Printf("⚠️  No update state found for timeframe %s", tf)
				return false, nil // No error, just not ready yet
			}
			return false, fmt.Errorf("failed to check update state for %s: %w", tf, err)
		}

		// If earliest_loaded_at is NULL or not far enough back, not ready
		if earliestLoaded == nil || earliestLoaded.After(requiredEarliest) {
			if earliestLoaded == nil {
				// log.Printf("⚠️  OHLCV data for %s has no earliest bound yet", tf)
			} else {
				// log.Printf("⚠️  OHLCV data for %s earliest coverage is %v, need at least %v", tf, earliestLoaded.Format("2006-01-02"), requiredEarliest.Format("2006-01-02"))
			}
			return false, nil
		}
	}

	// log.Printf("✅ OHLCV partial coverage sufficient: both timeframes have data going back at least 2 months")
	return true, nil
}
