package alerts

import (
	"backend/internal/data"
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var userID int

func TestAlertSuite(t *testing.T) {
	conn, cleanup := data.InitTestConn(t)
	defer cleanup()

	ctx := context.Background()

	// Create test user first
	err := conn.DB.QueryRow(ctx,
		`INSERT INTO users (username, password, email)
		 VALUES ($1, $2, $3) RETURNING userId`,
		fmt.Sprintf("testuser_%d", time.Now().UnixNano()),
		"testpass",
		"test@example.com").Scan(&userID)
	require.NoError(t, err)
	t.Logf("Created test user with ID: %d", userID)

	// Create test securities
	testSecurities := createTestSecurities(t, conn, ctx)
	t.Logf("Created %d test securities", len(testSecurities))

	// Create test price snapshot
	priceSnapshot := createTestPriceSnapshot(testSecurities)

	// Run sub-tests
	t.Run("TestAddPriceAlert", func(t *testing.T) {
		testAddPriceAlert(t, conn, testSecurities)
	})

	t.Run("TestRemoveAlert", func(t *testing.T) {
		testRemoveAlert(t, conn, testSecurities)
	})

	t.Run("TestPriceAlertTriggering", func(t *testing.T) {
		testPriceAlertTriggering(t, conn, testSecurities, priceSnapshot)
	})

	t.Run("TestShardBoundaryRecalculation", func(t *testing.T) {
		testShardBoundaryRecalculation(t, conn, testSecurities)
	})

	t.Run("TestConcurrentAlertOperations", func(t *testing.T) {
		testConcurrentAlertOperations(t, conn, testSecurities)
	})

	t.Run("TestBatchCleanupAlerts", func(t *testing.T) {
		testBatchCleanupAlerts(t, conn, testSecurities)
	})
}

// TestSecurity represents our test security data
type TestSecurity struct {
	SecurityID int
	Ticker     string
	Name       string
	Sector     string
	Industry   string
	FIGI       string
}

func createTestSecurities(t *testing.T, conn *data.Conn, ctx context.Context) []TestSecurity {
	securities := []TestSecurity{
		{
			Ticker: "TESTAAPL",
			Name:   "Test Apple",
			FIGI:   "BBG000B9XRY4",
		},
		{
			Ticker: "TESTTSLA",
			Name:   "Test Tesla",
			FIGI:   "BBG000N9MNX3",
		},
		{
			Ticker: "TESTGOOG",
			Name:   "Test Alphabet",
			FIGI:   "BBG009S39JX6",
		},
		{
			Ticker: "TESTAMZN",
			Name:   "Test Amazon",
			FIGI:   "BBG000BVPV84",
		},
		{
			Ticker: "TESTMSFT",
			Name:   "Test Microsoft",
			FIGI:   "BBG000BPH459",
		},
	}

	for i := range securities {
		err := conn.DB.QueryRow(ctx,
			`INSERT INTO securities (
				ticker,
				name,
				figi,
				minDate,
				maxDate,
				active
			) VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING securityId`,
			securities[i].Ticker,
			securities[i].Name,
			securities[i].FIGI,
			time.Now().AddDate(-1, 0, 0),
			nil,
			true,
		).Scan(&securities[i].SecurityID)
		require.NoError(t, err)
		t.Logf("✅ Created security: %s (ID: %d, FIGI: %s)",
			securities[i].Ticker, securities[i].SecurityID, securities[i].FIGI)
	}

	return securities
}

// Create test price snapshot for testing
func createTestPriceSnapshot(securities []TestSecurity) map[int]float64 {
	return map[int]float64{
		securities[0].SecurityID: 150.0,  // TESTAAPL
		securities[1].SecurityID: 200.0,  // TESTTSLA
		securities[2].SecurityID: 2500.0, // TESTGOOG
		securities[3].SecurityID: 100.0,  // TESTAMZN
		securities[4].SecurityID: 300.0,  // TESTMSFT
	}
}

func testAddPriceAlert(t *testing.T, conn *data.Conn, securities []TestSecurity) {
	// Clear any existing price alert shards
	priceAlertsMutex.Lock()
	priceAlertShards = make(map[int]*PriceAlertShard)
	priceAlertsMutex.Unlock()

	security := securities[0]
	price := 160.0
	direction := true // above

	alert := Alert{
		AlertID:    1001,
		UserID:     userID,
		AlertType:  "price",
		SecurityID: &security.SecurityID,
		Price:      &price,
		Direction:  &direction,
	}

	AddAlert(conn, alert)

	// Verify the alert was added to the correct shard
	priceAlertsMutex.RLock()
	shard, exists := priceAlertShards[security.SecurityID]
	priceAlertsMutex.RUnlock()

	assert.True(t, exists, "Price alert shard should exist")
	assert.NotNil(t, shard, "Shard should not be nil")

	shard.Mutex.RLock()
	storedAlert, alertExists := shard.Alerts[alert.AlertID]
	shard.Mutex.RUnlock()

	assert.True(t, alertExists, "Alert should exist in shard")
	assert.Equal(t, alert.AlertID, storedAlert.AlertID)
	assert.Equal(t, price, shard.LowestAbove)
	assert.NotNil(t, storedAlert.Ticker, "Ticker should be set")

	t.Logf("✅ Successfully added price alert for %s at $%.2f", *storedAlert.Ticker, price)
}

func testRemoveAlert(t *testing.T, conn *data.Conn, securities []TestSecurity) {
	security := securities[1]
	price1, price2 := 180.0, 220.0
	direction := true

	// Add two alerts
	alert1 := Alert{
		AlertID:    1002,
		UserID:     userID,
		AlertType:  "price",
		SecurityID: &security.SecurityID,
		Price:      &price1,
		Direction:  &direction,
	}

	alert2 := Alert{
		AlertID:    1003,
		UserID:     userID,
		AlertType:  "price",
		SecurityID: &security.SecurityID,
		Price:      &price2,
		Direction:  &direction,
	}

	AddAlert(conn, alert1)
	AddAlert(conn, alert2)

	// Verify both alerts exist
	priceAlertsMutex.RLock()
	shard := priceAlertShards[security.SecurityID]
	priceAlertsMutex.RUnlock()

	shard.Mutex.RLock()
	initialCount := len(shard.Alerts)
	shard.Mutex.RUnlock()

	assert.Equal(t, 2, initialCount, "Should have 2 alerts initially")

	// Remove one alert
	RemoveAlert(alert1)

	shard.Mutex.RLock()
	finalCount := len(shard.Alerts)
	_, alert1Exists := shard.Alerts[alert1.AlertID]
	_, alert2Exists := shard.Alerts[alert2.AlertID]
	shard.Mutex.RUnlock()

	assert.Equal(t, 1, finalCount, "Should have 1 alert after removal")
	assert.False(t, alert1Exists, "Alert1 should be removed")
	assert.True(t, alert2Exists, "Alert2 should still exist")

	t.Logf("✅ Successfully removed alert, remaining count: %d", finalCount)
}

func testPriceAlertTriggering(t *testing.T, conn *data.Conn, securities []TestSecurity, priceSnapshot map[int]float64) {
	// Clear existing alerts
	priceAlertsMutex.Lock()
	priceAlertShards = make(map[int]*PriceAlertShard)
	priceAlertsMutex.Unlock()

	security := securities[0]
	currentPrice := priceSnapshot[security.SecurityID] // 150.0

	// Test alert that should trigger (price below current)
	triggerPrice := 140.0
	direction := true // above

	alert := Alert{
		AlertID:    1004,
		UserID:     userID,
		AlertType:  "price",
		SecurityID: &security.SecurityID,
		Price:      &triggerPrice,
		Direction:  &direction,
	}

	AddAlert(conn, alert)

	// Mock the dispatch function to avoid actual notifications
	originalDispatch := dispatchAlert
	var dispatchedAlert *Alert
	dispatchAlert = func(a Alert) error {
		dispatchedAlert = &a
		return nil
	}
	defer func() { dispatchAlert = originalDispatch }()

	// Process price alerts with the test snapshot
	processPriceAlerts(conn, priceSnapshot)

	// Verify the alert was triggered and removed from shard
	assert.NotNil(t, dispatchedAlert, "Alert should have been dispatched")
	assert.Equal(t, alert.AlertID, dispatchedAlert.AlertID)

	priceAlertsMutex.RLock()
	shard := priceAlertShards[security.SecurityID]
	priceAlertsMutex.RUnlock()

	if shard != nil {
		shard.Mutex.RLock()
		_, stillExists := shard.Alerts[alert.AlertID]
		shard.Mutex.RUnlock()
		assert.False(t, stillExists, "Triggered alert should be removed from shard")
	}

	t.Logf("✅ Price alert triggered successfully at $%.2f (trigger: $%.2f)",
		currentPrice, triggerPrice)

	// Test alert that should NOT trigger
	noTriggerPrice := 160.0 // above current price
	alert2 := Alert{
		AlertID:    1005,
		UserID:     userID,
		AlertType:  "price",
		SecurityID: &security.SecurityID,
		Price:      &noTriggerPrice,
		Direction:  &direction,
	}

	AddAlert(conn, alert2)
	dispatchedAlert = nil // reset

	// Process again
	processPriceAlerts(conn, priceSnapshot)

	// Verify the alert was NOT triggered
	assert.Nil(t, dispatchedAlert, "Alert should NOT have been dispatched")

	priceAlertsMutex.RLock()
	shard = priceAlertShards[security.SecurityID]
	priceAlertsMutex.RUnlock()

	if shard != nil {
		shard.Mutex.RLock()
		_, stillExists := shard.Alerts[alert2.AlertID]
		shard.Mutex.RUnlock()
		assert.True(t, stillExists, "Non-triggered alert should remain in shard")
	}

	t.Logf("✅ Price alert correctly NOT triggered at $%.2f (trigger: $%.2f)",
		currentPrice, noTriggerPrice)
}

func testShardBoundaryRecalculation(t *testing.T, conn *data.Conn, securities []TestSecurity) {
	// Clear existing alerts
	priceAlertsMutex.Lock()
	priceAlertShards = make(map[int]*PriceAlertShard)
	priceAlertsMutex.Unlock()

	security := securities[2]

	// Add multiple alerts with different prices
	prices := []float64{2400.0, 2450.0, 2550.0, 2600.0}
	directions := []bool{false, false, true, true} // below, below, above, above

	var alerts []Alert
	for i, price := range prices {
		alert := Alert{
			AlertID:    1005 + i,
			UserID:     userID,
			AlertType:  "price",
			SecurityID: &security.SecurityID,
			Price:      &price,
			Direction:  &directions[i],
		}
		alerts = append(alerts, alert)
		AddAlert(conn, alert)
	}

	priceAlertsMutex.RLock()
	shard := priceAlertShards[security.SecurityID]
	priceAlertsMutex.RUnlock()

	shard.Mutex.RLock()
	initialLowest := shard.LowestAbove
	initialHighest := shard.HighestBelow
	shard.Mutex.RUnlock()

	assert.Equal(t, 2550.0, initialLowest, "LowestAbove should be 2550.0")
	assert.Equal(t, 2450.0, initialHighest, "HighestBelow should be 2450.0")

	// Remove the boundary alert and force recalculation
	RemoveAlert(alerts[2]) // Remove the 2550.0 above alert

	shard.Mutex.Lock()
	shard.recalculateBoundariesIfDirty()
	newLowest := shard.LowestAbove
	shard.Mutex.Unlock()

	assert.Equal(t, 2600.0, newLowest, "LowestAbove should be recalculated to 2600.0")

	t.Logf("✅ Boundary recalculation: %.2f -> %.2f", initialLowest, newLowest)
}

func testConcurrentAlertOperations(t *testing.T, conn *data.Conn, securities []TestSecurity) {
	// Clear existing alerts
	priceAlertsMutex.Lock()
	priceAlertShards = make(map[int]*PriceAlertShard)
	priceAlertsMutex.Unlock()

	security := securities[3]
	var wg sync.WaitGroup
	numGoroutines := 10
	alertsPerGoroutine := 5

	// Concurrently add alerts
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < alertsPerGoroutine; j++ {
				alertID := goroutineID*alertsPerGoroutine + j + 2000
				price := 95.0 + float64(j)
				direction := j%2 == 0

				alert := Alert{
					AlertID:    alertID,
					UserID:     userID,
					AlertType:  "price",
					SecurityID: &security.SecurityID,
					Price:      &price,
					Direction:  &direction,
				}

				AddAlert(conn, alert)
			}
		}(i)
	}

	wg.Wait()

	// Verify all alerts were added
	priceAlertsMutex.RLock()
	shard := priceAlertShards[security.SecurityID]
	priceAlertsMutex.RUnlock()

	assert.NotNil(t, shard, "Shard should exist")

	shard.Mutex.RLock()
	alertCount := len(shard.Alerts)
	shard.Mutex.RUnlock()

	expectedCount := numGoroutines * alertsPerGoroutine
	assert.Equal(t, expectedCount, alertCount,
		"Should have %d alerts after concurrent operations", expectedCount)

	t.Logf("✅ Concurrent operations completed: %d alerts added", alertCount)
}

