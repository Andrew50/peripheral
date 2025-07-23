package screener

import (
	"backend/internal/data"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
)

type TestQuery struct {
	Name  string
	Query string
}

type AnalysisConfig struct {
	LogFilePath      string
	StaleQuery       string
	StaleQueryParams []interface{}
	Tables           []string
	QueryPatterns    []string
	TestFunctions    []TestQuery
	ComponentTests   []TestQuery
}

func RunPerformanceAnalysis(conn *data.Conn, config AnalysisConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create analysis log file with absolute path
	logFilePath := config.LogFilePath
	fallbackPath := "./screener_analysis.log" // Fallback if primary fails

	log.Printf("📊 Creating performance analysis log at: %s", logFilePath)

	// Validate file path to prevent directory traversal and ensure it's safe
	cleanPath := filepath.Clean(logFilePath)
	if strings.Contains(cleanPath, "..") || !filepath.IsAbs(cleanPath) {
		return fmt.Errorf("invalid log file path: path traversal detected or relative path not allowed")
	}

	// Ensure the directory exists
	dir := filepath.Dir(cleanPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for log file: %v", err)
	}

	logFile, err := os.Create(cleanPath)
	if err != nil {
		log.Printf("❌ Failed to create analysis log file at %s: %v", cleanPath, err)
		log.Printf("📊 Trying fallback location: %s", fallbackPath)
		logFile, err = os.Create(fallbackPath)
		if err != nil {
			log.Printf("❌ Failed to create analysis log file at fallback location %s: %v", fallbackPath, err)
			return fmt.Errorf("failed to create analysis log file: %v", err)
		}
		logFilePath = fallbackPath
	} else {
		logFilePath = cleanPath
	}
	defer logFile.Close()

	log.Printf("✅ Successfully created analysis log file at: %s", logFilePath)

	// Write header
	fmt.Fprintf(logFile, "=== PERFORMANCE ANALYSIS LOG - %s ===\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(logFile, "Log file path: %s\n\n", logFilePath)

	// Get items (e.g., tickers) if StaleQuery is provided
	var items []string
	if config.StaleQuery != "" {
		rows, err := conn.DB.Query(ctx, config.StaleQuery, config.StaleQueryParams...)
		if err != nil {
			fmt.Fprintf(logFile, "⚠️  Failed to get items list: %v\n", err)
		} else {
			defer rows.Close()
			for rows.Next() {
				var item string
				var dummyTime time.Time
				var dummyBool bool
				if err := rows.Scan(&item, &dummyTime, &dummyBool); err == nil {
					items = append(items, item)
				}
			}
		}
	}

	if len(items) > 0 {
		fmt.Fprintf(logFile, "\n📊 Total items to process: %d\n", len(items))
		fmt.Fprintf(logFile, "📊 Sample items: %v\n\n", items[:min(len(items), 5)])
	} else {
		fmt.Fprintln(logFile, "📊 No items found for analysis")
	}

	// Enable track_io_timing
	_, err = conn.DB.Exec(ctx, "SET track_io_timing = on")
	if err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to enable track_io_timing: %v\n", err)
	}

	// Run general analyses
	if err := analyzeDatabaseConfiguration(ctx, conn, logFile); err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to analyze database configuration: %v\n", err)
	}
	if err := analyzeDatabaseActivity(ctx, conn, logFile, config.QueryPatterns); err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to analyze database activity: %v\n", err)
	}
	if err := analyzeLockActivity(ctx, conn, logFile, config.QueryPatterns); err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to analyze lock activity: %v\n", err)
	}
	if err := analyzeWaitEvents(ctx, conn, logFile, config.QueryPatterns); err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to analyze wait events: %v\n", err)
	}
	if err := analyzePgStatStatements(ctx, conn, logFile, config.QueryPatterns); err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to analyze pg_stat_statements: %v\n", err)
	}
	if err := analyzeQueryPlans(ctx, conn, logFile, config.TestFunctions); err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to analyze query plans: %v\n", err)
	}
	if err := analyzeTableStatistics(ctx, conn, logFile, config.Tables); err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to analyze table statistics: %v\n", err)
	}
	if err := analyzeIndexUsage(ctx, conn, logFile, config.Tables); err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to analyze index usage: %v\n", err)
	}
	if err := analyzeMemoryUsage(ctx, conn, logFile); err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to analyze memory usage: %v\n", err)
	}
	if err := analyzeMaintenanceStatus(ctx, conn, logFile, config.Tables); err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to analyze maintenance status: %v\n", err)
	}
	if err := analyzeConcurrentQueries(ctx, conn, logFile, config.QueryPatterns); err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to analyze concurrent queries: %v\n", err)
	}
	if err := analyzePerQueryIO(ctx, conn, logFile, config.TestFunctions); err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to analyze per-query IO: %v\n", err)
	}
	if err := analyzeTableBloat(ctx, conn, logFile, config.Tables); err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to analyze table bloat: %v\n", err)
	}
	if err := analyzeOSDiskMetrics(logFile); err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to analyze OS disk metrics: %v\n", err)
	}
	if err := analyzeCPUMemory(logFile); err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to analyze CPU memory: %v\n", err)
	}
	if err := analyzeWALCheckpoint(ctx, conn, logFile); err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to analyze WAL checkpoint: %v\n", err)
	}
	if err := analyzeRefreshScreenerPlan(ctx, conn, logFile); err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to analyze refresh screener plan: %v\n", err)
	}
	if err := analyzePgStatIO(ctx, conn, logFile); err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to analyze pg_stat_io: %v\n", err)
	}
	if err := analyzeContinuousAggLag(ctx, conn, logFile); err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to analyze continuous agg lag: %v\n", err)
	}
	_, err = conn.DB.Exec(ctx, "SET log_checkpoints = on")
	if err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to set log_checkpoints: %v\n", err)
	}

	// Run query performance analysis if applicable
	if len(items) > 0 && (len(config.TestFunctions) > 0 || len(config.ComponentTests) > 0) {
		if err := analyzeQueryPerformance(ctx, conn, logFile, config.TestFunctions, config.ComponentTests, items); err != nil {
			fmt.Fprintf(logFile, "⚠️  Failed to analyze query performance: %v\n", err)
		}
	}

	fmt.Fprintf(logFile, "\n📊 Analysis complete at %s\n", time.Now().Format("2006-01-02 15:04:05"))

	if err := logFile.Sync(); err != nil {
		log.Printf("⚠️  Failed to sync log file: %v", err)
	}

	log.Printf("📊 Performance analysis complete - logs written to: %s", logFilePath)

	return nil
}

