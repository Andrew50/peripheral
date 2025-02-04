package alerts

import (
	"backend/socket"
	"backend/utils"
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

type Alert struct {
	AlertId    int
	UserId     int
	AlertType  string
	AlgoId     *int
	SetupId    *int
	Price      *float64
	Direction  *bool
	SecurityId *int
	Ticker     *string
	//Message    *string
}

var (
	frequency = time.Second * 1
	alerts    sync.Map
	ctx       context.Context
	cancel    context.CancelFunc
)

func AddAlert(conn *utils.Conn, alert Alert) {
	if alert.AlertType == "price" {
		ticker, err := utils.GetTicker(conn, *alert.SecurityId, time.Now())
		if err != nil {
			fmt.Println("error getting ticker: %w", err)
			return
		}
		alert.Ticker = &ticker
	}
	alerts.Store(alert.AlertId, alert)
}

func RemoveAlert(alertId int) {
	alerts.Delete(alertId)
}

func StartAlertLoop(conn *utils.Conn) error { //entrypoint
	err := InitTelegramBot()
	if err != nil {
		return err
	}
	if err := initAlerts(conn); err != nil {
		fmt.Println("error : god0ws")
		return err
	}

	ctx, cancel = context.WithCancel(context.Background())
	go alertLoop(ctx, conn)
	return nil
}

func StopAlertLoop() {
	if cancel != nil {
		cancel()
	}
}

func alertLoop(ctx context.Context, conn *utils.Conn) {
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

func printAlert(alert Alert) {
	fmt.Printf("AlertId: %d, UserId: %d, AlertType: %s, SetupId: %v, Price: %v, Direction: %v, SecurityId: %v, Ticker: %v\n", alert.AlertId, alert.UserId, alert.AlertType, nilOrValue(alert.SetupId), nilOrValue(alert.Price), nilOrValue(alert.Direction), nilOrValue(alert.SecurityId), nilOrValue(alert.Ticker))
}

func nilOrValue[T any](ptr *T) any {
	if ptr == nil {
		return "nil"
	}
	return *ptr
}

func processAlerts(conn *utils.Conn) {
	var wg sync.WaitGroup
	alerts.Range(func(key, value interface{}) bool {
		alert := value.(Alert)
		printAlert(alert)
		wg.Add(1)
		go func(a Alert) {
			defer wg.Done()
			var err error
			switch a.AlertType {
			case "price":
				err = processPriceAlert(conn, a)
			case "setup":
				err = processSetupAlert(conn, a)
			case "algo":
				err = processAlgoAlert(conn, a)
			default:
				log.Printf("Unknown alert type: %s", a.AlertType)
				return
			}
			if err != nil {
				log.Printf("Error processing alert %d: %v", a.AlertId, err)
				return
			}
		}(alert)
		return true
	})
	wg.Wait()
}

func initAlerts(conn *utils.Conn) error {
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
			&alert.AlertId,
			&alert.UserId,
			&alert.AlertType,
			&alert.SetupId,
			&alert.Price,
			&alert.Direction,
			&alert.SecurityId,
		)
		if err != nil {
			return fmt.Errorf("scanning alert row: %w", err)
		}
		if alert.AlertType == "price" {
			ticker, err := utils.GetTicker(conn, *alert.SecurityId, time.Now())
			if err != nil {
				fmt.Println("error getting ticker: %w", err)
				return fmt.Errorf("getting ticker: %w", err)
			}
			alert.Ticker = &ticker
		}

		alerts.Store(alert.AlertId, alert)
	}

	if err = rows.Err(); err != nil {
		return fmt.Errorf("iterating alert rows: %w", err)
	}

	// Manually create an algo alert for testing
	algoAlert := Alert{
		AlertId:    999999, // Use a high number to avoid conflicts
		UserId:     1,      // Set to an existing user ID
		AlertType:  "algo",
		SecurityId: nil, // Not needed for algo alerts
		Price:      nil, // Not needed for algo alerts
		Direction:  nil, // Not needed for algo alerts
		SetupId:    nil, // Not needed for algo alerts
		Ticker:     nil, // Not needed for algo alerts
	}
	alerts.Store(algoAlert.AlertId, algoAlert)
	fmt.Println("Added manual algo alert for testing")

	// Validate alert securities exist in data map
	var alertErrors []error
	alerts.Range(func(key, value interface{}) bool {
		alert := value.(Alert)
		if alert.SecurityId != nil {
			if _, exists := socket.AggData[*alert.SecurityId]; !exists {
				alertErrors = append(alertErrors,
					fmt.Errorf("alert ID %d references non-existent security ID %d",
						alert.AlertId, *alert.SecurityId))
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

	fmt.Println("Finished initializing alerts")
	return nil
}
