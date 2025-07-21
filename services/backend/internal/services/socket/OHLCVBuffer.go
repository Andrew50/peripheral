package socket

import (
	"backend/internal/data"
	"backend/internal/data/utils"
	"context"
	"log"
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
}

const flushThreshold = 50000
const flushTimeout = 30 * time.Second
const checkInterval = 10 * time.Second

const mergeQuery1m = `
INSERT INTO ohlcv_1m (ticker, volume, open, close, high, low, "timestamp")
SELECT ticker, volume, open, close, high, low, "timestamp" FROM ohlcv_1m_stage
ON CONFLICT (ticker, "timestamp") DO UPDATE SET
  high = GREATEST(ohlcv_1m.high, EXCLUDED.high),
  low = LEAST(ohlcv_1m.low, EXCLUDED.low),
  close = EXCLUDED.close,
  volume = ohlcv_1m.volume + EXCLUDED.volume;`

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
		buffer:         make([]OHLCVRecord, 0, 5000), // Pre-allocate for performance
		dbConn:         conn,
		lastFlush:      time.Now(),
		stopTimeout:    make(chan struct{}),
		enableRealtime: enableRealtime,
	}
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

	if len(b.buffer) > 0 && time.Since(b.lastFlush) > flushTimeout {
		batchToFlush := make([]OHLCVRecord, len(b.buffer))
		copy(batchToFlush, b.buffer)
		b.buffer = b.buffer[:0]
		go b.batchInsert(batchToFlush)
		b.lastFlush = time.Now()
	}
}

// Add a new OHLCV bar to the buffer
func (b *OHLCVBuffer) addBar(timestamp int64, ticker string, bar models.EquityAgg) {
	// Check if realtime insertion is enabled
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
	b.buffer = append(b.buffer, record)
	if len(b.buffer) >= flushThreshold {
		batchToFlush := make([]OHLCVRecord, len(b.buffer))
		copy(batchToFlush, b.buffer)
		b.buffer = b.buffer[:0]
		b.lastFlush = time.Now()
		b.mu.Unlock()
		go b.batchInsert(batchToFlush)
	} else {
		b.mu.Unlock()
	}
}

func (b *OHLCVBuffer) batchInsert(records []OHLCVRecord) {
	var m1Rows [][]interface{}
	var d1Rows [][]interface{}

	for _, record := range records {
		sec := record.Timestamp / 1000
		nsec := (record.Timestamp % 1000) * 1_000_000
		ts := time.Unix(sec, nsec).Truncate(time.Minute)
		o := int64(record.Open * 1000)
		c := int64(record.Close * 1000)
		h := int64(record.High * 1000)
		l := int64(record.Low * 1000)
		v := record.Volume
		t := record.Ticker

		m1Rows = append(m1Rows, []interface{}{t, v, o, c, h, l, ts})

		if utils.IsTimestampRegularHours(ts) {
			d1Rows = append(d1Rows, []interface{}{t, v, o, c, h, l, ts})
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := b.dbConn.DB.Acquire(ctx)
	if err != nil {
		log.Printf("Acquire conn error: %v", err)
		return
	}
	defer conn.Release()

	_, err = conn.Exec(ctx, "SET synchronous_commit = off;")
	if err != nil {
		log.Printf("SET synchronous_commit error: %v", err)
	}

	_, err = conn.Exec(ctx, "SET timescaledb.parallel_copy = on;")
	if err != nil {
		log.Printf("SET parallel_copy error: %v", err)
	}

	columns := []string{"ticker", "volume", "open", "close", "high", "low", "timestamp"}

	if len(m1Rows) > 0 {
		_, err := conn.Conn().CopyFrom(ctx, pgx.Identifier{"ohlcv_1m_stage"}, columns, pgx.CopyFromRows(m1Rows))
		if err != nil {
			log.Printf("CopyFrom 1m error: %v", err)
			return
		}
		_, err = conn.Exec(ctx, mergeQuery1m)
		if err != nil {
			log.Printf("Merge 1m error: %v", err)
		}
		_, err = conn.Exec(ctx, "TRUNCATE ohlcv_1m_stage;")
		if err != nil {
			log.Printf("Truncate 1m error: %v", err)
		}
	}

	if len(d1Rows) > 0 {
		_, err := conn.Conn().CopyFrom(ctx, pgx.Identifier{"ohlcv_1d_stage"}, columns, pgx.CopyFromRows(d1Rows))
		if err != nil {
			log.Printf("CopyFrom 1d error: %v", err)
			return
		}
		_, err = conn.Exec(ctx, mergeQuery1d)
		if err != nil {
			log.Printf("Merge 1d error: %v", err)
		}
		_, err = conn.Exec(ctx, "TRUNCATE ohlcv_1d_stage;")
		if err != nil {
			log.Printf("Truncate 1d error: %v", err)
		}
	}
}

func (b *OHLCVBuffer) FlushRemaining() {
	b.mu.Lock()
	if len(b.buffer) > 0 {
		batchToFlush := make([]OHLCVRecord, len(b.buffer))
		copy(batchToFlush, b.buffer)
		b.buffer = b.buffer[:0]
		b.mu.Unlock()

		// Give it 10 seconds max for shutdown
		done := make(chan struct{})
		go func() {
			b.batchInsert(batchToFlush)
			close(done)
		}()

		select {
		case <-done:
			log.Printf("✅ Graceful shutdown completed")
		case <-time.After(10 * time.Second):
			log.Printf("⚠️ Shutdown timeout - some data may be lost")
		}
	} else {
		b.mu.Unlock()
	}
}

func (b *OHLCVBuffer) Stop() {
	select {
	case <-b.stopTimeout:
		return
	default:
		close(b.stopTimeout)
	}
	b.FlushRemaining()
}
