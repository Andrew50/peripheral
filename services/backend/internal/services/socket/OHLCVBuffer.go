package socket

import (
	"backend/internal/data"
	"backend/internal/data/utils"
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/polygon-io/client-go/websocket/models"
)

// CriticalAlertFunc defines a function type for sending critical alerts
type CriticalAlertFunc func(error, ...string) error

// Global variable to hold the critical alert function
var criticalAlertCallback CriticalAlertFunc

// SetCriticalAlertCallback sets the callback function for sending critical alerts
func SetCriticalAlertCallback(callback CriticalAlertFunc) {
	criticalAlertCallback = callback
}

// sendCriticalAlert sends a critical alert if the callback is set
func sendCriticalAlert(err error, functionName string) {
	if criticalAlertCallback != nil {
		_ = criticalAlertCallback(err, functionName)
	}
}

type OHLCVRecord struct {
	Timestamp int64
	Ticker    string
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    int64
}

type OHLCVBuffer struct {
	buffer      []OHLCVRecord
	dbConn      *data.Conn
	mu          sync.Mutex
	lastFlush   time.Time
	stopTimeout chan struct{}
	flushCh     chan []OHLCVRecord
	wg          sync.WaitGroup
	stopped     bool
	writerID    int // Added to identify each writer's staging tables
	// Performance tracking
	totalRecordsAdded     int64
	totalBatchesProcessed int64
	totalRecordsDropped   int64
	lastLogTime           time.Time
}

const flushThreshold = 7500                  // flush when more than this many records in buffer (balanced for throughput)
const flushTimeout = 2 * time.Second         // flush if buffer is older than this (increased to reduce timeout flushes)
const checkInterval = 1 * time.Second        // check for stale buffer every this many seconds (increased from 1s)
const healthCheckInterval = 30 * time.Second // check for staging table health every this many seconds
const channelBufferSize = 50                 // increased buffer to handle more peak buffering (increased from 20)
const criticalChannelThreshold = 20          // send critical alert when channel backlog exceeds this (reduced false alerts)

var ohlcvBuffer *OHLCVBuffer

// verifyStagingTablesExist checks that both staging tables are accessible
// This function works outside of any transaction to avoid 25P02 errors
func (b *OHLCVBuffer) verifyStagingTablesExist() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := b.dbConn.DB.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection for verification: %w", err)
	}
	defer conn.Release()

	// Check worker-specific staging tables exist
	var check1m, check1d *string

	stage1mTable := fmt.Sprintf("public.ohlcv_1m_stage_w%d", b.writerID)
	stage1dTable := fmt.Sprintf("public.ohlcv_1d_stage_w%d", b.writerID)

	err = conn.QueryRow(ctx, "SELECT to_regclass($1)", stage1mTable).Scan(&check1m)
	if err != nil {
		return fmt.Errorf("verify %s existence: %w", stage1mTable, err)
	}
	if check1m == nil {
		return fmt.Errorf("%s table does not exist", stage1mTable)
	}

	err = conn.QueryRow(ctx, "SELECT to_regclass($1)", stage1dTable).Scan(&check1d)
	if err != nil {
		return fmt.Errorf("verify %s existence: %w", stage1dTable, err)
	}
	if check1d == nil {
		return fmt.Errorf("%s table does not exist", stage1dTable)
	}

	// log.Printf("✅ Pre-flight check: Both worker-%d staging tables verified to exist", b.writerID)
	return nil
}

// ensureStagingTablesExist creates tables if they don't exist, with verification
func (b *OHLCVBuffer) ensureStagingTablesExist() error {
	// First, try to verify existing tables
	if err := b.verifyStagingTablesExist(); err != nil {
		log.Printf("⚠️ Staging tables missing or inaccessible: %v", err)
		log.Printf("🔧 Attempting to recreate staging tables...")

		// Tables missing, try to create them
		if createErr := b.createStagingTables(); createErr != nil {
			return fmt.Errorf("failed to create staging tables: %w", createErr)
		}

		// Verify creation was successful
		if verifyErr := b.verifyStagingTablesExist(); verifyErr != nil {
			return fmt.Errorf("staging tables still not accessible after creation: %w", verifyErr)
		}

		// log.Printf("✅ Staging tables successfully recreated and verified")
	}

	return nil
}

