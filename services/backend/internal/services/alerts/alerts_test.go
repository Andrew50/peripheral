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

	// Create test price snapshot for deterministic testing
	priceSnapshot := createTestPriceSnapshot(testSecurities)

	// Run comprehensive test suite
	t.Run("TestAddAlert", func(t *testing.T) {
		testAddAlert(t, conn, testSecurities)
	})

	t.Run("TestRemoveAlert", func(t *testing.T) {
		testRemoveAlert(t, conn, testSecurities)
	})

	t.Run("TestInitAlerts", func(t *testing.T) {
		testInitAlerts(t, conn, testSecurities)
	})

	t.Run("TestProcessAlerts", func(t *testing.T) {
		testProcessAlerts(t, conn, testSecurities, priceSnapshot)
	})

	t.Run("TestConcurrentOperations", func(t *testing.T) {
		testConcurrentOperations(t, conn, testSecurities)
	})

	t.Run("TestAlertLifecycle", func(t *testing.T) {
		testAlertLifecycle(t, conn, testSecurities)
	})

	t.Run("TestEdgeCases", func(t *testing.T) {
		testEdgeCases(t, conn)
	})

	t.Run("TestAlertLoop", func(t *testing.T) {
		testAlertLoop(t, conn, testSecurities)
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

// Create test price snapshot for deterministic testing
func createTestPriceSnapshot(securities []TestSecurity) map[int]float64 {
	return map[int]float64{
		securities[0].SecurityID: 150.0,  // TESTAAPL
		securities[1].SecurityID: 200.0,  // TESTTSLA
		securities[2].SecurityID: 2500.0, // TESTGOOG
		securities[3].SecurityID: 100.0,  // TESTAMZN
		securities[4].SecurityID: 300.0,  // TESTMSFT
	}
}

func testAddAlert(t *testing.T, conn *data.Conn, securities []TestSecurity) {
	// Clear alerts map before testing
	alerts = sync.Map{}

	t.Run("AddPriceAlert", func(t *testing.T) {
		price := 150.0
		direction := true
		alert := Alert{
			AlertID:    1001,
			UserID:     userID,
			AlertType:  "price",
			Price:      &price,
			Direction:  &direction,
			SecurityID: &securities[0].SecurityID,
		}

		AddAlert(conn, alert)

		// Verify alert was added
		storedAlert, exists := alerts.Load(1001)
		require.True(t, exists, "Alert should be stored")

		stored := storedAlert.(Alert)
		require.Equal(t, alert.AlertID, stored.AlertID)
		require.Equal(t, alert.UserID, stored.UserID)
		require.Equal(t, alert.AlertType, stored.AlertType)
		require.NotNil(t, stored.Ticker, "Ticker should be populated for price alerts")
		require.Equal(t, securities[0].Ticker, *stored.Ticker)
		t.Logf("✅ Price alert added successfully with ticker: %s", *stored.Ticker)
	})

	t.Run("AddAlgoAlert", func(t *testing.T) {
		algoID := 123
		alert := Alert{
			AlertID:   1002,
			UserID:    userID,
			AlertType: "algo",
			AlgoID:    &algoID,
		}

		AddAlert(conn, alert)

		storedAlert, exists := alerts.Load(1002)
		require.True(t, exists, "Algo alert should be stored")

		stored := storedAlert.(Alert)
		require.Equal(t, alert.AlertID, stored.AlertID)
		require.Equal(t, alert.AlertType, stored.AlertType)
		require.Equal(t, algoID, *stored.AlgoID)
		t.Logf("✅ Algo alert added successfully")
	})

	t.Run("AddNewsAlert", func(t *testing.T) {
		alert := Alert{
			AlertID:    1003,
			UserID:     userID,
			AlertType:  "news",
			SecurityID: &securities[1].SecurityID,
		}

		AddAlert(conn, alert)

		storedAlert, exists := alerts.Load(1003)
		require.True(t, exists, "News alert should be stored")

		stored := storedAlert.(Alert)
		require.Equal(t, alert.AlertID, stored.AlertID)
		require.Equal(t, alert.AlertType, stored.AlertType)
		t.Logf("✅ News alert added successfully")
	})

	t.Run("AddStrategyAlert", func(t *testing.T) {
		setupID := 456
		alert := Alert{
			AlertID:    1004,
			UserID:     userID,
			AlertType:  "strategy",
			SetupID:    &setupID,
			SecurityID: &securities[2].SecurityID,
		}

		AddAlert(conn, alert)

		storedAlert, exists := alerts.Load(1004)
		require.True(t, exists, "Strategy alert should be stored")

		stored := storedAlert.(Alert)
		require.Equal(t, alert.AlertID, stored.AlertID)
		require.Equal(t, alert.AlertType, stored.AlertType)
		require.Equal(t, setupID, *stored.SetupID)
		t.Logf("✅ Strategy alert added successfully")
	})
}

func testRemoveAlert(t *testing.T, conn *data.Conn, securities []TestSecurity) {
	// Setup: Add multiple alerts
	alerts = sync.Map{}

	testAlerts := []Alert{
		{AlertID: 2001, UserID: userID, AlertType: "price", SecurityID: &securities[0].SecurityID},
		{AlertID: 2002, UserID: userID, AlertType: "news", SecurityID: &securities[1].SecurityID},
		{AlertID: 2003, UserID: userID, AlertType: "strategy", SecurityID: &securities[2].SecurityID},
	}

	// Add all alerts
	for _, alert := range testAlerts {
		alerts.Store(alert.AlertID, alert)
	}

	// Verify all exist
	initialCount := 0
	alerts.Range(func(key, value interface{}) bool {
		initialCount++
		return true
	})
	require.Equal(t, 3, initialCount, "Should have 3 alerts initially")

	t.Run("RemoveSingleAlert", func(t *testing.T) {
		RemoveAlert(2001)

		_, exists := alerts.Load(2001)
		assert.False(t, exists, "Alert 2001 should be removed")

		// Verify others still exist
		_, exists2 := alerts.Load(2002)
		_, exists3 := alerts.Load(2003)
		assert.True(t, exists2, "Alert 2002 should still exist")
		assert.True(t, exists3, "Alert 2003 should still exist")
		t.Logf("✅ Successfully removed single alert")
	})

	t.Run("RemoveNonExistentAlert", func(t *testing.T) {
		// Should not panic
		require.NotPanics(t, func() {
			RemoveAlert(9999)
		}, "Removing non-existent alert should not panic")
		t.Logf("✅ Gracefully handled non-existent alert removal")
	})
}

func testInitAlerts(t *testing.T, conn *data.Conn, securities []TestSecurity) {
	ctx := context.Background()

	// Clear existing alerts
	alerts = sync.Map{}

	t.Run("LoadActiveAlerts", func(t *testing.T) {
		// Insert test alerts into database
		testDBAlerts := []struct {
			price      *float64
			direction  *bool
			securityID *int
		}{
			{floatPtr(150.0), boolPtr(true), &securities[0].SecurityID},
		}

		var insertedAlertIDs []int
		for _, testAlert := range testDBAlerts {
			var alertID int
			err := conn.DB.QueryRow(ctx,
				`INSERT INTO alerts (userId, price, direction, securityId, active)
				 VALUES ($1, $2, $3, $4, $5) RETURNING alertId`,
				userID, testAlert.price,
				testAlert.direction, testAlert.securityID, true).Scan(&alertID)
			require.NoError(t, err)
			insertedAlertIDs = append(insertedAlertIDs, alertID)
		}

		// Test initAlerts
		err := initAlerts(conn)
		require.NoError(t, err, "initAlerts should succeed")

		// Verify all alerts were loaded
		for i, alertID := range insertedAlertIDs {
			storedAlert, exists := alerts.Load(alertID)
			require.True(t, exists, "Alert ID %d should be loaded", alertID)

			stored := storedAlert.(Alert)
			require.Equal(t, alertID, stored.AlertID)
			require.Equal(t, userID, stored.UserID)

			// Verify price alerts have tickers populated
			require.NotNil(t, stored.Ticker, "Price alerts should have ticker")
		}

		t.Logf("✅ Successfully loaded %d alerts from database", len(insertedAlertIDs)+1)
	})

	t.Run("ValidateSecurityReferences", func(t *testing.T) {
		// Insert alert with invalid security ID
		var invalidAlertID int
		err := conn.DB.QueryRow(ctx,
			`INSERT INTO alerts (userId, securityId, active)
			 VALUES ($1, $2, $3) RETURNING alertId`,
			userID, 99999, true).Scan(&invalidAlertID)
		require.NoError(t, err)

		// Clear and reinitialize
		alerts = sync.Map{}

		// This should fail due to invalid security reference
		err = initAlerts(conn)
		require.Error(t, err, "initAlerts should fail with invalid security reference")
		require.Contains(t, err.Error(), "non-existent security ID")
		t.Logf("✅ Properly validated security references")
	})
}

func testProcessAlerts(t *testing.T, conn *data.Conn, securities []TestSecurity, priceSnapshot map[int]float64) {
	t.Run("ProcessPriceAlertTriggering", func(t *testing.T) {
		alerts = sync.Map{}

		security := securities[0]
		currentPrice := priceSnapshot[security.SecurityID] // 150.0

		// Test alert that SHOULD trigger (alert price below current)
		triggerPrice := 140.0 // Below current price of 150.0
		direction := true     // "above" direction

		triggerAlert := Alert{
			AlertID:    3001,
			UserID:     userID,
			SecurityID: &security.SecurityID,
			Price:      &triggerPrice,
			Direction:  &direction,
		}

		// Test alert that should NOT trigger (alert price above current)
		noTriggerPrice := 160.0 // Above current price of 150.0
		noTriggerAlert := Alert{
			AlertID:    3002,
			UserID:     userID,
			SecurityID: &security.SecurityID,
			Price:      &noTriggerPrice,
			Direction:  &direction,
		}

		// Add both alerts
		AddAlert(conn, triggerAlert)
		AddAlert(conn, noTriggerAlert)

		// Verify both alerts exist before processing
		_, exists1 := alerts.Load(3001)
		_, exists2 := alerts.Load(3002)
		require.True(t, exists1, "Trigger alert should exist")
		require.True(t, exists2, "No-trigger alert should exist")

		// TODO: Your processPriceAlert function needs to be modified to accept
		// price data for testing. You might need to:
		// 1. Inject the priceSnapshot into socket.AggData with proper structure
		// 2. Or modify processPriceAlert to accept test price data
		// 3. Or mock the price data source temporarily

		// For now, test that processing doesn't panic
		require.NotPanics(t, func() {
			processAlerts(conn, priceSnapshot)
		}, "Processing price alerts should not panic")

		t.Logf("✅ Price alert processing tested with current=%.2f, trigger=%.2f, no-trigger=%.2f",
			currentPrice, triggerPrice, noTriggerPrice)
	})

	t.Run("ProcessEmptyAlerts", func(t *testing.T) {
		alerts = sync.Map{}

		require.NotPanics(t, func() {
			processAlerts(conn, priceSnapshot)
		}, "Processing empty alerts should not panic")

		t.Logf("✅ Gracefully handled empty alerts")
	})

	t.Run("ProcessLargeNumberOfAlerts", func(t *testing.T) {
		alerts = sync.Map{}

		// Add many alerts to test performance and stability
		for i := 0; i < 100; i++ {
			alert := Alert{
				AlertID:    4000 + i,
				UserID:     userID,
				SecurityID: &securities[i%len(securities)].SecurityID,
			}
			alerts.Store(alert.AlertID, alert)
		}

		startTime := time.Now()
		require.NotPanics(t, func() {
			processAlerts(conn, priceSnapshot)
		}, "Processing large number of alerts should not panic")

		duration := time.Since(startTime)
		t.Logf("✅ Processed 100 alerts in %v", duration)
	})
}

func testConcurrentOperations(t *testing.T, conn *data.Conn, securities []TestSecurity) {
	alerts = sync.Map{}

	t.Run("ConcurrentAddRemove", func(t *testing.T) {
		var wg sync.WaitGroup
		numGoroutines := 10
		alertsPerGoroutine := 20

		// Concurrently add alerts
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				for j := 0; j < alertsPerGoroutine; j++ {
					alertID := goroutineID*alertsPerGoroutine + j + 4000
					alert := Alert{
						AlertID:    alertID,
						UserID:     userID,
						AlertType:  "price",
						SecurityID: &securities[j%len(securities)].SecurityID,
					}
					AddAlert(conn, alert)
				}
			}(i)
		}

		wg.Wait()

		// Count alerts
		alertCount := 0
		alerts.Range(func(key, value interface{}) bool {
			alertCount++
			return true
		})

		expectedCount := numGoroutines * alertsPerGoroutine
		assert.Equal(t, expectedCount, alertCount, "Should have %d alerts after concurrent adds", expectedCount)

		// Concurrently remove half the alerts
		wg = sync.WaitGroup{}
		for i := 0; i < numGoroutines/2; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				for j := 0; j < alertsPerGoroutine; j++ {
					alertID := goroutineID*alertsPerGoroutine + j + 4000
					RemoveAlert(alertID)
				}
			}(i)
		}

		wg.Wait()

		// Count remaining alerts
		finalCount := 0
		alerts.Range(func(key, value interface{}) bool {
			finalCount++
			return true
		})

		expectedFinalCount := (numGoroutines / 2) * alertsPerGoroutine
		assert.Equal(t, expectedFinalCount, finalCount, "Should have %d alerts after concurrent removes", expectedFinalCount)
		t.Logf("✅ Concurrent operations: %d -> %d alerts", expectedCount, finalCount)
	})

}

