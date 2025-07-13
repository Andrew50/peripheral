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
	Close     float64
	Volume    int64
}

type OHLCVBuffer struct {
	currentTimestamp int64
	buffer           []OHLCVRecord
	dbConn           *data.Conn
	mu               sync.Mutex
	lastFlush        time.Time
	stopTimeout      chan struct{}
	enableRealtime   bool
}

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
	ticker := time.NewTicker(5 * time.Second)
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

	if len(b.buffer) > 0 && time.Since(b.lastFlush) > 10*time.Second {
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

	var batchToFlush []OHLCVRecord
	//var oldTimestamp int64
	var shouldFlush bool

	if timestamp > b.currentTimestamp {
		if len(b.buffer) > 0 {
			batchToFlush = make([]OHLCVRecord, len(b.buffer))
			copy(batchToFlush, b.buffer)
			//oldTimestamp = b.currentTimestamp
			shouldFlush = true
			b.lastFlush = time.Now()
		}

		b.buffer = b.buffer[:0]
		b.currentTimestamp = timestamp
	}

	b.buffer = append(b.buffer, record)

	b.mu.Unlock()

	if shouldFlush {
		go b.batchInsert(batchToFlush)
		//log.Printf("Flushing batch: %d records for timestamp %d",
		//len(batchToFlush), oldTimestamp)
	}
}

func (b *OHLCVBuffer) batchInsert(records []OHLCVRecord) {
	batch := &pgx.Batch{}
	statementsQueued := 0

	for _, record := range records {
		minuteTimestamp := time.Unix(record.Timestamp/1000, 0).Truncate(time.Minute)
		dayTimestamp := minuteTimestamp.Truncate(24 * time.Hour)

		// 1m upsert
		batch.Queue(`
            INSERT INTO ohlcv_1m (ticker, volume, open, close, high, low, "timestamp", transactions)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
            ON CONFLICT (ticker, "timestamp") DO UPDATE SET
                high = GREATEST(ohlcv_1m.high, EXCLUDED.high),
                low = LEAST(ohlcv_1m.low, EXCLUDED.low),
                close = EXCLUDED.close,
                volume = ohlcv_1m.volume + EXCLUDED.volume`,
			record.Ticker,
			record.Volume,
			record.Open,
			record.Close,
			record.High,
			record.Low,
			minuteTimestamp,
			nil, // transactions - not available from real-time data
		)
		statementsQueued++

		// 1d upsert only during regular hours
		if utils.IsTimestampRegularHours(minuteTimestamp) {
			batch.Queue(`
            INSERT INTO ohlcv_1d (ticker, volume, open, close, high, low, "timestamp", transactions)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
            ON CONFLICT (ticker, "timestamp") DO UPDATE SET
                high = GREATEST(ohlcv_1d.high, EXCLUDED.high),
                low = LEAST(ohlcv_1d.low, EXCLUDED.low),
                close = EXCLUDED.close,
                volume = ohlcv_1d.volume + EXCLUDED.volume,
                open = COALESCE(ohlcv_1d.open, EXCLUDED.open)`,
				record.Ticker,
				record.Volume,
				record.Open,
				record.Close,
				record.High,
				record.Low,
				dayTimestamp,
				nil, // transactions - not available from real-time data
			)
			statementsQueued++
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	br := b.dbConn.DB.SendBatch(ctx, batch)
	defer br.Close()

	// Execute all queued statements
	for i := 0; i < statementsQueued; i++ {
		_, err := br.Exec()
		if err != nil {
			log.Printf("Batch exec error on statement %d: %v", i, err)
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