// analyzeDatabaseConfiguration analyzes database configuration and version
func analyzeDatabaseConfiguration(ctx context.Context, conn *data.Conn, logFile *os.File) error {
	fmt.Fprintln(logFile, "📊 Database Configuration Analysis:")

	// Get database version
	var version string
	err := conn.DB.QueryRow(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		return fmt.Errorf("failed to get database version: %v", err)
	}
	fmt.Fprintf(logFile, "📊 Database Version: %s\n", version)

	// Get key configuration parameters
	configQuery := `
		SELECT name, setting, unit, context, source
		FROM pg_settings 
		WHERE name IN (
			'shared_buffers', 'work_mem', 'maintenance_work_mem', 'effective_cache_size',
			'random_page_cost', 'seq_page_cost', 'cpu_tuple_cost', 'cpu_index_tuple_cost',
			'cpu_operator_cost', 'effective_io_concurrency', 'max_worker_processes',
			'max_parallel_workers_per_gather', 'max_parallel_workers', 'wal_buffers',
			'checkpoint_completion_target', 'synchronous_commit', 'default_statistics_target'
		)
		ORDER BY name
	`

	rows, err := conn.DB.Query(ctx, configQuery)
	if err != nil {
		return fmt.Errorf("failed to get configuration: %v", err)
	}
	defer rows.Close()

	fmt.Fprintln(logFile, "📊 Key Configuration Parameters:")
	for rows.Next() {
		var name, setting, unit, context, source string
		if err := rows.Scan(&name, &setting, &unit, &context, &source); err != nil {
			continue
		}
		fmt.Fprintf(logFile, "📊   %s: %s %s (context: %s, source: %s)\n", name, setting, unit, context, source)
	}

	// Get database size info
	sizeQuery := `
		SELECT 
			pg_size_pretty(pg_database_size(current_database())) as db_size,
			pg_size_pretty(pg_total_relation_size('ohlcv_1m')) as ohlcv_1m_size,
			pg_size_pretty(pg_total_relation_size('ohlcv_1d')) as ohlcv_1d_size,
			pg_size_pretty(pg_total_relation_size('screener')) as screener_size
	`

	var dbSize, ohlcv1mSize, ohlcv1dSize, screenerSize string
	err = conn.DB.QueryRow(ctx, sizeQuery).Scan(&dbSize, &ohlcv1mSize, &ohlcv1dSize, &screenerSize)
	if err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to get size info: %v\n", err)
	} else {
		fmt.Fprintf(logFile, "📊 Database Size: %s\n", dbSize)
		fmt.Fprintf(logFile, "📊 OHLCV 1m Size: %s\n", ohlcv1mSize)
		fmt.Fprintf(logFile, "📊 OHLCV 1d Size: %s\n", ohlcv1dSize)
		fmt.Fprintf(logFile, "📊 Screener Size: %s\n", screenerSize)
	}

	fmt.Fprintln(logFile, "")
	return nil
}

// analyzeLockActivity analyzes current lock activity and blocking queries
func analyzeLockActivity(ctx context.Context, conn *data.Conn, logFile *os.File, patterns []string) error {
	fmt.Fprintln(logFile, "📊 Lock Activity and Blocking Analysis (Screener-Related):")

	// Build filter for both blocked and blocking
	filter := ""
	if len(patterns) > 0 {
		var conds []string
		for _, p := range patterns {
			escP := strings.ReplaceAll(p, "'", "''")
			conds = append(conds, fmt.Sprintf("(blocked_activity.query ILIKE '%%%s%%' OR blocking_activity.query ILIKE '%%%s%%')", escP, escP))
		}
		filter = " AND (" + strings.Join(conds, " OR ") + ")"
	}

	lockQuery := `
		SELECT 
			blocked_locks.pid     AS blocked_pid,
			blocked_activity.usename  AS blocked_user,
			blocking_locks.pid     AS blocking_pid,
			blocking_activity.usename AS blocking_user,
			blocked_activity.query    AS blocked_statement,
			blocking_activity.query   AS current_statement_in_blocking_process,
			now() - blocked_activity.query_start AS blocked_duration
		FROM  pg_catalog.pg_locks         blocked_locks
		JOIN pg_catalog.pg_stat_activity blocked_activity  ON blocked_activity.pid = blocked_locks.pid
		JOIN pg_catalog.pg_locks         blocking_locks 
			ON blocking_locks.locktype = blocked_locks.locktype
			AND blocking_locks.database IS NOT DISTINCT FROM blocked_locks.database
			AND blocking_locks.relation IS NOT DISTINCT FROM blocked_locks.relation
			AND blocking_locks.page IS NOT DISTINCT FROM blocked_locks.page
			AND blocking_locks.tuple IS NOT DISTINCT FROM blocked_locks.tuple
			AND blocking_locks.virtualxid IS NOT DISTINCT FROM blocked_locks.virtualxid
			AND blocking_locks.transactionid IS NOT DISTINCT FROM blocked_locks.transactionid
			AND blocking_locks.classid IS NOT DISTINCT FROM blocked_locks.classid
			AND blocking_locks.objid IS NOT DISTINCT FROM blocked_locks.objid
			AND blocking_locks.objsubid IS NOT DISTINCT FROM blocked_locks.objsubid
			AND blocking_locks.pid != blocked_locks.pid 
		JOIN pg_catalog.pg_stat_activity blocking_activity ON blocking_activity.pid = blocking_locks.pid
		WHERE NOT blocked_locks.granted
` + filter + `
		ORDER BY blocked_duration DESC
		LIMIT 10
	`

	rows, err := conn.DB.Query(ctx, lockQuery)
	if err != nil {
		return fmt.Errorf("failed to analyze locks: %v", err)
	}
	defer rows.Close()

	lockCount := 0
	for rows.Next() {
		var blockedPid, blockingPid int
		var blockedUser, blockingUser, blockedStmt, blockingStmt string
		var duration time.Duration

		if err := rows.Scan(&blockedPid, &blockedUser, &blockingPid, &blockingUser, &blockedStmt, &blockingStmt, &duration); err != nil {
			continue
		}

		lockCount++
		fmt.Fprintf(logFile, "📊 Blocking #%d: Blocked PID %d (%s) by PID %d (%s), Duration: %v\n", lockCount, blockedPid, blockedUser, blockingPid, blockingUser, duration)
		fmt.Fprintf(logFile, "📊   Blocked Query: %s\n", blockedStmt[:min(len(blockedStmt), 100)])
		fmt.Fprintf(logFile, "📊   Blocking Query: %s\n", blockingStmt[:min(len(blockingStmt), 100)])
		fmt.Fprintln(logFile, "📊   ---")
	}

	if lockCount == 0 {
		fmt.Fprintln(logFile, "📊 No blocking locks found")
	}

	fmt.Fprintln(logFile, "")
	return nil
}

// analyzeWaitEvents analyzes what the database is waiting for
func analyzeWaitEvents(ctx context.Context, conn *data.Conn, logFile *os.File, patterns []string) error {
	fmt.Fprintln(logFile, "📊 Wait Events Analysis (Screener-Related):")

	// Build filter
	filter := ""
	if len(patterns) > 0 {
		var conds []string
		for _, p := range patterns {
			conds = append(conds, fmt.Sprintf("query ILIKE '%%%s%%'", strings.ReplaceAll(p, "'", "''")))
		}
		filter = " AND (" + strings.Join(conds, " OR ") + ")"
	}

	waitQuery := `
		SELECT 
			wait_event_type,
			wait_event,
			COUNT(*) as count,
			AVG(EXTRACT(EPOCH FROM now() - query_start)) as avg_wait_time
		FROM pg_stat_activity
		WHERE wait_event IS NOT NULL
		  AND state = 'active'
` + filter + `
		GROUP BY wait_event_type, wait_event
		ORDER BY count DESC, avg_wait_time DESC
		LIMIT 10
	`

	rows, err := conn.DB.Query(ctx, waitQuery)
	if err != nil {
		return fmt.Errorf("failed to analyze wait events: %v", err)
	}
	defer rows.Close()

	waitCount := 0
	for rows.Next() {
		var waitEventType, waitEvent string
		var count int
		var avgWaitTime float64

		if err := rows.Scan(&waitEventType, &waitEvent, &count, &avgWaitTime); err != nil {
			continue
		}

		waitCount++
		fmt.Fprintf(logFile, "📊 Wait Event: %s:%s (count: %d, avg: %.2fs)\n",
			waitEventType, waitEvent, count, avgWaitTime)
	}

	if waitCount == 0 {
		fmt.Fprintln(logFile, "📊 No wait events found")
	}

	fmt.Fprintln(logFile, "")
	return nil
}