// Initialize the OHLCV buffer
func InitOHLCVBuffer(conn *data.Conn) error {

	if ohlcvBuffer != nil {
		log.Printf("⚠️ OHLCV buffer already initialized, stopping existing buffer")
		ohlcvBuffer.Stop()
	}

	ohlcvBuffer = &OHLCVBuffer{
		buffer:      make([]OHLCVRecord, 0, flushThreshold),
		dbConn:      conn,
		lastFlush:   time.Now(),
		stopTimeout: make(chan struct{}),
		flushCh:     make(chan []OHLCVRecord, channelBufferSize),
		stopped:     false,
		lastLogTime: time.Now(),
	}

	// Pre-flight check: Ensure staging tables exist and are accessible
	// log.Printf("🔍 Running pre-flight checks for staging tables...")
	if err := ohlcvBuffer.ensureStagingTablesExist(); err != nil {
		log.Printf("❌ Pre-flight check failed: %v", err)
		return fmt.Errorf("pre-flight staging table check failed: %w", err)
	}

	// log.Printf("🔄 Starting OHLCV buffer writer goroutines...")
	// Start single writer goroutine to avoid lock contention on shared staging tables and target tables
	// Multiple workers were causing TimescaleDB chunk lock conflicts and staging table contention
	numWorkers := 1 // Reduced from 4 to eliminate database lock contention
	for i := 0; i < numWorkers; i++ {
		ohlcvBuffer.wg.Add(1)
		go ohlcvBuffer.writer(i)
	}

	// log.Printf("⏰ Starting OHLCV buffer timeout flusher...")
	ohlcvBuffer.startTimeoutFlusher()

	// log.Printf("🩺 Starting OHLCV buffer health checker...")
	ohlcvBuffer.startHealthChecker()

	// log.Printf("📊 Starting OHLCV buffer performance monitor...")
	ohlcvBuffer.startPerformanceMonitor()

	// log.Printf("✅ OHLCV buffer initialized successfully with channel buffer size: %d batches (single writer to avoid lock contention)", channelBufferSize)
	return nil
}

func (b *OHLCVBuffer) startTimeoutFlusher() {
	ticker := time.NewTicker(checkInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				b.flushIfStale()
			case <-b.stopTimeout:
				return
			}
		}
	}()
}

func (b *OHLCVBuffer) startHealthChecker() {
	ticker := time.NewTicker(healthCheckInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				b.performHealthCheck()
			case <-b.stopTimeout:
				return
			}
		}
	}()
}

func (b *OHLCVBuffer) startPerformanceMonitor() {
	ticker := time.NewTicker(10 * time.Second) // Log performance every 10 seconds
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				b.logPerformanceMetrics()
			case <-b.stopTimeout:
				return
			}
		}
	}()
}

