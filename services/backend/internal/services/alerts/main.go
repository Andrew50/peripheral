package alerts

import (
	"backend/internal/data"
	"backend/internal/data/postgres"
	"backend/internal/services/socket"
	"context"
	"fmt"

	//"log"
	"math"
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

type PriceAlertShard struct {
	Mutex        sync.RWMutex
	Alerts       map[int]Alert
	LowestAbove  float64
	HighestBelow float64
	IsDirty      bool
}

var (
	frequency = time.Second * 1
	ctx       context.Context
	cancel    context.CancelFunc

	priceAlertsMutex sync.RWMutex
	priceAlertShards map[int]*PriceAlertShard

	mu             sync.Mutex
	nonPriceAlerts sync.Map
)

// AddAlert adds an alert to the in memory store, updating price shard info when needed
func AddAlert(conn *data.Conn, alert Alert) {
	println("entry")

	if alert.AlertType == "price" {
		if alert.SecurityID == nil {
			// log the error
			println("nil security ID")
			return
		}
		ticker, err := postgres.GetTicker(conn, *alert.SecurityID, time.Now())
		if err != nil {
			fmt.Println("error getting ticker: %w", err)
			return
		}
		alert.Ticker = &ticker

		priceAlertsMutex.Lock()
		defer priceAlertsMutex.Unlock()

		if priceAlertShards == nil {
			priceAlertShards = make(map[int]*PriceAlertShard)
		}

		shard, exists := priceAlertShards[*alert.SecurityID]

		if !exists {
			shard = &PriceAlertShard{
				Alerts:       map[int]Alert{},
				LowestAbove:  math.MaxFloat64,
				HighestBelow: 0,
			}
			priceAlertShards[*alert.SecurityID] = shard
		}

		shard.Mutex.Lock()
		defer shard.Mutex.Unlock()

		shard.Alerts[alert.AlertID] = alert

		// update thresholds
		if alert.Direction != nil && alert.Price != nil {
			if *alert.Direction {
				if *alert.Price < shard.LowestAbove {
					shard.LowestAbove = *alert.Price
					shard.IsDirty = false
				}
			} else {
				if *alert.Price > shard.HighestBelow {
					shard.HighestBelow = *alert.Price
					shard.IsDirty = false
				}
			}
		}
	} else {
		nonPriceAlerts.Store(alert.AlertID, alert)
	}
	println("ur mom")

}

// RemoveAlert removes an alert from the in-memory store
func RemoveAlertFromID(alertID int) {
	// TODO: make this for hte price alerts

	// tries to delete from the sync map
	if _, deleted := nonPriceAlerts.LoadAndDelete(alertID); deleted {
		return
	}

	priceAlertsMutex.Lock()
	defer priceAlertsMutex.Unlock()

	for _, shard := range priceAlertShards {
		shard.Mutex.Lock()
		defer shard.Mutex.Unlock()

		if alert, exists := shard.Alerts[alertID]; exists {
			delete(shard.Alerts, alertID)

			if len(shard.Alerts) == 0 {
				delete(priceAlertShards, *alert.SecurityID)
			} else {
				if alert.Price != nil && alert.Direction != nil {
					alertWasBoundary := (*alert.Direction && shard.LowestAbove == *alert.Price) ||
						(!*alert.Direction && shard.HighestBelow == *alert.Price)
					if alertWasBoundary {
						shard.IsDirty = true
					}
				}
			}
			return
		}
	}

}

// RemoveAlert removes an alert from the in-memory store
func RemoveAlert(alert Alert) {
	// TODO: make this for hte price alerts

	// TODO: need to be able to delete the shard itself
	if alert.AlertType != "price" {
		nonPriceAlerts.Delete(alert.AlertID)
		return
	}

	if alert.SecurityID == nil {
		// TODO: log invalid alert
		return
	}

	priceAlertsMutex.Lock()
	defer priceAlertsMutex.Unlock()

	shard, exist := priceAlertShards[*alert.SecurityID]
	if !exist {
		return
	}

	shard.Mutex.Lock()

	delete(shard.Alerts, alert.AlertID)

	if len(shard.Alerts) == 0 {
		shard.Mutex.Unlock()
		delete(priceAlertShards, *alert.SecurityID)
	} else {
		if alert.Price != nil && alert.Direction != nil {
			alertWasBoundary := (*alert.Direction && shard.LowestAbove == *alert.Price) ||
				(!*alert.Direction && shard.HighestBelow == *alert.Price)
			if alertWasBoundary {
				shard.IsDirty = true
			}
		}
		shard.Mutex.Unlock()
	}

}

func (shard *PriceAlertShard) recalculateBoundariesIfDirty() {
	if !shard.IsDirty {
		return
	}

	shard.LowestAbove = math.MaxFloat64
	shard.HighestBelow = 0

	for _, alert := range shard.Alerts {
		if alert.Direction == nil || alert.Price == nil {
			continue
		}

		if *alert.Direction {
			if *alert.Price < shard.LowestAbove {
				shard.LowestAbove = *alert.Price
			}
		} else {
			if *alert.Price > shard.HighestBelow {
				shard.HighestBelow = *alert.Price
			}
		}
	}

	shard.IsDirty = false
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
	priceSnapshot := getCurrentPriceSnapshot()

	processPriceAlerts(conn, priceSnapshot)

	processNonPriceAlerts(conn)
}

func processPriceAlerts(conn *data.Conn, snapshot map[int]float64) {
	var triggeredAlerts []Alert

	priceAlertsMutex.RLock()
	defer priceAlertsMutex.RUnlock()

	// iterate through the shards and quickly check if any could be triggered
	for securityID, shard := range priceAlertShards {
		currentPrice, exists := snapshot[securityID]
		if !exists {
			continue
		}

		shard.Mutex.Lock()

		shard.recalculateBoundariesIfDirty()

		if currentPrice > shard.HighestBelow && currentPrice < shard.LowestAbove {
			shard.Mutex.Unlock()
			continue
		}

		curLowestAbove := math.MaxFloat64
		curHighestBelow := 0.0

		// if alerts can be triggered we then go through dispatching them and updating the alerts
		for _, alert := range shard.Alerts {
			if alert.Direction == nil || alert.Price == nil {
				continue
			}

			if *alert.Direction {
				if currentPrice >= *alert.Price {
					triggeredAlerts = append(triggeredAlerts, alert)
				} else if curLowestAbove > *alert.Price {
					curLowestAbove = *alert.Price
				}
			} else {
				if currentPrice <= *alert.Price {
					triggeredAlerts = append(triggeredAlerts, alert)
				} else if curHighestBelow < *alert.Price {
					curHighestBelow = *alert.Price
				}
			}
		}

		for _, alert := range triggeredAlerts {
			delete(shard.Alerts, alert.AlertID)
		}
		if len(triggeredAlerts) > 0 {
			shard.HighestBelow = curHighestBelow
			shard.LowestAbove = curLowestAbove
			shard.IsDirty = false
		}

		shard.Mutex.Unlock()
	}

	var successfulDispatches []Alert
	for _, alert := range triggeredAlerts {
		err := dispatchAlert(alert)
		if err != nil {
			AddAlert(conn, alert) // need to retry the send later
			// TODO: log
		} else {
			successfulDispatches = append(successfulDispatches, alert)
		}
	}

	if len(successfulDispatches) > 0 {
		err := batchCleanupAlerts(conn, successfulDispatches)
		if err != nil {
			return
		}
	}

}

func batchCleanupAlerts(conn *data.Conn, alerts []Alert) error {
	if len(alerts) == 0 {
		return nil
	}

	tx, err := conn.DB.Begin(context.Background())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())

	timestamp := time.Now()

	alertIDs := make([]int, len(alerts))

	// IDK if this exists, i am erroring every time
	for i, alert := range alerts {
		alertIDs[i] = alert.AlertID

		continue // FORNOW

		if alert.SecurityID == nil {
			// log this it shouldnt happen
			continue
		}

		query := `
		INSERT INTO alertLogs (alertId, timestamp, securityId)
		VALUES ($1, $2, $3)
		`

		_, err := tx.Exec(context.Background(),
			query,
			alert.AlertID,
			timestamp,
			*alert.SecurityID,
		)

		if err != nil {
			//log.Printf("Failed to log alert to database: %v", err)
			fmt.Printf("wowow %v", err)
			return fmt.Errorf("failed to log alert: %v", err)
		}

	}

	// Disable the alert by setting its active status to false
	updateQuery := `
	UPDATE alerts
	SET active = false
	WHERE alertId = ANY($1)
	`
	_, err = tx.Exec(context.Background(), updateQuery, alertIDs)
	if err != nil {
		//log.Printf("Failed to disable alert with ID %d: %v", alert.AlertID, err)
		return fmt.Errorf("failed to disable alert: %v", err)
	}

	return tx.Commit(context.Background())

}