// analyzeQueryPlans analyzes execution plans for provided queries
func analyzeQueryPlans(ctx context.Context, conn *data.Conn, logFile *os.File, queries []TestQuery) error {
	fmt.Fprintln(logFile, "📊 Query Plans Analysis:")

	for _, q := range queries {
		planQuery := fmt.Sprintf(`EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) %s`, q.Query)

		var planJSON string
		err := conn.DB.QueryRow(ctx, planQuery).Scan(&planJSON)
		if err != nil {
			fmt.Fprintf(logFile, "⚠️  Failed to get query plan for %s: %v\n", q.Name, err)
			continue
		}

		fmt.Fprintf(logFile, "📊 Execution Plan for %s:\n", q.Name)
		fmt.Fprintf(logFile, "📊 %s\n", planJSON)
		fmt.Fprintln(logFile, "📊   ---")
	}

	if len(queries) == 0 {
		fmt.Fprintln(logFile, "📊 No queries provided for plan analysis")
	}

	fmt.Fprintln(logFile, "")
	return nil
}

// analyzeTableStatistics analyzes table and index statistics for provided tables
func analyzeTableStatistics(ctx context.Context, conn *data.Conn, logFile *os.File, tables []string) error {
	fmt.Fprintln(logFile, "📊 Table Statistics Analysis:")

	if len(tables) == 0 {
		fmt.Fprintln(logFile, "📊 No tables provided for analysis")
		return nil
	}

	tableStatsQuery := `
		SELECT 
			schemaname,
			relname,
			n_tup_ins,
			n_tup_upd,
			n_tup_del,
			n_live_tup,
			n_dead_tup,
			last_vacuum,
			last_autovacuum,
			last_analyze,
			last_autoanalyze,
			vacuum_count,
			autovacuum_count,
			analyze_count,
			autoanalyze_count
		FROM pg_stat_user_tables
		WHERE relname = ANY($1)
		ORDER BY n_live_tup DESC
	`

	rows, err := conn.DB.Query(ctx, tableStatsQuery, tables)
	if err != nil {
		return fmt.Errorf("failed to get table statistics: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var schemaName, tableName string
		var nTupIns, nTupUpd, nTupDel, nLiveTup, nDeadTup int64
		var lastVacuum, lastAutovacuum, lastAnalyze, lastAutoanalyze *time.Time
		var vacuumCount, autovacuumCount, analyzeCount, autoanalyzeCount int64

		if err := rows.Scan(&schemaName, &tableName, &nTupIns, &nTupUpd, &nTupDel, &nLiveTup, &nDeadTup,
			&lastVacuum, &lastAutovacuum, &lastAnalyze, &lastAutoanalyze,
			&vacuumCount, &autovacuumCount, &analyzeCount, &autoanalyzeCount); err != nil {
			continue
		}

		fmt.Fprintf(logFile, "📊 Table: %s.%s\n", schemaName, tableName)
		fmt.Fprintf(logFile, "📊   Live tuples: %d, Dead tuples: %d (%.2f%% dead)\n",
			nLiveTup, nDeadTup, float64(nDeadTup)/float64(max64(nLiveTup+nDeadTup, 1))*100)
		fmt.Fprintf(logFile, "📊   Inserts: %d, Updates: %d, Deletes: %d\n", nTupIns, nTupUpd, nTupDel)

		if lastVacuum != nil {
			fmt.Fprintf(logFile, "📊   Last vacuum: %v\n", *lastVacuum)
		}
		if lastAutovacuum != nil {
			fmt.Fprintf(logFile, "📊   Last autovacuum: %v\n", *lastAutovacuum)
		}
		if lastAnalyze != nil {
			fmt.Fprintf(logFile, "📊   Last analyze: %v\n", *lastAnalyze)
		}
		if lastAutoanalyze != nil {
			fmt.Fprintf(logFile, "📊   Last autoanalyze: %v\n", *lastAutoanalyze)
		}

		fmt.Fprintf(logFile, "📊   Vacuum count: %d, Autovacuum count: %d\n", vacuumCount, autovacuumCount)
		fmt.Fprintf(logFile, "📊   Analyze count: %d, Autoanalyze count: %d\n", analyzeCount, autoanalyzeCount)
		fmt.Fprintln(logFile, "📊   ---")
	}

	fmt.Fprintln(logFile, "")
	return nil
}

// analyzeIndexUsage analyzes index usage and effectiveness for provided tables
func analyzeIndexUsage(ctx context.Context, conn *data.Conn, logFile *os.File, tables []string) error {
	fmt.Fprintln(logFile, "📊 Index Usage Analysis:")

	if len(tables) == 0 {
		fmt.Fprintln(logFile, "📊 No tables provided for index analysis")
		return nil
	}

	indexQuery := `
		SELECT 
			schemaname,
			relname,
			indexname,
			idx_scan,
			idx_tup_read,
			idx_tup_fetch,
			pg_size_pretty(pg_relation_size(indexrelid)) as index_size
		FROM pg_stat_user_indexes
		WHERE relname = ANY($1)
		ORDER BY idx_scan DESC
		LIMIT 20
	`

	rows, err := conn.DB.Query(ctx, indexQuery, tables)
	if err != nil {
		return fmt.Errorf("failed to get index usage: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var schemaName, tableName, indexName, indexSize string
		var idxScan, idxTupRead, idxTupFetch int64

		if err := rows.Scan(&schemaName, &tableName, &indexName, &idxScan, &idxTupRead, &idxTupFetch, &indexSize); err != nil {
			continue
		}

		fmt.Fprintf(logFile, "📊 Index: %s on %s.%s\n", indexName, schemaName, tableName)
		fmt.Fprintf(logFile, "📊   Scans: %d, Tuples read: %d, Tuples fetched: %d\n", idxScan, idxTupRead, idxTupFetch)
		fmt.Fprintf(logFile, "📊   Size: %s\n", indexSize)

		if idxScan > 0 {
			fmt.Fprintf(logFile, "📊   Avg tuples per scan: %.2f\n", float64(idxTupRead)/float64(idxScan))
		}
		if idxScan == 0 {
			fmt.Fprintf(logFile, "📊   ⚠️  Index not being used!\n")
		}
		fmt.Fprintln(logFile, "📊   ---")
	}

	fmt.Fprintln(logFile, "")
	return nil
}

// analyzeMemoryUsage analyzes memory usage and temp file creation
func analyzeMemoryUsage(ctx context.Context, conn *data.Conn, logFile *os.File) error {
	fmt.Fprintln(logFile, "📊 Memory Usage Analysis:")

	// Get temp file usage
	tempQuery := `
		SELECT 
			datname,
			temp_files,
			temp_bytes,
			pg_size_pretty(temp_bytes) as temp_size
		FROM pg_stat_database
		WHERE datname = current_database()
	`

	var datname, tempSize string
	var tempFiles, tempBytes int64
	err := conn.DB.QueryRow(ctx, tempQuery).Scan(&datname, &tempFiles, &tempBytes, &tempSize)
	if err != nil {
		return fmt.Errorf("failed to get temp file usage: %v", err)
	}

	fmt.Fprintf(logFile, "📊 Database: %s\n", datname)
	fmt.Fprintf(logFile, "📊 Temp files created: %d\n", tempFiles)
	fmt.Fprintf(logFile, "📊 Temp bytes used: %s\n", tempSize)

	if tempFiles > 0 {
		fmt.Fprintf(logFile, "📊 ⚠️  Temp files indicate work_mem may be too small!\n")
	}

	// Get buffer stats - check PostgreSQL version to handle column moves
	// Check if pg_stat_checkpointer exists (PostgreSQL 17+)
	var checkpointerExists bool
	err = conn.DB.QueryRow(ctx, "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'pg_stat_checkpointer')").Scan(&checkpointerExists)
	if err != nil {
		checkpointerExists = false
	}

	if checkpointerExists {
		// PostgreSQL 17+ - some columns moved to pg_stat_io
		bufferQuery := `
			SELECT 
				buffers_clean,
				buffers_alloc
			FROM pg_stat_bgwriter
		`

		var buffersClean, buffersAlloc int64
		err = conn.DB.QueryRow(ctx, bufferQuery).Scan(&buffersClean, &buffersAlloc)
		if err != nil {
			fmt.Fprintf(logFile, "⚠️  Failed to get buffer stats: %v\n", err)
		} else {
			fmt.Fprintf(logFile, "📊 Buffer stats:\n")
			fmt.Fprintf(logFile, "📊   Clean: %d, Allocated: %d\n", buffersClean, buffersAlloc)

			// Try to get backend buffer stats from pg_stat_io
			var buffersBackend, buffersBackendFsync int64
			err = conn.DB.QueryRow(ctx, `
				SELECT 
					COALESCE(SUM(writes), 0) as buffers_backend,
					COALESCE(SUM(fsyncs), 0) as buffers_backend_fsync
				FROM pg_stat_io
				WHERE backend_type = 'client backend'
			`).Scan(&buffersBackend, &buffersBackendFsync)
			if err != nil {
				fmt.Fprintf(logFile, "📊   Backend buffer stats not available: %v\n", err)
			} else {
				fmt.Fprintf(logFile, "📊   Backend: %d, Backend fsync: %d\n", buffersBackend, buffersBackendFsync)
			}
		}
	} else {
		// PostgreSQL 16 and earlier - all columns in pg_stat_bgwriter
		bufferQuery := `
			SELECT 
				buffers_clean,
				buffers_backend,
				buffers_backend_fsync,
				buffers_alloc
			FROM pg_stat_bgwriter
		`

		var buffersClean, buffersBackend, buffersBackendFsync, buffersAlloc int64
		err = conn.DB.QueryRow(ctx, bufferQuery).Scan(&buffersClean, &buffersBackend, &buffersBackendFsync, &buffersAlloc)
		if err != nil {
			fmt.Fprintf(logFile, "⚠️  Failed to get buffer stats: %v\n", err)
		} else {
			fmt.Fprintf(logFile, "📊 Buffer stats:\n")
			fmt.Fprintf(logFile, "📊   Clean: %d, Backend: %d\n", buffersClean, buffersBackend)
			fmt.Fprintf(logFile, "📊   Backend fsync: %d, Allocated: %d\n", buffersBackendFsync, buffersAlloc)
		}
	}

	fmt.Fprintln(logFile, "")
	return nil
}

// analyzeMaintenanceStatus analyzes vacuum and analyze status for provided tables
func analyzeMaintenanceStatus(ctx context.Context, conn *data.Conn, logFile *os.File, tables []string) error {
	fmt.Fprintln(logFile, "📊 Maintenance Status Analysis:")

	if len(tables) == 0 {
		fmt.Fprintln(logFile, "📊 No tables provided for maintenance analysis")
		return nil
	}

	maintenanceQuery := `
		SELECT 
			schemaname,
			relname,
			n_dead_tup,
			n_live_tup,
			CASE 
				WHEN n_live_tup = 0 THEN 0
				ELSE (n_dead_tup::float / n_live_tup::float) * 100
			END as dead_tuple_pct,
			last_vacuum,
			last_autovacuum,
			last_analyze,
			last_autoanalyze
		FROM pg_stat_user_tables
		WHERE relname = ANY($1)
		ORDER BY dead_tuple_pct DESC
	`

	rows, err := conn.DB.Query(ctx, maintenanceQuery, tables)
	if err != nil {
		return fmt.Errorf("failed to get maintenance status: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var schemaName, tableName string
		var nDeadTup, nLiveTup int64
		var deadTuplePct float64
		var lastVacuum, lastAutovacuum, lastAnalyze, lastAutoanalyze *time.Time

		if err := rows.Scan(&schemaName, &tableName, &nDeadTup, &nLiveTup, &deadTuplePct,
			&lastVacuum, &lastAutovacuum, &lastAnalyze, &lastAutoanalyze); err != nil {
			continue
		}

		fmt.Fprintf(logFile, "📊 Table: %s.%s\n", schemaName, tableName)
		fmt.Fprintf(logFile, "📊   Dead tuples: %d (%.2f%% of live)\n", nDeadTup, deadTuplePct)

		if deadTuplePct > 20 {
			fmt.Fprintf(logFile, "📊   ⚠️  High dead tuple percentage - needs vacuum!\n")
		}

		now := time.Now()
		if lastVacuum != nil {
			fmt.Fprintf(logFile, "📊   Last vacuum: %v (%v ago)\n", *lastVacuum, now.Sub(*lastVacuum))
		}
		if lastAutovacuum != nil {
			fmt.Fprintf(logFile, "📊   Last autovacuum: %v (%v ago)\n", *lastAutovacuum, now.Sub(*lastAutovacuum))
		}
		if lastAnalyze != nil {
			fmt.Fprintf(logFile, "📊   Last analyze: %v (%v ago)\n", *lastAnalyze, now.Sub(*lastAnalyze))
		}
		if lastAutoanalyze != nil {
			fmt.Fprintf(logFile, "📊   Last autoanalyze: %v (%v ago)\n", *lastAutoanalyze, now.Sub(*lastAutoanalyze))
		}

		fmt.Fprintln(logFile, "📊   ---")
	}

	fmt.Fprintln(logFile, "")
	return nil
}

// analyzeConcurrentQueries analyzes concurrent query impact using provided patterns
func analyzeConcurrentQueries(ctx context.Context, conn *data.Conn, logFile *os.File, patterns []string) error {
	fmt.Fprintln(logFile, "📊 Concurrent Query Impact Analysis (Screener-Related):")

	// Build filter
	filter := ""
	if len(patterns) > 0 {
		var conds []string
		for _, p := range patterns {
			conds = append(conds, fmt.Sprintf("query ILIKE '%%%s%%'", strings.ReplaceAll(p, "'", "''")))
		}
		filter = " AND (" + strings.Join(conds, " OR ") + ")"
	}

	specificQuery := `
		SELECT 
			pid,
			usename,
			state,
			wait_event_type,
			wait_event,
			EXTRACT(EPOCH FROM now() - query_start) as duration,
			left(query, 100) as query_start
		FROM pg_stat_activity
		WHERE state != 'idle'
		  AND query NOT LIKE '%pg_stat_activity%'
		  AND pid != pg_backend_pid()
` + filter + `
		ORDER BY duration DESC
	`

	rows, err := conn.DB.Query(ctx, specificQuery)
	if err != nil {
		return fmt.Errorf("failed to get concurrent queries: %v", err)
	}
	defer rows.Close()

	totalActive := 0
	activeQueries := 0
	waitingQueries := 0
	var totalDuration float64

	queryCount := 0
	for rows.Next() {
		var pid int
		var username, state, queryStart string
		var waitEventType, waitEvent *string
		var duration float64

		if err := rows.Scan(&pid, &username, &state, &waitEventType, &waitEvent, &duration, &queryStart); err != nil {
			continue
		}

		totalActive++
		if state == "active" {
			activeQueries++
		}
		if waitEvent != nil {
			waitingQueries++
		}
		totalDuration += duration

		queryCount++
		waitInfo := "None"
		if waitEventType != nil && waitEvent != nil {
			waitInfo = fmt.Sprintf("%s:%s", *waitEventType, *waitEvent)
		}

		fmt.Fprintf(logFile, "📊 Concurrent Query #%d:\n", queryCount)
		fmt.Fprintf(logFile, "📊   PID: %d, User: %s, State: %s, Duration: %.2fs\n", pid, username, state, duration)
		fmt.Fprintf(logFile, "📊   Wait: %s\n", waitInfo)
		fmt.Fprintf(logFile, "📊   Query: %s\n", queryStart)
		fmt.Fprintln(logFile, "📊   ---")
	}

	avgDuration := 0.0
	if totalActive > 0 {
		avgDuration = totalDuration / float64(totalActive)
	}

	fmt.Fprintf(logFile, "📊 Total screener-related connections: %d\n", totalActive)
	fmt.Fprintf(logFile, "📊 Active queries: %d\n", activeQueries)
	fmt.Fprintf(logFile, "📊 Waiting queries: %d\n", waitingQueries)
	fmt.Fprintf(logFile, "📊 Average query duration: %.2f seconds\n", avgDuration)

	if waitingQueries > 0 {
		fmt.Fprintf(logFile, "📊 ⚠️  %d queries are waiting - possible contention!\n", waitingQueries)
	}

	fmt.Fprintln(logFile, "")
	return nil
}

// analyzeQueryPerformance runs performance tests on provided functions and component queries
func analyzeQueryPerformance(ctx context.Context, conn *data.Conn, logFile *os.File, testFunctions []TestQuery, componentTests []TestQuery, items []string) error {
	fmt.Fprintln(logFile, "📊 Query Performance Analysis:")

	if len(items) == 0 {
		fmt.Fprintln(logFile, "📊 No items provided for component tests")
	}

	// Test functions (using Exec, no params)
	for _, fn := range testFunctions {
		fmt.Fprintf(logFile, "📊 Testing %s...\n", fn.Name)

		start := time.Now()
		_, err := conn.DB.Exec(ctx, fn.Query)
		duration := time.Since(start)

		if err != nil {
			fmt.Fprintf(logFile, "📊   ❌ Failed: %v\n", err)
		} else {
			fmt.Fprintf(logFile, "📊   ✅ Success: %v\n", duration)
		}
		fmt.Fprintln(logFile, "📊   ---")
	}

	// Test components (using Query with sample items)
	sampleItems := items[:min(len(items), 3)]
	batchSize := len(items)
	if batchSize > 10 {
		batchSize = 10 // Limit batch size for testing
	}

	for _, test := range componentTests {
		start := time.Now()

		// Handle queries that need additional parameters
		var rows pgx.Rows
		var err error
		if test.Name == "batch_stale_processing" {
			// This query needs both ticker array and batch size limit
			rows, err = conn.DB.Query(ctx, test.Query, sampleItems, batchSize)
		} else {
			// Standard queries with just ticker array
			rows, err = conn.DB.Query(ctx, test.Query, sampleItems)
		}

		if err != nil {
			fmt.Fprintf(logFile, "📊 %s: ❌ %v\n", test.Name, err)
			continue
		}

		rowCount := 0
		for rows.Next() {
			rowCount++
		}
		rows.Close()

		duration := time.Since(start)
		msPerRow := float64(duration.Milliseconds()) / float64(max(rowCount, 1))
		fmt.Fprintf(logFile, "📊 %s: ✅ %v (%d rows, %.2f ms per row)\n",
			test.Name, duration, rowCount, msPerRow)
	}

	fmt.Fprintln(logFile, "")
	return nil
}

// analyzePgStatStatements analyzes query performance using pg_stat_statements with provided patterns
func analyzePgStatStatements(ctx context.Context, conn *data.Conn, logFile *os.File, patterns []string) error {
	fmt.Fprintln(logFile, "📊 Query Performance Statistics Analysis:")

	if len(patterns) == 0 {
		fmt.Fprintln(logFile, "📊 No query patterns provided")
		return nil
	}

	// Check available columns
	checkColumnsQuery := `
		SELECT 
			CASE WHEN EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'pg_stat_statements' AND column_name = 'total_time') THEN 'total_time' ELSE 'total_exec_time' END AS time_column,
			CASE WHEN EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'pg_stat_statements' AND column_name = 'mean_time') THEN 'mean_time' ELSE 'mean_exec_time' END AS mean_column,
			EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'pg_stat_statements' AND column_name = 'blk_read_time') AS has_io_timing,
			EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'pg_stat_statements' AND column_name = 'max_exec_time') AS has_max_exec_time
	`

	var timeColumn, meanColumn string
	var hasIoTiming, hasMaxExecTime bool
	err := conn.DB.QueryRow(ctx, checkColumnsQuery).Scan(&timeColumn, &meanColumn, &hasIoTiming, &hasMaxExecTime)
	if err != nil {
		timeColumn = "total_exec_time"
		meanColumn = "mean_exec_time"
		hasIoTiming = false
		hasMaxExecTime = true
		fmt.Fprintf(logFile, "⚠️  Failed to check pg_stat_statements columns, using fallback: %v\n", err)
	}

	// Build select clause
	maxExecClause := "NULL as max_exec_time, NULL as min_exec_time,"
	if hasMaxExecTime {
		maxExecClause = "max_exec_time, min_exec_time,"
	}

	ioTimingClause := "NULL as blk_read_time, NULL as blk_write_time"
	if hasIoTiming {
		ioTimingClause = "blk_read_time, blk_write_time"
	}

	selectClause := fmt.Sprintf(`
		SELECT 
			substring(query for 100) as short_query,
			calls,
			%s as total_time,
			%s as mean_time,
			%s
			rows,
			100.0 * shared_blks_hit / nullif(shared_blks_hit + shared_blks_read, 0) AS hit_percent,
			shared_blks_read,
			shared_blks_hit,
			shared_blks_dirtied,
			shared_blks_written,
			local_blks_read,
			local_blks_hit,
			local_blks_dirtied,
			local_blks_written,
			temp_blks_read,
			temp_blks_written,
			%s
	`, timeColumn, meanColumn, maxExecClause, ioTimingClause)

	// Build where clause
	conditions := []string{}
	for _, p := range patterns {
		conditions = append(conditions, fmt.Sprintf("query ILIKE '%%%s%%'", p))
	}
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " OR ")
	}

	pgStatQuery := fmt.Sprintf("%s\nFROM pg_stat_statements\n%s\nORDER BY %s DESC\nLIMIT 20", selectClause, whereClause, timeColumn)

	rows, err := conn.DB.Query(ctx, pgStatQuery)
	if err != nil {
		return fmt.Errorf("failed to query pg_stat_statements: %v", err)
	}
	defer rows.Close()

	fmt.Fprintln(logFile, "📊 Top matching queries by total time:")
	queryCount := 0
	for rows.Next() {
		var shortQuery string
		var calls int64
		var totalTime, meanTime float64
		var maxTime, minTime *float64
		var rowsProcessed int64
		var hitPercent *float64
		var sharedBlksRead, sharedBlksHit, sharedBlksDirtied, sharedBlksWritten int64
		var localBlksRead, localBlksHit, localBlksDirtied, localBlksWritten int64
		var tempBlksRead, tempBlksWritten int64
		var blkReadTime, blkWriteTime *float64

		if err := rows.Scan(&shortQuery, &calls, &totalTime, &meanTime, &maxTime, &minTime,
			&rowsProcessed, &hitPercent, &sharedBlksRead, &sharedBlksHit, &sharedBlksDirtied,
			&sharedBlksWritten, &localBlksRead, &localBlksHit, &localBlksDirtied, &localBlksWritten,
			&tempBlksRead, &tempBlksWritten, &blkReadTime, &blkWriteTime); err != nil {
			continue
		}

		queryCount++
		hitPercentStr := "N/A"
		if hitPercent != nil {
			hitPercentStr = fmt.Sprintf("%.2f%%", *hitPercent)
		}

		maxTimeStr := "N/A"
		if maxTime != nil {
			maxTimeStr = fmt.Sprintf("%.2fms", *maxTime)
		}

		readTimeStr := "N/A"
		writeTimeStr := "N/A"
		if blkReadTime != nil {
			readTimeStr = fmt.Sprintf("%.2fms", *blkReadTime)
		}
		if blkWriteTime != nil {
			writeTimeStr = fmt.Sprintf("%.2fms", *blkWriteTime)
		}

		fmt.Fprintf(logFile, "📊 Query #%d: %s\n", queryCount, shortQuery)
		fmt.Fprintf(logFile, "📊   Performance: %d calls, %.2fms total, %.2fms mean, %s max\n",
			calls, totalTime, meanTime, maxTimeStr)
		fmt.Fprintf(logFile, "📊   Rows: %d processed (%.2f rows/call)\n",
			rowsProcessed, float64(rowsProcessed)/float64(max64(calls, 1)))
		fmt.Fprintf(logFile, "📊   Cache: %s hit ratio, %d reads, %d hits\n",
			hitPercentStr, sharedBlksRead, sharedBlksHit)

		if calls > 0 && meanTime > 1000 {
			fmt.Fprintf(logFile, "📊   ⚠️  SLOW: Mean time > 1s - optimization needed!\n")
		}
		if hitPercent != nil && *hitPercent < 90 {
			fmt.Fprintf(logFile, "📊   ⚠️  LOW CACHE HIT: %.2f%% - consider increasing shared_buffers\n", *hitPercent)
		}
		if tempBlksRead > 0 || tempBlksWritten > 0 {
			fmt.Fprintf(logFile, "📊   ⚠️  TEMP FILES: %d read, %d written - increase work_mem\n",
				tempBlksRead, tempBlksWritten)
		}

		if blkReadTime != nil && blkWriteTime != nil {
			fmt.Fprintf(logFile, "📊   I/O: Read %s, Write %s\n", readTimeStr, writeTimeStr)
		}

		fmt.Fprintln(logFile, "📊   ---")
	}

	if queryCount == 0 {
		fmt.Fprintln(logFile, "📊 No matching queries found in pg_stat_statements")
	}

	fmt.Fprintln(logFile, "")
	return nil
}

// analyzeDatabaseActivity analyzes current database activity
func analyzeDatabaseActivity(ctx context.Context, conn *data.Conn, logFile *os.File, patterns []string) error {
	fmt.Fprintln(logFile, "📊 Current Database Activity Analysis (Screener-Related):")

	// Build filter
	filter := ""
	if len(patterns) > 0 {
		var conds []string
		for _, p := range patterns {
			conds = append(conds, fmt.Sprintf("query ILIKE '%%%s%%'", strings.ReplaceAll(p, "'", "''")))
		}
		filter = " AND (" + strings.Join(conds, " OR ") + ")"
	}

	activityQuery := `
		SELECT 
			pid,
			now() - pg_stat_activity.query_start AS duration,
			query,
			state,
			wait_event_type,
			wait_event
		FROM pg_stat_activity 
		WHERE state = 'active' 
		  AND query NOT LIKE '%pg_stat_activity%'
` + filter + `
		ORDER BY duration DESC
		LIMIT 10
	`

	rows, err := conn.DB.Query(ctx, activityQuery)
	if err != nil {
		return fmt.Errorf("failed to query pg_stat_activity: %v", err)
	}
	defer rows.Close()

	fmt.Fprintln(logFile, "📊 Current active queries:")
	for rows.Next() {
		var pid int
		var duration time.Duration
		var query, state string
		var waitEventType, waitEvent *string

		if err := rows.Scan(&pid, &duration, &query, &state, &waitEventType, &waitEvent); err != nil {
			continue
		}

		waitInfo := "None"
		if waitEventType != nil && waitEvent != nil {
			waitInfo = fmt.Sprintf("%s:%s", *waitEventType, *waitEvent)
		}

		fmt.Fprintf(logFile, "📊 PID: %d, Duration: %v, State: %s, Wait: %s\n", pid, duration, state, waitInfo)
		fmt.Fprintf(logFile, "📊   Query: %s\n", query[:min(len(query), 100)])
		fmt.Fprintln(logFile, "📊   ---")
	}

	fmt.Fprintln(logFile, "")
	return nil
}

// analyzePerQueryIO: Use EXPLAIN (ANALYZE, VERBOSE, BUFFERS) on the test queries provided in config.TestFunctions.
func analyzePerQueryIO(ctx context.Context, conn *data.Conn, logFile *os.File, queries []TestQuery) error {
	fmt.Fprintln(logFile, "📊 Per-Query I/O Breakdown Analysis:")

	for _, q := range queries {
		// Replace any references to stale_tickers with screener_stale
		adjustedQuery := strings.ReplaceAll(q.Query, "stale_tickers", "screener_stale")
		planQuery := fmt.Sprintf(`EXPLAIN (ANALYZE, VERBOSE, BUFFERS) %s`, adjustedQuery)

		var planText string
		err := conn.DB.QueryRow(ctx, planQuery).Scan(&planText)
		if err != nil {
			fmt.Fprintf(logFile, "⚠️  Failed to get I/O breakdown for %s: %v\n", q.Name, err)
			continue
		}

		fmt.Fprintf(logFile, "📊 I/O Breakdown for %s:\n", q.Name)
		fmt.Fprintf(logFile, "📊 %s\n", planText)
		fmt.Fprintln(logFile, "📊   ---")
	}

	fmt.Fprintln(logFile, "")
	return nil
}

// analyzeTableBloat: Use pgstattuple extension if available to check bloat for config.Tables.
func analyzeTableBloat(ctx context.Context, conn *data.Conn, logFile *os.File, tables []string) error {
	fmt.Fprintln(logFile, "📊 Table Bloat Analysis:")

	// Check if pgstattuple is installed
	var extInstalled bool
	err := conn.DB.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pgstattuple')").Scan(&extInstalled)
	if err != nil || !extInstalled {
		fmt.Fprintln(logFile, "📊 pgstattuple extension not installed - skipping bloat analysis")
		return nil
	}

	for _, table := range tables {
		bloatQuery := fmt.Sprintf(`
			SELECT 
				table_len,
				tuple_count,
				tuple_len,
				dead_tuple_count,
				dead_tuple_len,
				free_space
			FROM pgstattuple('%s')
		`, table)

		var tableLen, tupleCount, tupleLen, deadTupleCount, deadTupleLen, freeSpace int64
		err = conn.DB.QueryRow(ctx, bloatQuery).Scan(&tableLen, &tupleCount, &tupleLen, &deadTupleCount, &deadTupleLen, &freeSpace)
		if err != nil {
			fmt.Fprintf(logFile, "⚠️  Failed for %s: %v\n", table, err)
			continue
		}

		bloatPct := float64(deadTupleLen+freeSpace) / float64(max64(tableLen, 1)) * 100

		fmt.Fprintf(logFile, "📊 Table: %s\n", table)
		fmt.Fprintf(logFile, "📊   Size: %d bytes\n", tableLen)
		fmt.Fprintf(logFile, "📊   Tuples: %d live, %d dead\n", tupleCount, deadTupleCount)
		fmt.Fprintf(logFile, "📊   Bloat: %.2f%% (dead: %d bytes, free: %d bytes)\n", bloatPct, deadTupleLen, freeSpace)
		if bloatPct > 20 {
			fmt.Fprintln(logFile, "📊   ⚠️  High bloat - consider vacuum full or pg_repack")
		}
		fmt.Fprintln(logFile, "📊   ---")
	}

	fmt.Fprintln(logFile, "")
	return nil
}

// analyzeOSDiskMetrics: Use run_terminal_cmd to run 'iostat -x 1 2' and parse output.
func analyzeOSDiskMetrics(logFile *os.File) error {
	fmt.Fprintln(logFile, "📊 OS-Level Disk Metrics:")

	cmd := exec.Command("iostat", "-x", "1", "2")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to run iostat: %v\n", err)
		return nil
	}

	scanner := bufio.NewScanner(strings.NewReader(out.String()))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "%util") || strings.Contains(line, "avg-cpu") {
			fmt.Fprintf(logFile, "📊 %s\n", line)
		}
	}

	fmt.Fprintln(logFile, "")
	return nil
}

