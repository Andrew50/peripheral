package socket

import (
	"backend/internal/data"
	"context"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/polygon-io/client-go/websocket/models"
)

type OHLCVRecord struct {
	Timestamp  int64
	SecurityID int
	Open       float64
	High       float64
	Low        float64
	Close      float64
	Volume     int64
}

type OHLCVBuffer struct {
	currentTimestamp int64
	buffer           []OHLCVRecord
	dbConn           *data.Conn
	mu               sync.Mutex
	lastFlush        time.Time
	stopTimeout      chan struct{}
}

var ohlcvBuffer *OHLCVBuffer

// Initialize the OHLCV buffer
func InitOHLCVBuffer(conn *data.Conn) {
	ohlcvBuffer = &OHLCVBuffer{
		buffer:      make([]OHLCVRecord, 0, 5000), // Pre-allocate for performance
		dbConn:      conn,
		lastFlush:   time.Now(),
		stopTimeout: make(chan struct{}),
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
		go b.batchInsertToSeconds(batchToFlush)
		b.lastFlush = time.Now()
	}
}

// Add a new OHLCV bar to the buffer
func (b *OHLCVBuffer) addBar(timestamp int64, securityID int, bar models.EquityAgg) {
	record := OHLCVRecord{
		Timestamp:  timestamp,
		SecurityID: securityID,
		Open:       bar.Open,
		High:       bar.High,
		Low:        bar.Low,
		Close:      bar.Close,
		Volume:     int64(bar.Volume),
	}

	b.mu.Lock()

	var batchToFlush []OHLCVRecord
	var oldTimestamp int64
	var shouldFlush bool

	if timestamp > b.currentTimestamp {
		if len(b.buffer) > 0 {
			batchToFlush = make([]OHLCVRecord, len(b.buffer))
			copy(batchToFlush, b.buffer)
			oldTimestamp = b.currentTimestamp
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
		log.Printf("Flushing batch: %d records for timestamp %d",
			len(batchToFlush), oldTimestamp)
	}
}

func (b *OHLCVBuffer) batchInsert(records []OHLCVRecord) {
	b.batchInsertToSeconds(records)
	b.batchInsertToMinutes(records)
}

// Batch insert multiple records for the same timestamp
func (b *OHLCVBuffer) batchInsertToSeconds(records []OHLCVRecord) {
	batch := &pgx.Batch{}

	for _, record := range records {
		batch.Queue(`
            INSERT INTO ohlcv_1s (timestamp, securityid, open, high, low, close, volume) 
            VALUES ($1, $2, $3, $4, $5, $6, $7)
            ON CONFLICT (securityid, timestamp) DO UPDATE SET
                open = EXCLUDED.open, high = EXCLUDED.high, 
                low = EXCLUDED.low, close = EXCLUDED.close, volume = EXCLUDED.volume`,
			time.Unix(record.Timestamp/1000, 0),
			record.SecurityID,
			record.Open, record.High, record.Low, record.Close, record.Volume,
		)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	br := b.dbConn.DB.SendBatch(ctx, batch)
	defer br.Close()

	// Execute all statements
	for i := 0; i < len(records); i++ {
		_, err := br.Exec()
		if err != nil {
			log.Printf("Batch exec error on record %d: %v", i, err)
		}
	}
}

func (b *OHLCVBuffer) batchInsertToMinutes(records []OHLCVRecord) {
	batch := &pgx.Batch{}

	for _, record := range records {
		minuteTimestamp := time.Unix(record.Timestamp/1000, 0).Truncate(time.Minute)

		batch.Queue(`
            INSERT INTO ohlcv_1m (timestamp, securityid, open, high, low, close, volume)
            VALUES ($1, $2, $3, $4, $5, $6, $7)
            ON CONFLICT (securityid, timestamp) DO UPDATE SET
                high = GREATEST(ohlcv_1m.high, EXCLUDED.high),
                low = LEAST(ohlcv_1m.low, EXCLUDED.low),
                close = EXCLUDED.close,
                volume = ohlcv_1m.volume + EXCLUDED.volume`,
			minuteTimestamp,
			record.SecurityID,
			record.Open, record.High, record.Low, record.Close, record.Volume,
		)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	br := b.dbConn.DB.SendBatch(ctx, batch)
	defer br.Close()

	// Execute all statements
	for i := 0; i < len(records); i++ {
		_, err := br.Exec()
		if err != nil {
			log.Printf("Batch exec error on record %d: %v", i, err)
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