func (b *OHLCVBuffer) logPerformanceMetrics() {
	b.mu.Lock()
	bufferSize := len(b.buffer)
	channelBacklog := len(b.flushCh)
	// totalAdded := b.totalRecordsAdded
	// totalProcessed := b.totalBatchesProcessed
	totalDropped := b.totalRecordsDropped
	b.mu.Unlock()

	channelUtilization := float64(channelBacklog) / float64(channelBufferSize) * 100
	bufferUtilization := float64(bufferSize) / float64(flushThreshold) * 100

	// CRITICAL: Log connection pool stats to detect leaks
	poolStats := b.dbConn.DB.Stat()
	// pgxpool Stats does NOT provide a direct "waiting" metric. The closest proxies are:
	//   ConstructingConns  – number of connections currently being established
	//   EmptyAcquireCount – cumulative # of times Acquire() had to wait because the pool was empty
	// We log both instead of computing a misleading value from AcquireCount.
	constructing := poolStats.ConstructingConns()
	emptyAcquire := poolStats.EmptyAcquireCount()

	// Log pool stats if there are any waiting connections or high utilization
	if constructing > 5 || emptyAcquire > 100 || channelUtilization > 80 || bufferUtilization > 80 || totalDropped > 0 {
		// log.Printf("📊 OHLCV Performance Metrics:")
		// log.Printf("   📈 Buffer: %d/%d records (%.1f%% full)", bufferSize, flushThreshold, bufferUtilization)
		// log.Printf("   🚰 Channel: %d/%d batches queued (%.1f%% full)", channelBacklog, channelBufferSize, channelUtilization)
		// log.Printf("   🔗 DB Pool: total=%d, idle=%d, used=%d, constructing=%d, empty_acquire=%d",
		//	poolStats.TotalConns(), poolStats.IdleConns(), poolStats.AcquiredConns(), constructing, emptyAcquire)
		// log.Printf("   📥 Total records added: %d", totalAdded)
		// log.Printf("   ✅ Total batches processed: %d", totalProcessed)
		// log.Printf("   ❌ Total records dropped: %d", totalDropped)

		// Alert if pool is regularly empty or too many connections are constructing.
		if constructing > 5 {
			log.Printf("⚠️ WARNING: %d connections currently being established – pool may be under-sized or DB slow to accept conns", constructing)
		}
		if emptyAcquire > 100 {
			log.Printf("⚠️ WARNING: pool has had %d empty-acquire events – consider upping Min/MaxConns or investigating long-running queries", emptyAcquire)
		}

		if channelUtilization > 80 {
			log.Printf("⚠️  WARNING: Channel utilization high (%.1f%%) - writer may be falling behind", channelUtilization)
		}
		if bufferUtilization > 80 {
			log.Printf("⚠️  WARNING: Buffer utilization high (%.1f%%) - approaching flush threshold", bufferUtilization)
		}
		if totalDropped > 0 {
			log.Printf("🚨 ALERT: %d records have been dropped due to channel overflow", totalDropped)
		}
	}
}

func (b *OHLCVBuffer) performHealthCheck() {
	if err := b.verifyStagingTablesExist(); err != nil {
		log.Printf("🚨 Health check failed: Staging tables missing: %v", err)
		log.Printf("🔧 Health check: Attempting to recreate staging tables...")

		if createErr := b.ensureStagingTablesExist(); createErr != nil {
			log.Printf("❌ Health check: Failed to recreate staging tables: %v", createErr)
		} else {
			// log.Printf("✅ Health check: Staging tables successfully recreated")
		}
	} else {
		// log.Printf("💚 Health check: Staging tables are healthy")
	}
}

func (b *OHLCVBuffer) flushIfStale() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.stopped {
		return
	}

	if len(b.buffer) > 0 && time.Since(b.lastFlush) > flushTimeout {
		channelBacklog := len(b.flushCh)
		// log.Printf("⏰ Timeout flush triggered: %d records (channel backlog: %d/%d)", len(b.buffer), channelBacklog, channelBufferSize)

		// Send critical alert if channel backlog is high
		if channelBacklog > criticalChannelThreshold {
			alertErr := fmt.Errorf("OHLCV channel backlog critical: %d/%d batches queued, writer falling behind", channelBacklog, channelBufferSize)
			sendCriticalAlert(alertErr, "OHLCVBuffer.flushIfStale")
		}

		batchToFlush := make([]OHLCVRecord, len(b.buffer))
		copy(batchToFlush, b.buffer)
		b.buffer = b.buffer[:0]
		b.lastFlush = time.Now()
		select {
		case b.flushCh <- batchToFlush:
		//	log.Printf("✅ Timeout batch sent to flush channel (new backlog: %d/%d)", len(b.flushCh), channelBufferSize)
		default:
			b.totalRecordsDropped += int64(len(batchToFlush))

			// Send critical alert for dropped records
			alertErr := fmt.Errorf("OHLCV records DROPPED: %d records lost due to timeout flush channel overflow (channel: %d/%d)", len(batchToFlush), channelBacklog, channelBufferSize)
			sendCriticalAlert(alertErr, "OHLCVBuffer.flushIfStale")

			log.Printf("🚨 CRITICAL: Timeout flush channel full, dropping batch of %d records (channel: %d/%d)", len(batchToFlush), channelBacklog, channelBufferSize)
		}
	}
}