// analyzeCPUMemory: Use run_terminal_cmd to run 'vmstat 1 2' and 'free -h'.
func analyzeCPUMemory(logFile *os.File) error {
	fmt.Fprintln(logFile, "📊 CPU and Memory Metrics:")

	// vmstat
	cmd := exec.Command("vmstat", "1", "2")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to run vmstat: %v\n", err)
	} else {
		fmt.Fprintln(logFile, "📊 vmstat output:")
		fmt.Fprintln(logFile, out.String())
	}

	// free -h
	cmd = exec.Command("free", "-h")
	out.Reset()
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to run free: %v\n", err)
	} else {
		fmt.Fprintln(logFile, "📊 free output:")
		fmt.Fprintln(logFile, out.String())
	}

	fmt.Fprintln(logFile, "")
	return nil
}

// analyzeWALCheckpoint: Query pg_stat_bgwriter and pg_stat_wal.
func analyzeWALCheckpoint(ctx context.Context, conn *data.Conn, logFile *os.File) error {
	fmt.Fprintln(logFile, "📊 WAL and Checkpoint Analysis:")

	// Check if pg_stat_checkpointer exists (PostgreSQL 17+)
	var checkpointerExists bool
	err := conn.DB.QueryRow(ctx, "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'pg_stat_checkpointer')").Scan(&checkpointerExists)
	if err != nil {
		checkpointerExists = false
	}

	if checkpointerExists {
		// PostgreSQL 17+ - use pg_stat_checkpointer
		var checkpointsTimed, checkpointsReq int64
		var checkpointWriteTime, checkpointSyncTime float64
		var buffersCheckpoint int64

		err := conn.DB.QueryRow(ctx, `
			SELECT 
				checkpoints_timed,
				checkpoints_req,
				checkpoint_write_time,
				checkpoint_sync_time,
				buffers_checkpoint
			FROM pg_stat_checkpointer
		`).Scan(&checkpointsTimed, &checkpointsReq, &checkpointWriteTime, &checkpointSyncTime, &buffersCheckpoint)
		if err != nil {
			fmt.Fprintf(logFile, "⚠️  Failed to get checkpointer stats: %v\n", err)
		} else {
			fmt.Fprintln(logFile, "📊 Checkpoints (from pg_stat_checkpointer):")
			fmt.Fprintf(logFile, "📊   Timed: %d, Requested: %d\n", checkpointsTimed, checkpointsReq)
			fmt.Fprintf(logFile, "📊   Write time: %.2fms, Sync time: %.2fms\n", checkpointWriteTime, checkpointSyncTime)
			fmt.Fprintf(logFile, "📊   Buffers written: %d\n", buffersCheckpoint)
		}

		// Get background writer stats from pg_stat_bgwriter
		var buffersClean, maxwrittenClean, buffersAlloc int64
		err = conn.DB.QueryRow(ctx, `
			SELECT 
				buffers_clean,
				maxwritten_clean,
				buffers_alloc
			FROM pg_stat_bgwriter
		`).Scan(&buffersClean, &maxwrittenClean, &buffersAlloc)
		if err != nil {
			fmt.Fprintf(logFile, "⚠️  Failed to get bgwriter stats: %v\n", err)
		} else {
			fmt.Fprintln(logFile, "📊 Background Writer:")
			fmt.Fprintf(logFile, "📊   Clean: %d, Max written clean: %d, Alloc: %d\n", buffersClean, maxwrittenClean, buffersAlloc)
		}

		// Try to get backend buffer stats from pg_stat_io (PostgreSQL 17+)
		var buffersBackend, buffersBackendFsync int64
		err = conn.DB.QueryRow(ctx, `
			SELECT 
				COALESCE(SUM(writes), 0) as buffers_backend,
				COALESCE(SUM(fsyncs), 0) as buffers_backend_fsync
			FROM pg_stat_io
			WHERE backend_type = 'client backend'
		`).Scan(&buffersBackend, &buffersBackendFsync)
		if err != nil {
			fmt.Fprintf(logFile, "📊   Backend buffer stats not available from pg_stat_io: %v\n", err)
		} else {
			fmt.Fprintf(logFile, "📊   Backend: %d, Backend fsync: %d\n", buffersBackend, buffersBackendFsync)
		}
	} else {
		// PostgreSQL 16 and earlier - use pg_stat_bgwriter
		var checkpointsTimed, checkpointsReq int64
		var checkpointWriteTime, checkpointSyncTime float64
		var buffersCheckpoint, buffersClean, maxwrittenClean int64
		var buffersBackend, buffersBackendFsync, buffersAlloc int64

		err := conn.DB.QueryRow(ctx, `
			SELECT 
				checkpoints_timed,
				checkpoints_req,
				checkpoint_write_time,
				checkpoint_sync_time,
				buffers_checkpoint,
				buffers_clean,
				maxwritten_clean,
				buffers_backend,
				buffers_backend_fsync,
				buffers_alloc
			FROM pg_stat_bgwriter
		`).Scan(&checkpointsTimed, &checkpointsReq, &checkpointWriteTime, &checkpointSyncTime,
			&buffersCheckpoint, &buffersClean, &maxwrittenClean, &buffersBackend,
			&buffersBackendFsync, &buffersAlloc)
		if err != nil {
			fmt.Fprintf(logFile, "⚠️  Failed to get bgwriter stats: %v\n", err)
		} else {
			fmt.Fprintln(logFile, "📊 Checkpoints (from pg_stat_bgwriter):")
			fmt.Fprintf(logFile, "📊   Timed: %d, Requested: %d\n", checkpointsTimed, checkpointsReq)
			fmt.Fprintf(logFile, "📊   Write time: %.2fms, Sync time: %.2fms\n", checkpointWriteTime, checkpointSyncTime)
			fmt.Fprintf(logFile, "📊   Buffers: Checkpoint %d, Clean %d, Backend %d, Alloc %d\n",
				buffersCheckpoint, buffersClean, buffersBackend, buffersAlloc)
			fmt.Fprintf(logFile, "📊   Max written clean: %d, Backend fsync: %d\n", maxwrittenClean, buffersBackendFsync)
		}
	}

	// pg_stat_wal (PostgreSQL 13+)
	var walRecords, walFpi, walBytes int64
	err = conn.DB.QueryRow(ctx, `
		SELECT 
			wal_records,
			wal_fpi,
			wal_bytes
		FROM pg_stat_wal
	`).Scan(&walRecords, &walFpi, &walBytes)
	if err == nil {
		fmt.Fprintln(logFile, "📊 WAL Stats:")
		fmt.Fprintf(logFile, "📊   Records: %d, FPI: %d, Bytes: %d\n", walRecords, walFpi, walBytes)
	} else {
		fmt.Fprintf(logFile, "📊 pg_stat_wal not available\n")
	}

	fmt.Fprintln(logFile, "")
	return nil
}

