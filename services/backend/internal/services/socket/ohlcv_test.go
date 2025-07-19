package socket

import (
	"sync"
	"testing"
	"time"

	"github.com/stock-screener/backend/internal/models"
)

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
