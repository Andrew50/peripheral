package socket

import (
	"backend/internal/data"
	"backend/internal/data/utils"
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/polygon-io/client-go/websocket/models"
)

type OHLCVRecord struct {
	Timestamp int64
	Ticker    string
	Open      float64
	High      float64
	Low       float64

	Close  float64
	Volume int64
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
}

const flushThreshold = 200000
const flushTimeout = 1 * time.Second
const checkInterval = 1 * time.Second

const mergeQuery1m = `
INSERT INTO ohlcv_1m (ticker, volume, open, close, high, low, "timestamp")
SELECT ticker, SUM(volume) AS volume, first_open, last_close, MAX(high) AS high, MIN(low) AS low, minute AS "timestamp"
FROM (
  SELECT ticker, volume, open, close, high, low, "timestamp",
    FIRST_VALUE(open) OVER (PARTITION BY ticker, date_trunc('minute', "timestamp") ORDER BY "timestamp" ASC) AS first_open,
    FIRST_VALUE(close) OVER (PARTITION BY ticker, date_trunc('minute', "timestamp") ORDER BY "timestamp" DESC) AS last_close,
    date_trunc('minute', "timestamp") AS minute
  FROM ohlcv_1m_stage
) sub
GROUP BY ticker, minute, first_open, last_close
ON CONFLICT (ticker, "timestamp") DO UPDATE SET
  high = GREATEST(ohlcv_1m.high, EXCLUDED.high),
  low = LEAST(ohlcv_1m.low, EXCLUDED.low),
  close = EXCLUDED.close,
  volume = ohlcv_1m.volume + EXCLUDED.volume,
  open = COALESCE(ohlcv_1m.open, EXCLUDED.open);`

const mergeQuery1d = `
INSERT INTO ohlcv_1d (ticker, volume, open, close, high, low, "timestamp")
SELECT ticker, SUM(volume) AS volume, first_open, last_close, MAX(high) AS high, MIN(low) AS low, day AS "timestamp"
FROM (
  SELECT ticker, volume, open, close, high, low, "timestamp",
    FIRST_VALUE(open) OVER (PARTITION BY ticker, date_trunc('day', "timestamp") ORDER BY "timestamp" ASC) AS first_open,
    FIRST_VALUE(close) OVER (PARTITION BY ticker, date_trunc('day', "timestamp") ORDER BY "timestamp" DESC) AS last_close,
    date_trunc('day', "timestamp") AS day
  FROM ohlcv_1d_stage
) sub
GROUP BY ticker, day, first_open, last_close
ON CONFLICT (ticker, "timestamp") DO UPDATE SET
  high = GREATEST(ohlcv_1d.high, EXCLUDED.high),
  low = LEAST(ohlcv_1d.low, EXCLUDED.low),
  close = EXCLUDED.close,
  volume = ohlcv_1d.volume + EXCLUDED.volume,
  open = COALESCE(ohlcv_1d.open, EXCLUDED.open);`

var ohlcvBuffer *OHLCVBuffer

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
		flushCh:     make(chan []OHLCVRecord, 50),
		stopped:     false,
	}

	// Create staging tables immediately on initialization
	if err := ohlcvBuffer.createStagingTables(); err != nil {
		log.Printf("❌ Failed to create staging tables: %v", err)
		return err
	}

	log.Printf("🔄 Starting OHLCV buffer writer goroutine...")
	ohlcvBuffer.wg.Add(1)
	go ohlcvBuffer.writer()

	log.Printf("⏰ Starting OHLCV buffer timeout flusher...")
	ohlcvBuffer.startTimeoutFlusher()
	log.Printf("✅ OHLCV buffer initialized successfully")
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