// analyzeRefreshScreenerPlan: Insert 3 synthetic stale tickers, run EXPLAIN (ANALYZE, BUFFERS, WAL) SELECT refresh_screener(3); then clean up.
func analyzeRefreshScreenerPlan(ctx context.Context, conn *data.Conn, logFile *os.File) error {
	fmt.Fprintln(logFile, "📊 Refresh Screener Plan Analysis:")

	// Insert synthetic stale tickers
	_, err := conn.DB.Exec(ctx, `
		INSERT INTO screener_stale (ticker, stale)
		VALUES ('TEST1', TRUE), ('TEST2', TRUE), ('TEST3', TRUE)
		ON CONFLICT (ticker) DO UPDATE SET stale = TRUE
	`)
	if err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to insert synthetic data: %v\n", err)
		return nil
	}

	defer func() {
		_, _ = conn.DB.Exec(ctx, "DELETE FROM screener_stale WHERE ticker LIKE 'TEST%'")
	}()

	// Check if WAL option is available (PostgreSQL 15+)
	var hasWALOption bool
	err = conn.DB.QueryRow(ctx, "SELECT current_setting('server_version_num')::int >= 150000").Scan(&hasWALOption)
	if err != nil {
		hasWALOption = false
	}

	var planQuery string
	if hasWALOption {
		planQuery = "EXPLAIN (ANALYZE, BUFFERS, WAL) SELECT refresh_screener(3)"
	} else {
		planQuery = "EXPLAIN (ANALYZE, BUFFERS) SELECT refresh_screener(3)"
	}

	var planText string
	err = conn.DB.QueryRow(ctx, planQuery).Scan(&planText)
	if err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to explain refresh_screener: %v\n", err)
		return nil
	}

	fmt.Fprintln(logFile, "📊 Plan:")
	fmt.Fprintln(logFile, planText)
	fmt.Fprintln(logFile, "")
	return nil
}