// Add a new OHLCV bar to the buffer
func (b *OHLCVBuffer) addBar(timestamp int64, ticker string, bar models.EquityAgg) {
	record := OHLCVRecord{
		Timestamp: timestamp,
		Ticker:    ticker,
		Open:      bar.Open,
		High:      bar.High,
		Low:       bar.Low,
		Close:     bar.Close,
		Volume:    int64(bar.Volume),
	}

	// Debug logging for first few bars or special tickers
	// if ticker == "COIN" || ticker == "AAPL" {
	// log.Printf("🪙 %s bar: ts=%d, O=%.4f, H=%.4f, L=%.4f, C=%.4f, V=%d",
	//	ticker, timestamp, bar.Open, bar.High, bar.Low, bar.Close, int64(bar.Volume))
	// }

	b.mu.Lock()
	if b.stopped {
		log.Printf("⚠️ addBar skipping %s - buffer stopped", ticker)
		b.mu.Unlock()
		return
	}

	b.buffer = append(b.buffer, record)
	b.totalRecordsAdded++
	b.lastFlush = time.Now() // Update lastFlush on every add to prevent unnecessary timeout flushes
	// log.Printf("📈 Added %s to buffer (buffer size: %d/%d)", ticker, len(b.buffer), flushThreshold)

	if len(b.buffer) >= flushThreshold {
		channelBacklog := len(b.flushCh)
		// log.Printf("🚨 Buffer threshold reached, flushing %d records (channel backlog: %d/%d)", len(b.buffer), channelBacklog, channelBufferSize)

		// Send critical alert if channel backlog is high
		if channelBacklog > criticalChannelThreshold {
			alertErr := fmt.Errorf("OHLCV channel backlog critical: %d/%d batches queued, writer falling behind", channelBacklog, channelBufferSize)
			sendCriticalAlert(alertErr, "OHLCVBuffer.addBar")
		}

		batchToFlush := make([]OHLCVRecord, len(b.buffer))
		copy(batchToFlush, b.buffer)
		b.buffer = b.buffer[:0]
		b.lastFlush = time.Now()
		b.mu.Unlock()
		select {
		case b.flushCh <- batchToFlush:
			// log.Printf("✅ Batch sent to flush channel (new backlog: %d/%d)", len(b.flushCh), channelBufferSize)
		default:
			b.mu.Lock()
			b.totalRecordsDropped += int64(len(batchToFlush))
			b.mu.Unlock()

			// Send critical alert for dropped records
			alertErr := fmt.Errorf("OHLCV records DROPPED: %d records lost due to channel overflow (channel: %d/%d)", len(batchToFlush), channelBacklog, channelBufferSize)
			sendCriticalAlert(alertErr, "OHLCVBuffer.addBar")

			log.Printf("🚨 CRITICAL: Flush channel full, dropping batch of %d records (channel: %d/%d)", len(batchToFlush), channelBacklog, channelBufferSize)
		}
	} else {
		b.mu.Unlock()
	}
}

func (b *OHLCVBuffer) createStagingTables() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := b.dbConn.DB.Acquire(ctx)
	if err != nil {
		log.Printf("Create staging tables - acquire conn error: %v", err)
		return fmt.Errorf("acquire conn for staging tables: %w", err)
	}
	defer conn.Release()

	// Create staging tables for all workers (single writer mode = 1 worker)
	numWorkers := 1
	for workerID := 0; workerID < numWorkers; workerID++ {
		createStage1m := fmt.Sprintf(`CREATE UNLOGGED TABLE IF NOT EXISTS public.ohlcv_1m_stage_w%d (
			ticker        text         NOT NULL,
			volume        bigint,
			open          bigint,
			close         bigint,
			high          bigint,
			low           bigint,
			"timestamp"   timestamptz  NOT NULL,
			transactions  integer
		) WITH (autovacuum_enabled = false);`, workerID)

		createStage1d := fmt.Sprintf(`CREATE UNLOGGED TABLE IF NOT EXISTS public.ohlcv_1d_stage_w%d (
			ticker        text         NOT NULL,
			volume        bigint,
			open          bigint,
			close         bigint,
			high          bigint,
			low           bigint,
			"timestamp"   timestamptz  NOT NULL,
			transactions  integer
		) WITH (autovacuum_enabled = false);`, workerID)

		_, err = conn.Exec(ctx, createStage1m)
		if err != nil {
			return fmt.Errorf("create ohlcv_1m_stage_w%d: %w", workerID, err)
		}

		_, err = conn.Exec(ctx, createStage1d)
		if err != nil {
			return fmt.Errorf("create ohlcv_1d_stage_w%d: %w", workerID, err)
		}
	}

	// log.Printf("✅ Worker-specific staging tables created (ohlcv_1m_stage_w0-w%d & ohlcv_1d_stage_w0-w%d)", numWorkers-1, numWorkers-1)
	return nil
}