func cleanupEmptyShards() {
	priceAlertsMutex.Lock()
	defer priceAlertsMutex.Unlock()

	for securityID, shard := range priceAlertShards {
		shard.Mutex.RLock()
		isEmpty := len(shard.Alerts) == 0
		shard.Mutex.RUnlock()

		if isEmpty {
			delete(priceAlertShards, securityID)
		}
	}
}

func processNonPriceAlerts(conn *data.Conn) {
	var wg sync.WaitGroup
	nonPriceAlerts.Range(func(_, value interface{}) bool {
		alert := value.(Alert)
		wg.Add(1)
		go func(a Alert) {
			defer wg.Done()
			var err error
			switch a.AlertType {
			case "price":
				err = processPriceAlert(conn, a) // probably depricated but dont wnat to remove yet
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

	nonPriceAlerts = sync.Map{}
	priceAlertShards = map[int]*PriceAlertShard{}

	println("created maps")
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

		AddAlert(conn, alert)

	}

	if err = rows.Err(); err != nil {
		return fmt.Errorf("iterating alert rows: %w", err)
	}

	// Manually create an algo alert for testing
	algoAlert := Alert{
		AlertID:    999999, // Use a high number to avoid conflicts
		UserID:     1,      // Set to an existing user ID
		AlertType:  "algo",
		SecurityID: nil, // Not needed for algo alerts
		Price:      nil, // Not needed for algo alerts
		Direction:  nil, // Not needed for algo alerts
		SetupID:    nil, // Not needed for algo alerts
		Ticker:     nil, // Not needed for algo alerts
	}
	nonPriceAlerts.Store(algoAlert.AlertID, algoAlert)
	////fmt.Println("Added manual algo alert for testing")

	// Validate alert securities exist in data map
	var alertErrors []error
	nonPriceAlerts.Range(func(_, value interface{}) bool {
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

	priceAlertsMutex.RLock()
	for securityID, _ := range priceAlertShards {
		if _, exists := socket.AggData[securityID]; !exists {
			alertErrors = append(alertErrors,
				fmt.Errorf("shard references non-existent security ID %d",
					securityID))
		}
	}
	priceAlertsMutex.RUnlock()

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

	////fmt.Println("Finished initializing alerts")
	return nil
}