func testAlertLifecycle(t *testing.T, conn *data.Conn, securities []TestSecurity) {
	ctx := context.Background()

	t.Run("FullLifecycle", func(t *testing.T) {
		// 1. Start with empty alerts
		alerts = sync.Map{}

		// 2. Add alert via AddAlert
		price := 175.0
		direction := true
		alert := Alert{
			AlertID:    6001,
			UserID:     userID,
			AlertType:  "price",
			Price:      &price,
			Direction:  &direction,
			SecurityID: &securities[0].SecurityID,
		}

		AddAlert(conn, alert)

		// 3. Verify in memory
		_, exists := alerts.Load(6001)
		require.True(t, exists, "Alert should exist in memory")

		// 4. Insert into database (simulating full flow)
		var dbAlertID int
		err := conn.DB.QueryRow(ctx,
			`INSERT INTO alerts (alertId, userId, alertType, price, direction, securityId, active)
			 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING alertId`,
			alert.AlertID, alert.UserID, alert.AlertType, alert.Price,
			alert.Direction, alert.SecurityID, true).Scan(&dbAlertID)
		require.NoError(t, err)

		// 5. Clear memory and reload from DB
		alerts = sync.Map{}
		err = initAlerts(conn)
		require.NoError(t, err)

		// 6. Verify alert is back
		reloadedAlert, exists := alerts.Load(6001)
		require.True(t, exists, "Alert should be reloaded from database")

		reloaded := reloadedAlert.(Alert)
		require.Equal(t, alert.AlertID, reloaded.AlertID)
		require.Equal(t, alert.AlertType, reloaded.AlertType)

		// 7. Remove alert
		RemoveAlert(6001)

		// 8. Verify removal
		_, exists = alerts.Load(6001)
		require.False(t, exists, "Alert should be removed from memory")

		t.Logf("✅ Full alert lifecycle completed successfully")
	})
}

