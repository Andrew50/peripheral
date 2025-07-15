package alerts

import (
	"backend/internal/data"
	"backend/internal/data/postgres"

	//"backend/internal/services/socket"
	"backend/internal/app/limits"
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Alert represents a structure for handling Alert data.
type Alert struct {
	AlertID    int
	UserID     int
	AlertType  string
	AlgoID     *int
	SetupID    *int
	Price      *float64
	Direction  *bool
	SecurityID *int
	Ticker     *string
	//Message    *string
}

var (
	frequency = time.Second * 1
	alerts    sync.Map
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.Mutex
)

// AddAlert performs operations related to AddAlert functionality.
func AddAlert(conn *data.Conn, alert Alert) {
	if alert.AlertType == "price" {
		ticker, err := postgres.GetTicker(conn, *alert.SecurityID, time.Now())
		if err != nil {
			////fmt.Println("error getting ticker: %w", err)
			return
		}
		alert.Ticker = &ticker
	}
	alerts.Store(alert.AlertID, alert)
}

// RemoveAlert removes an alert from the in-memory store and decrements the counter
func RemoveAlert(conn *data.Conn, alertID int) error {
	mu.Lock()
	defer mu.Unlock()

	// Get the alert before removing it to access user information
	if alertInterface, exists := alerts.Load(alertID); exists {
		alert := alertInterface.(Alert)

		// Only decrement counter for real alerts (not system alerts like strategy processor)
		if alert.UserID > 0 {
			// Determine which counter to decrement based on alert type
			if alert.AlertType == "strategy" {
				// Decrement the active strategy alerts counter
				if err := limits.DecrementActiveStrategyAlerts(conn, alert.UserID, 1); err != nil {
					return fmt.Errorf("failed to decrement active strategy alerts counter for user %d: %w", alert.UserID, err)
				}
			} else {
				// Decrement the active alerts counter for regular alerts (price, news, etc.)
				if err := limits.DecrementActiveAlerts(conn, alert.UserID, 1); err != nil {
					return fmt.Errorf("failed to decrement active alerts counter for user %d: %w", alert.UserID, err)
				}
			}
		}
	}

	alerts.Delete(alertID)
	return nil
}

// RemoveAlertFromMemory removes an alert from the in-memory store without decrementing counters
// This is used when the counter has already been decremented elsewhere
func RemoveAlertFromMemory(alertID int) {
	mu.Lock()
	defer mu.Unlock()
	alerts.Delete(alertID)
}

// StartAlertLoop performs operations related to StartAlertLoop functionality.
func StartAlertLoop(conn *data.Conn) error { //entrypoint
	err := InitTelegramBot()
	if err != nil {
		return err
	}
	if err := initAlerts(conn); err != nil {
		////fmt.Println("error : god0ws")
		return err
	}

	ctx, cancel = context.WithCancel(context.Background())
	go alertLoop(ctx, conn)
	return nil
}

// StopAlertLoop performs operations related to StopAlertLoop functionality.
func StopAlertLoop() {
	if cancel != nil {
		cancel()
	}
}

func alertLoop(ctx context.Context, conn *data.Conn) {
	ticker := time.NewTicker(frequency)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			processAlerts(conn)
		}
	}
}

func processAlerts(conn *data.Conn) {
	var wg sync.WaitGroup
	alerts.Range(func(_, value interface{}) bool {
		alert := value.(Alert)
		wg.Add(1)
		go func(a Alert) {
			defer wg.Done()
			var err error
			switch a.AlertType {
			case "price":
				err = processPriceAlert(conn, a)
			case "news":
				err = processNewsAlert(conn, a)
			case "strategy":
				err = processStrategyAlert(conn, a)
			default:
				//log.Printf("Unknown alert type: %s", a.AlertType)
				return
			}
			if err != nil {
				//log.Printf("Error processing alert %d: %v", a.AlertID, err)
				return
			}
		}(alert)
		return true
	})
	wg.Wait()
}

func initAlerts(conn *data.Conn) error {
	ctx := context.Background()

	// Load active alerts
	query := `
        SELECT alertId, userId, alertType, setupId, price, direction, securityId
        FROM alerts
        WHERE active = true
    `
	rows, err := conn.DB.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("querying active alerts: %w", err)
	}
	defer rows.Close()

	alerts = sync.Map{}
	for rows.Next() {
		var alert Alert
		err := rows.Scan(
			&alert.AlertID,
			&alert.UserID,
			&alert.AlertType,
			&alert.SetupID,
			&alert.Price,
			&alert.Direction,
			&alert.SecurityID,
		)
		if err != nil {
			return fmt.Errorf("scanning alert row: %w", err)
		}
		if alert.AlertType == "price" {
			ticker, err := postgres.GetTicker(conn, *alert.SecurityID, time.Now())
			if err != nil {
				////fmt.Println("error getting ticker: %w", err)
				return fmt.Errorf("getting ticker: %w", err)
			}
			alert.Ticker = &ticker
		}

		alerts.Store(alert.AlertID, alert)
	}

	if err = rows.Err(); err != nil {
		return fmt.Errorf("iterating alert rows: %w", err)
	}

	// Add a strategy alert that processes all active strategy alerts
	strategyAlert := Alert{
		AlertID:    999998, // Use a high number to avoid conflicts
		UserID:     0,      // System alert, not user-specific
		AlertType:  "strategy",
		SecurityID: nil, // Not needed for strategy alerts
		Price:      nil, // Not needed for strategy alerts
		Direction:  nil, // Not needed for strategy alerts
		SetupID:    nil, // Not needed for strategy alerts
		Ticker:     nil, // Not needed for strategy alerts
	}
	alerts.Store(strategyAlert.AlertID, strategyAlert)
	log.Printf("Added strategy alert processor")

	// Validate alert securities exist in data map
	/*
		var alertErrors []error
		alerts.Range(func(_, value interface{}) bool {
			alert := value.(Alert)
			if alert.SecurityID != nil {
				if _, exists := socket.AggData[*alert.SecurityID]; !exists {
					alertErrors = append(alertErrors,
						fmt.Errorf("alert ID %d references non-existent security ID %d",
							alert.AlertID, *alert.SecurityID))
				}
			}
			return true
		})

		// Report any alert validation errors
		if len(alertErrors) > 0 {
			var errMsg string
			for i, err := range alertErrors {
				if i > 0 {
					errMsg += "; "
				}
				errMsg += err.Error()
			}
			return fmt.Errorf("errors validating alerts: %s", errMsg)
		}
	*/

	////fmt.Println("Finished initializing alerts")
	return nil
}