func (b *OHLCVBuffer) writer(workerID int) {
	// log.Printf("🔄 OHLCV writer goroutine #%d started", workerID)
	defer b.wg.Done()

	// Store the worker ID for this writer instance
	b.mu.Lock()
	b.writerID = workerID
	b.mu.Unlock()

	for batch := range b.flushCh {
		startTime := time.Now()
		// channelBacklog := len(b.flushCh)
		// log.Printf("📝 Writer #%d processing batch of %d records (remaining backlog: %d/%d)",
		//	workerID, len(batch), channelBacklog, channelBufferSize)

		// Single worker processes batches sequentially to avoid TimescaleDB chunk lock conflicts
		// and staging table contention that was causing timeouts with multiple concurrent writers

		// Database operation
		dbStart := time.Now()
		b.doCopyMerge(batch, workerID)
		dbTime := time.Since(dbStart)

		processingTime := time.Since(startTime)
		b.mu.Lock()
		b.totalBatchesProcessed++
		b.mu.Unlock()

		// log.Printf("✅ Writer #%d completed batch: db=%v, total=%v (processed %d batches total, backlog now: %d/%d)",
		//	workerID, dbTime, processingTime, b.totalBatchesProcessed, len(b.flushCh), channelBufferSize)

		if processingTime > 5*time.Second {
			log.Printf("⚠️ WARNING: Writer #%d - Slow batch processing detected (total=%v, db=%v) - database may be bottleneck",
				workerID, processingTime, dbTime)
		}
		if dbTime > 4*time.Second {
			log.Printf("⚠️ WARNING: Writer #%d - Slow database operation detected (%v)", workerID, dbTime)
		}
	}
	// log.Printf("⚠️ OHLCV writer goroutine #%d exiting", workerID)
}