func (b *OHLCVBuffer) flushIfStale() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.stopped {
		return
	}

	if len(b.buffer) > 0 && time.Since(b.lastFlush) > flushTimeout {
		batchToFlush := make([]OHLCVRecord, len(b.buffer))
		copy(batchToFlush, b.buffer)
		b.buffer = b.buffer[:0]
		b.lastFlush = time.Now()
		select {
		case b.flushCh <- batchToFlush:
		default:
			log.Printf("Warning: flush channel full, dropping batch of %d records", len(batchToFlush))
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
	//if ticker == "COIN" || ticker == "AAPL" {
	//log.Printf("🪙 %s bar: ts=%d, O=%.4f, H=%.4f, L=%.4f, C=%.4f, V=%d",
	//ticker, timestamp, bar.Open, bar.High, bar.Low, bar.Close, int64(bar.Volume))
	//}

	b.mu.Lock()
	if b.stopped {
		log.Printf("⚠️ addBar skipping %s - buffer stopped", ticker)
		b.mu.Unlock()
		return
	}

	b.buffer = append(b.buffer, record)
	//log.Printf("📈 Added %s to buffer (buffer size: %d/%d)", ticker, len(b.buffer), flushThreshold)

	if len(b.buffer) >= flushThreshold {
		log.Printf("🚨 Buffer threshold reached, flushing %d records", len(b.buffer))
		batchToFlush := make([]OHLCVRecord, len(b.buffer))
		copy(batchToFlush, b.buffer)
		b.buffer = b.buffer[:0]
		b.lastFlush = time.Now()
		b.mu.Unlock()
		select {
		case b.flushCh <- batchToFlush:
			log.Printf("✅ Batch sent to flush channel")
		default:
			log.Printf("Warning: flush channel full, dropping batch of %d records", len(batchToFlush))
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

	createStage1m := `CREATE UNLOGGED TABLE IF NOT EXISTS public.ohlcv_1m_stage (
		ticker        text         NOT NULL,
		volume        bigint,
		open          bigint,
		close         bigint,
		high          bigint,
		low           bigint,
		"timestamp"   timestamptz  NOT NULL,
		transactions  integer
	) WITH (autovacuum_enabled = false);`

	createStage1d := `CREATE UNLOGGED TABLE IF NOT EXISTS public.ohlcv_1d_stage (
		ticker        text         NOT NULL,
		volume        bigint,
		open          bigint,
		close         bigint,
		high          bigint,
		low           bigint,
		"timestamp"   timestamptz  NOT NULL,
		transactions  integer
	) WITH (autovacuum_enabled = false);`

	_, err = conn.Exec(ctx, createStage1m)
	if err != nil {
		return fmt.Errorf("create ohlcv_1m_stage: %w", err)
	}

	_, err = conn.Exec(ctx, createStage1d)
	if err != nil {
		return fmt.Errorf("create ohlcv_1d_stage: %w", err)
	}

	// -------------------------------------------------------------------
	// Verify that the staging tables are actually visible from **this** and
	// future connections.  This rules out search_path problems or silent
	// roll-backs.
	// -------------------------------------------------------------------
	var check1m, check1d *string
	if err := conn.QueryRow(ctx, "SELECT to_regclass('public.ohlcv_1m_stage')").Scan(&check1m); err != nil {
		return fmt.Errorf("verify ohlcv_1m_stage: %w", err)
	}
	if check1m == nil {
		return fmt.Errorf("verify ohlcv_1m_stage: table not found after creation")
	}
	if err := conn.QueryRow(ctx, "SELECT to_regclass('public.ohlcv_1d_stage')").Scan(&check1d); err != nil {
		return fmt.Errorf("verify ohlcv_1d_stage: %w", err)
	}
	if check1d == nil {
		return fmt.Errorf("verify ohlcv_1d_stage: table not found after creation")
	}

	log.Printf("✅ Verified staging tables exist (ohlcv_1m_stage & ohlcv_1d_stage)")

	return nil
}

func (b *OHLCVBuffer) writer() {
	log.Printf("🔄 OHLCV writer goroutine started")
	defer b.wg.Done()

	for batch := range b.flushCh {
		//log.Printf("📝 Writer processing batch of %d records", len(batch))
		sort.Slice(batch, func(i, j int) bool {
			if batch[i].Ticker != batch[j].Ticker {
				return batch[i].Ticker < batch[j].Ticker
			}
			return batch[i].Timestamp < batch[j].Timestamp
		})
		b.doCopyMerge(batch)
		//log.Printf("✅ Writer completed batch processing")
	}
	log.Printf("⚠️ OHLCV writer goroutine exiting")
}

func (b *OHLCVBuffer) doCopyMerge(records []OHLCVRecord) {
	var m1Rows [][]interface{}
	var d1Rows [][]interface{}

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

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, err := b.dbConn.DB.Acquire(ctx)
	if err != nil {
		log.Printf("Acquire conn error: %v", err)
		return
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		log.Printf("Begin txn error: %v", err)
		return
	}

	var txCommitted bool
	defer func() {
		if !txCommitted {
			_ = tx.Rollback(ctx)
		}
	}()

	_, err = tx.Exec(ctx, "SET synchronous_commit = off;")
	if err != nil {
		log.Printf("SET synchronous_commit error: %v", err)
		return
	}

	_, err = tx.Exec(ctx, "SET timescaledb.parallel_copy = on;")
	if err != nil {
		log.Printf("SET parallel_copy error: %v", err)
		return
	}

	columns := []string{"ticker", "volume", "open", "close", "high", "low", "timestamp", "transactions"}

	if len(m1Rows) > 0 {
		_, err = tx.CopyFrom(ctx, pgx.Identifier{"public", "ohlcv_1m_stage"}, columns, pgx.CopyFromRows(m1Rows))
		if err != nil {
			log.Printf("CopyFrom 1m error: %v", err)
			return
		}
		_, err = tx.Exec(ctx, mergeQuery1m)
		if err != nil {
			log.Printf("Merge 1m error: %v", err)
			return
		}
		_, err = tx.Exec(ctx, "TRUNCATE ohlcv_1m_stage;")
		if err != nil {
			log.Printf("Truncate 1m error: %v", err)
			return
		}
	}

	if len(d1Rows) > 0 {
		_, err = tx.CopyFrom(ctx, pgx.Identifier{"public", "ohlcv_1d_stage"}, columns, pgx.CopyFromRows(d1Rows))
		if err != nil {
			// If COPY fails with relation not found, verify existence immediately
			log.Printf("CopyFrom 1d error: %v", err)
			var exists *string
			if e := tx.QueryRow(ctx, "SELECT to_regclass('public.ohlcv_1d_stage')").Scan(&exists); e != nil {
				log.Printf("⚠️  Verification query failed: %v", e)
			} else if exists == nil {
				log.Printf("⚠️  Verification: ohlcv_1d_stage is NOT present in current session after COPY failure")
			} else {
				log.Printf("ℹ️  Verification: ohlcv_1d_stage DOES exist despite COPY failure – possible search_path or privilege issue")
			}
			return
		}
		_, err = tx.Exec(ctx, mergeQuery1d)
		if err != nil {
			log.Printf("Merge 1d error: %v", err)
			return
		}
		_, err = tx.Exec(ctx, "TRUNCATE ohlcv_1d_stage;")
		if err != nil {
			log.Printf("Truncate 1d error: %v", err)
			return
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		log.Printf("Commit error: %v", err)
	} else {
		txCommitted = true
	}
}

func (b *OHLCVBuffer) FlushRemaining() {
	b.mu.Lock()
	if b.stopped {
		b.mu.Unlock()
		return
	}

	if len(b.buffer) > 0 {
		batchToFlush := make([]OHLCVRecord, len(b.buffer))
		copy(batchToFlush, b.buffer)
		b.buffer = b.buffer[:0]
		b.mu.Unlock()
		select {
		case b.flushCh <- batchToFlush:
		default:
			log.Printf("Warning: flush channel full during shutdown, dropping batch of %d records", len(batchToFlush))
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
