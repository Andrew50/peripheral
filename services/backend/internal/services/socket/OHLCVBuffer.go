package socket

import (
	"backend/internal/data"
	"backend/internal/data/utils"
	"context"
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
	buffer         []OHLCVRecord
	dbConn         *data.Conn
	mu             sync.Mutex
	lastFlush      time.Time
	stopTimeout    chan struct{}
	enableRealtime bool
	flushCh        chan []OHLCVRecord
	wg             sync.WaitGroup
	stopped        bool
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
func InitOHLCVBuffer(conn *data.Conn, enableRealtime bool) {
	ohlcvBuffer = &OHLCVBuffer{
		buffer:         make([]OHLCVRecord, 0, flushThreshold),
		dbConn:         conn,
		lastFlush:      time.Now(),
		stopTimeout:    make(chan struct{}),
		enableRealtime: enableRealtime,
		flushCh:        make(chan []OHLCVRecord, 50),
		stopped:        false,
	}
	ohlcvBuffer.wg.Add(1)
	go ohlcvBuffer.writer()
	ohlcvBuffer.startTimeoutFlusher()
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
	if !b.enableRealtime {
		return
	}

	record := OHLCVRecord{
		Timestamp: timestamp,
		Ticker:    ticker,
		Open:      bar.Open,
		High:      bar.High,
		Low:       bar.Low,
		Close:     bar.Close,
		Volume:    int64(bar.Volume),
	}

	b.mu.Lock()
	if b.stopped {
		b.mu.Unlock()
		return
	}

	b.buffer = append(b.buffer, record)
	if len(b.buffer) >= flushThreshold {
		batchToFlush := make([]OHLCVRecord, len(b.buffer))
		copy(batchToFlush, b.buffer)
		b.buffer = b.buffer[:0]
		b.lastFlush = time.Now()
		b.mu.Unlock()
		select {
		case b.flushCh <- batchToFlush:
		default:
			log.Printf("Warning: flush channel full, dropping batch of %d records", len(batchToFlush))
		}
	} else {
		b.mu.Unlock()
	}
}

func (b *OHLCVBuffer) createStagingTables() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := b.dbConn.DB.Acquire(ctx)
	if err != nil {
		log.Printf("Create staging tables - acquire conn error: %v", err)
		return
	}
	defer conn.Release()

	createStage1m := `CREATE UNLOGGED TABLE IF NOT EXISTS ohlcv_1m_stage (
		ticker        text         NOT NULL,
		volume        bigint,
		open          bigint,
		close         bigint,
		high          bigint,
		low           bigint,
		"timestamp"   timestamptz  NOT NULL,
		transactions  integer
	) WITH (autovacuum_enabled = false);`

	createStage1d := `CREATE UNLOGGED TABLE IF NOT EXISTS ohlcv_1d_stage (
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
		log.Printf("Create ohlcv_1m_stage error: %v", err)
	}

	_, err = conn.Exec(ctx, createStage1d)
	if err != nil {
		log.Printf("Create ohlcv_1d_stage error: %v", err)
	}
}

func (b *OHLCVBuffer) writer() {
	defer b.wg.Done()
	// Create staging tables once when writer starts
	b.createStagingTables()

	for batch := range b.flushCh {
		sort.Slice(batch, func(i, j int) bool {
			if batch[i].Ticker != batch[j].Ticker {
				return batch[i].Ticker < batch[j].Ticker
			}
			return batch[i].Timestamp < batch[j].Timestamp
		})
		b.doCopyMerge(batch)
	}
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
		_, err = tx.CopyFrom(ctx, pgx.Identifier{"ohlcv_1m_stage"}, columns, pgx.CopyFromRows(m1Rows))
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
		_, err = tx.CopyFrom(ctx, pgx.Identifier{"ohlcv_1d_stage"}, columns, pgx.CopyFromRows(d1Rows))
		if err != nil {
			log.Printf("CopyFrom 1d error: %v", err)
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