func (b *OHLCVBuffer) doCopyMerge(records []OHLCVRecord, workerID int) {
	operationStart := time.Now()

	// Pre-allocate slices with estimated capacity to avoid reallocations
	m1Rows := make([][]interface{}, 0, len(records))
	d1Rows := make([][]interface{}, 0, len(records))

	for _, record := range records {
		sec := record.Timestamp / 1000
		nsec := (record.Timestamp % 1000) * 1_000_000
		ts := time.Unix(sec, nsec)
		o := int64(record.Open * 1000)
		h := int64(record.High * 1000)
		l := int64(record.Low * 1000)
		c := int64(record.Close * 1000)
		v := record.Volume
		t := record.Ticker

		m1Rows = append(m1Rows, []interface{}{t, v, o, c, h, l, ts, 0})

		if utils.IsTimestampRegularHours(ts) {
			d1Rows = append(d1Rows, []interface{}{t, v, o, c, h, l, ts, 0})
		}
	}
	dataProcessingTime := time.Since(operationStart)
	// log.Printf("📊 DB Operation - Data processing: %v (1m_rows=%d, 1d_rows=%d)", dataProcessingTime, len(m1Rows), len(d1Rows))

	// Generate worker-specific table names and queries
	stage1mTable := fmt.Sprintf("ohlcv_1m_stage_w%d", workerID)
	stage1dTable := fmt.Sprintf("ohlcv_1d_stage_w%d", workerID)

	mergeQuery1m := fmt.Sprintf(`
INSERT INTO ohlcv_1m (ticker, volume, open, close, high, low, "timestamp")
SELECT ticker, SUM(volume) AS volume, first_open, last_close, MAX(high) AS high, MIN(low) AS low, minute AS "timestamp"
FROM (
  SELECT ticker, volume, open, close, high, low, "timestamp",
    FIRST_VALUE(open) OVER (PARTITION BY ticker, date_trunc('minute', "timestamp") ORDER BY "timestamp" ASC) AS first_open,
    FIRST_VALUE(close) OVER (PARTITION BY ticker, date_trunc('minute', "timestamp") ORDER BY "timestamp" DESC) AS last_close,
    date_trunc('minute', "timestamp") AS minute
  FROM %s
) sub
GROUP BY ticker, minute, first_open, last_close
ON CONFLICT (ticker, "timestamp") DO UPDATE SET
  high = GREATEST(ohlcv_1m.high, EXCLUDED.high),
  low = LEAST(ohlcv_1m.low, EXCLUDED.low),
  close = EXCLUDED.close,
  volume = ohlcv_1m.volume + EXCLUDED.volume,
  open = COALESCE(ohlcv_1m.open, EXCLUDED.open);`, stage1mTable)

	mergeQuery1d := fmt.Sprintf(`
INSERT INTO ohlcv_1d (ticker, volume, open, close, high, low, "timestamp")
SELECT ticker, SUM(volume) AS volume, first_open, last_close, MAX(high) AS high, MIN(low) AS low, day AS "timestamp"
FROM (
  SELECT ticker, volume, open, close, high, low, "timestamp",
    FIRST_VALUE(open) OVER (PARTITION BY ticker, date_trunc('day', "timestamp") ORDER BY "timestamp" ASC) AS first_open,
    FIRST_VALUE(close) OVER (PARTITION BY ticker, date_trunc('day', "timestamp") ORDER BY "timestamp" DESC) AS last_close,
    date_trunc('day', "timestamp") AS day
  FROM %s
) sub
GROUP BY ticker, day, first_open, last_close
ON CONFLICT (ticker, "timestamp") DO UPDATE SET
  high = GREATEST(ohlcv_1d.high, EXCLUDED.high),
  low = LEAST(ohlcv_1d.low, EXCLUDED.low),
  close = EXCLUDED.close,
  volume = ohlcv_1d.volume + EXCLUDED.volume,
  open = COALESCE(ohlcv_1d.open, EXCLUDED.open);`, stage1dTable)

	// Reduced timeout from 60s to 30s - operations shouldn't take this long
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Runtime verification: Check if staging tables still exist before proceeding
	/*
		if err := b.verifyStagingTablesExist(); err != nil {
			log.Printf("⚠️ Runtime check: Staging tables missing before COPY operation: %v", err)
			log.Printf("🔧 Attempting to recreate staging tables during runtime...")

			if createErr := b.ensureStagingTablesExist(); createErr != nil {
				log.Printf("❌ Failed to recreate staging tables during runtime: %v", createErr)
				return
			}
			log.Printf("✅ Staging tables recreated successfully during runtime")
		}*/

	connStart := time.Now()
	conn, err := b.dbConn.DB.Acquire(ctx)
	if err != nil {
		log.Printf("❌ DB Operation - Connection acquisition failed after %v: %v", time.Since(connStart), err)
		return
	}
	defer conn.Release()
	connTime := time.Since(connStart)

	// Log connection pool stats to help identify if pool exhaustion is causing delays
	// poolStats := b.dbConn.DB.Stat()
	// log.Printf("🔗 DB Operation - Connection acquired: %v (pool: total=%d, idle=%d, used=%d, constructing=%d, empty_acquire=%d)",
	//	connTime, poolStats.TotalConns(), poolStats.IdleConns(), poolStats.AcquiredConns(), poolStats.ConstructingConns(), poolStats.EmptyAcquireCount())

	txStart := time.Now()
	tx, err := conn.Begin(ctx)
	if err != nil {
		log.Printf("❌ DB Operation - Transaction begin failed after %v: %v", time.Since(txStart), err)
		return
	}
	txTime := time.Since(txStart)
	log.Printf("🔄 DB Operation - Transaction started: %v", txTime)

	var txCommitted bool
	defer func() {
		if !txCommitted {
			rollbackStart := time.Now()
			_ = tx.Rollback(ctx)
			log.Printf("↩️ DB Operation - Transaction rollback: %v", time.Since(rollbackStart))
		}
	}()

	// Batch all performance settings in one multi-statement query to reduce round trips
	settingsStart := time.Now()
	_, err = tx.Exec(ctx, `
		SET synchronous_commit = off;
		SET timescaledb.parallel_copy = on;
	`)
	if err != nil {
		log.Printf("❌ DB Operation - Performance settings failed after %v: %v", time.Since(settingsStart), err)
		return
	}
	settingsTime := time.Since(settingsStart)
	log.Printf("⚙️ DB Operation - Performance settings applied: %v", settingsTime)

	columns := []string{"ticker", "volume", "open", "close", "high", "low", "timestamp", "transactions"}

	if len(m1Rows) > 0 {
		// 1-minute operations
		copyStart := time.Now()
		_, err = tx.CopyFrom(ctx, pgx.Identifier{"public", stage1mTable}, columns, pgx.CopyFromRows(m1Rows))
		if err != nil {
			log.Printf("❌ DB Operation - COPY 1m failed after %v: %v", time.Since(copyStart), err)
			return
		}
		copyTime := time.Since(copyStart)
		log.Printf("📥 DB Operation - COPY 1m completed: %v (%d rows)", copyTime, len(m1Rows))

		mergeStart := time.Now()
		_, err = tx.Exec(ctx, mergeQuery1m)
		if err != nil {
			log.Printf("❌ DB Operation - MERGE 1m failed after %v: %v", time.Since(mergeStart), err)
			return
		}
		mergeTime := time.Since(mergeStart)
		log.Printf("🔀 DB Operation - MERGE 1m completed: %v", mergeTime)

		truncateStart := time.Now()
		_, err = tx.Exec(ctx, fmt.Sprintf("TRUNCATE %s;", stage1mTable))
		if err != nil {
			log.Printf("❌ DB Operation - TRUNCATE 1m failed after %v: %v", time.Since(truncateStart), err)
			return
		}
		truncateTime := time.Since(truncateStart)
		log.Printf("🗑️ DB Operation - TRUNCATE 1m completed: %v", truncateTime)
	}

	if len(d1Rows) > 0 {
		// 1-day operations
		copyStart := time.Now()
		_, err = tx.CopyFrom(ctx, pgx.Identifier{"public", stage1dTable}, columns, pgx.CopyFromRows(d1Rows))
		if err != nil {
			// Enhanced error handling - if COPY fails, don't attempt verification within aborted transaction
			log.Printf("❌ DB Operation - COPY 1d failed after %v: %v", time.Since(copyStart), err)
			log.Printf("⚠️ Transaction will be rolled back due to COPY failure")
			// The verification query would fail with 25P02 in an aborted transaction, so we skip it
			// Instead, we'll rely on the runtime check at the beginning of the next batch
			return
		}
		copyTime := time.Since(copyStart)
		log.Printf("📥 DB Operation - COPY 1d completed: %v (%d rows)", copyTime, len(d1Rows))

		mergeStart := time.Now()
		_, err = tx.Exec(ctx, mergeQuery1d)
		if err != nil {
			log.Printf("❌ DB Operation - MERGE 1d failed after %v: %v", time.Since(mergeStart), err)
			return
		}
		mergeTime := time.Since(mergeStart)
		log.Printf("🔀 DB Operation - MERGE 1d completed: %v", mergeTime)

		truncateStart := time.Now()
		_, err = tx.Exec(ctx, fmt.Sprintf("TRUNCATE %s;", stage1dTable))
		if err != nil {
			log.Printf("❌ DB Operation - TRUNCATE 1d failed after %v: %v", time.Since(truncateStart), err)
			return
		}
		truncateTime := time.Since(truncateStart)
		log.Printf("🗑️ DB Operation - TRUNCATE 1d completed: %v", truncateTime)
	}

	commitStart := time.Now()
	err = tx.Commit(ctx)
	if err != nil {
		log.Printf("❌ DB Operation - COMMIT failed after %v: %v", time.Since(commitStart), err)
	} else {
		txCommitted = true
		//commitTime := time.Since(commitStart)
		totalTime := time.Since(operationStart)
		//log.Printf("✅ DB Operation - COMMIT completed: %v (total_operation_time=%v)", commitTime, totalTime)

		// Performance analysis
		if totalTime > 5*time.Second {
			log.Printf("🐌 DB Operation - Performance Analysis (total=%v):", totalTime)
			log.Printf("   📊 Data processing: %v (%.1f%%)", dataProcessingTime, float64(dataProcessingTime)/float64(totalTime)*100)
			log.Printf("   🔗 Connection: %v (%.1f%%)", connTime, float64(connTime)/float64(totalTime)*100)
			log.Printf("   🔄 Transaction start: %v (%.1f%%)", txTime, float64(txTime)/float64(totalTime)*100)
			log.Printf("   ⚙️ Settings: %v (%.1f%%)", settingsTime, float64(settingsTime)/float64(totalTime)*100)
		}
	}
}