// analyzePgStatIO: Check if pg_stat_io exists (PG17+), query it.
func analyzePgStatIO(ctx context.Context, conn *data.Conn, logFile *os.File) error {
	fmt.Fprintln(logFile, "📊 PG Stat IO Analysis (PG17+):")

	// Check if available
	var exists bool
	err := conn.DB.QueryRow(ctx, "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'pg_stat_io')").Scan(&exists)
	if err != nil || !exists {
		fmt.Fprintln(logFile, "📊 pg_stat_io not available")
		return nil
	}

	rows, err := conn.DB.Query(ctx, `
		SELECT 
			backend_type,
			object,
			context,
			reads,
			read_time,
			writes,
			write_time,
			writebacks,
			extends
		FROM pg_stat_io
		ORDER BY reads + writes DESC
	`)
	if err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed: %v\n", err)
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		var backendType, object, context string
		var reads, writes, writebacks, extends int64
		var readTime, writeTime float64
		if err := rows.Scan(&backendType, &object, &context, &reads, &readTime, &writes, &writeTime, &writebacks, &extends); err != nil {
			continue
		}
		fmt.Fprintf(logFile, "📊 %s - %s (%s): Reads %d (%.2fms), Writes %d (%.2fms)\n", backendType, object, context, reads, readTime, writes, writeTime)
	}

	fmt.Fprintln(logFile, "")
	return nil
}