func testBatchCleanupAlerts(t *testing.T, conn *data.Conn, securities []TestSecurity) {
	ctx := context.Background()

	// Create test alerts in database
	var testAlerts []Alert
	for i := 0; i < 3; i++ {
		var alertID int
		err := conn.DB.QueryRow(ctx,
			`INSERT INTO alerts (userId, securityId, price, direction, active)
			 VALUES ($1, $2, $3, $4, $5) RETURNING alertId`,
			userID, securities[i%len(securities)].SecurityID,
			150.0+float64(i*10), true, true).Scan(&alertID)
		require.NoError(t, err)

		alert := Alert{
			AlertID:    alertID,
			UserID:     userID,
			AlertType:  "price",
			SecurityID: &securities[i%len(securities)].SecurityID,
		}
		testAlerts = append(testAlerts, alert)
	}

	// Test batch cleanup
	err := batchCleanupAlerts(conn, testAlerts)
	require.NoError(t, err)

	// Verify alerts were deactivated
	for _, alert := range testAlerts {
		var active bool
		err := conn.DB.QueryRow(ctx,
			"SELECT active FROM alerts WHERE alertId = $1",
			alert.AlertID).Scan(&active)
		require.NoError(t, err)
		assert.False(t, active, "Alert %d should be deactivated", alert.AlertID)

		// need to check alert logs?????
	}

	t.Logf("✅ Batch cleanup completed for %d alerts", len(testAlerts))
}