func (b *OHLCVBuffer) FlushRemaining() {
	b.mu.Lock()
	if b.stopped {
		b.mu.Unlock()
		return
	}

	if len(b.buffer) > 0 {
		channelBacklog := len(b.flushCh)
		log.Printf("🔄 Shutdown flush: %d remaining records (channel backlog: %d/%d)", len(b.buffer), channelBacklog, channelBufferSize)
		batchToFlush := make([]OHLCVRecord, len(b.buffer))
		copy(batchToFlush, b.buffer)
		b.buffer = b.buffer[:0]
		b.mu.Unlock()
		select {
		case b.flushCh <- batchToFlush:
			//log.Printf("✅ Shutdown batch sent to flush channel (final backlog: %d/%d)", len(b.flushCh), channelBufferSize)
		default:
			b.mu.Lock()
			b.totalRecordsDropped += int64(len(batchToFlush))
			b.mu.Unlock()

			// Send critical alert for dropped records during shutdown
			alertErr := fmt.Errorf("OHLCV records DROPPED during shutdown: %d records lost due to channel overflow (channel: %d/%d)", len(batchToFlush), channelBacklog, channelBufferSize)
			sendCriticalAlert(alertErr, "OHLCVBuffer.FlushRemaining")

			log.Printf("🚨 CRITICAL: Shutdown flush channel full, dropping batch of %d records (channel: %d/%d)", len(batchToFlush), channelBacklog, channelBufferSize)
		}
	} else {
		b.mu.Unlock()
	}
}