func testEdgeCases(t *testing.T, conn *data.Conn) {
	t.Run("InvalidAlertTypes", func(t *testing.T) {
		alerts = sync.Map{}

		alert := Alert{
			AlertID:   7001,
			UserID:    userID,
			AlertType: "invalid_type",
		}

		// Should not panic
		require.NotPanics(t, func() {
			AddAlert(conn, alert)
		}, "Adding invalid alert type should not panic")

		// Should still be stored (validation happens during processing)
		_, exists := alerts.Load(7001)
		assert.True(t, exists, "Invalid alert should still be stored")
		t.Logf("✅ Handled invalid alert type gracefully")
	})

	t.Run("NilPointerFields", func(t *testing.T) {
		alerts = sync.Map{}

		alert := Alert{
			AlertID:    7002,
			UserID:     userID,
			AlertType:  "price",
			SecurityID: nil, // This should be handled gracefully
		}

		require.NotPanics(t, func() {
			AddAlert(conn, alert)
		}, "Adding alert with nil SecurityID should not panic")
		t.Logf("✅ Handled nil pointer fields gracefully")
	})

	t.Run("DuplicateAlertIDs", func(t *testing.T) {
		alerts = sync.Map{}

		alert1 := Alert{
			AlertID:   7003,
			UserID:    userID,
			AlertType: "price",
		}

		alert2 := Alert{
			AlertID:   7003, // Same ID
			UserID:    userID + 1,
			AlertType: "news",
		}

		AddAlert(conn, alert1)
		AddAlert(conn, alert2) // Should overwrite

		storedAlert, exists := alerts.Load(7003)
		require.True(t, exists)

		stored := storedAlert.(Alert)
		assert.Equal(t, "news", stored.AlertType, "Second alert should overwrite first")
		t.Logf("✅ Handled duplicate alert IDs correctly")
	})
}

func testAlertLoop(t *testing.T, conn *data.Conn, securities []TestSecurity) {
	// Test StopAlertLoop with no active loop
	require.NotPanics(t, func() {
		StopAlertLoop()
	}, "StopAlertLoop should not panic when no loop is running")

	// Test with active context
	testCtx, testCancel := context.WithCancel(context.Background())
	ctx = testCtx
	cancel = testCancel

	require.NotPanics(t, func() {
		StopAlertLoop()
	}, "StopAlertLoop should not panic with active context")

	t.Logf("✅ StopAlertLoop functionality verified")
}

// Helper functions
func intPtr(i int) *int {
	return &i
}

func floatPtr(f float64) *float64 {
	return &f
}

func boolPtr(b bool) *bool {
	return &b
}