// analyzeContinuousAggLag: Query timescaledb_information.continuous_aggregate_stats.
func analyzeContinuousAggLag(ctx context.Context, conn *data.Conn, logFile *os.File) error {
	fmt.Fprintln(logFile, "📊 Continuous Aggregate Lag Analysis:")

	// Check if TimescaleDB is installed and the view exists
	var viewExists bool
	err := conn.DB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_schema = 'timescaledb_information' 
			AND table_name = 'continuous_aggregate_stats'
		)
	`).Scan(&viewExists)
	if err != nil || !viewExists {
		fmt.Fprintf(logFile, "⚠️  TimescaleDB continuous_aggregate_stats view not available\n")
		fmt.Fprintln(logFile, "")
		return nil
	}

	rows, err := conn.DB.Query(ctx, `
		SELECT 
			materialization_hypertable,
			lag,
			last_run_duration,
			last_run_status
		FROM timescaledb_information.continuous_aggregate_stats
		ORDER BY lag DESC
	`)
	if err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed: %v\n", err)
		fmt.Fprintln(logFile, "")
		return nil
	}
	defer rows.Close()

	aggCount := 0
	for rows.Next() {
		var hypertable string
		var lag, lastDuration time.Duration
		var lastStatus string
		if err := rows.Scan(&hypertable, &lag, &lastDuration, &lastStatus); err != nil {
			continue
		}
		aggCount++
		fmt.Fprintf(logFile, "📊 %s: Lag %v, Last duration %v, Status %s\n", hypertable, lag, lastDuration, lastStatus)
		if lag > time.Minute*5 {
			fmt.Fprintln(logFile, "⚠️  High lag - check policies")
		}
	}

	if aggCount == 0 {
		fmt.Fprintln(logFile, "📊 No continuous aggregates found")
	}

	fmt.Fprintln(logFile, "")
	return nil
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