func (b *OHLCVBuffer) Stop() {
	b.mu.Lock()
	if b.stopped {
		b.mu.Unlock()
		return
	}
	b.stopped = true
	b.mu.Unlock()

	select {
	case <-b.stopTimeout:
		return
	default:
		close(b.stopTimeout)
	}
	b.FlushRemaining()
	close(b.flushCh)
	b.wg.Wait()
}

/*import "testing"

func TestOHLCVBufferNoDeadlock(t *testing.T) {
	// This test simulates concurrent bar additions to verify no deadlocks occur.
	// For a complete test, set up a test database connection and verify data insertion.
	// Here, we focus on concurrent production without crashing.

	// Assume conn is initialized with a test DB pool
	// conn := &data.Conn{DB: testPool} // Replace with actual mock or test conn

	InitOHLCVBuffer(nil, true) // Using nil conn for simulation; in real test, use valid conn

	var producerWg sync.WaitGroup
	const numGoroutines = 20
	const numBarsPerGoroutine = 1000

	for i := 0; i < numGoroutines; i++ {
		producerWg.Add(1)
		go func() {
			defer producerWg.Done()
			for j := 0; j < numBarsPerGoroutine; j++ {
				ts := time.Now().UnixMilli()
				bar := models.EquityAgg{
					Open:   100.0 + float64(j),
					High:   101.0 + float64(j),
					Low:    99.0 + float64(j),
					Close:  100.5 + float64(j),
					Volume: 1000 + int64(j),
				}
				ohlcvBuffer.addBar(ts, "TEST", bar)
				// Small sleep to simulate burst
				time.Sleep(100 * time.Microsecond)
			}
		}()
	}

	producerWg.Wait()

	// Allow time for flushes
	time.Sleep(2 * time.Second)

	ohlcvBuffer.Stop()

	// If execution reaches here without errors, the test passes (no deadlocks observed)
	t.Log("Concurrent bar addition completed without deadlocks")
}
*/
