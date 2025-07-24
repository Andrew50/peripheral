package alerts

import (
	"backend/internal/app/limits"
	"backend/internal/data"
	"backend/internal/data/polygon"
	"backend/internal/services/alerts"
	"backend/internal/services/socket"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
)

/*
   ────────────────────────────────────────────────────────────────────────────────
   Data Models
   ────────────────────────────────────────────────────────────────────────────────
*/

// Alert mirrors the alerts table after the schema change.
// All alerts are "price" alerts, each with a single optional trigger timestamp.
type Alert struct {
	AlertID            int      `json:"alertId"`
	AlertType          string   `json:"alertType"`            // Always "price"
	Price              *float64 `json:"alertPrice,omitempty"` // Pointer -> nullable
	SecurityID         *int     `json:"securityId,omitempty"` // Pointer -> nullable
	Ticker             *string  `json:"ticker,omitempty"`
	Active             bool     `json:"active"`
	Direction          *bool    `json:"direction,omitempty"`          // true = above, false = below
	TriggeredTimestamp *int64   `json:"triggeredTimestamp,omitempty"` // ms since epoch, nil until fired
}

// GetAlertLogsResult now derives directly from the alerts table.  When an alert
// fires, its triggeredTimestamp is set; that single record substitutes for the
// old alertLogs table.
type GetAlertLogsResult struct {
	AlertLogID int     `json:"alertLogId"` // identical to alertId (kept to preserve signature)
	AlertID    int     `json:"alertId"`
	Timestamp  int64   `json:"timestamp"` // ms since epoch
	SecurityID int     `json:"securityId"`
	Ticker     *string `json:"ticker,omitempty"`
}

/*
   ────────────────────────────────────────────────────────────────────────────────
   Fetch all current alerts
   ────────────────────────────────────────────────────────────────────────────────
*/

func GetAlerts(conn *data.Conn, userID int, _ json.RawMessage) (interface{}, error) {
	rows, err := conn.DB.Query(context.Background(), `
		SELECT a.alertId,
		       'price' AS alertType,
		       a.price,
		       a.securityId,
		       s.ticker,
		       a.active,
		       a.direction,
		       a.triggeredTimestamp
		FROM alerts a
		LEFT JOIN securities s USING (securityId)
		WHERE a.userId = $1
		ORDER BY a.alertId`, userID)
	if err != nil {
		return nil, fmt.Errorf("querying alerts: %w", err)
	}
	defer rows.Close()

	var results []Alert
	for rows.Next() {
		var r Alert
		var triggered sql.NullTime
		if err := rows.Scan(&r.AlertID, &r.AlertType, &r.Price, &r.SecurityID,
			&r.Ticker, &r.Active, &r.Direction, &triggered); err != nil {
			return nil, fmt.Errorf("scanning alert: %w", err)
		}
		if triggered.Valid {
			ms := triggered.Time.UnixMilli()
			r.TriggeredTimestamp = &ms
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

/*
   ────────────────────────────────────────────────────────────────────────────────
   "Logs" = alerts that have a non-NULL triggeredTimestamp
   ────────────────────────────────────────────────────────────────────────────────
*/

type GetAlertLogsArgs struct {
	AlertType string `json:"alertType,omitempty"` // "price", "strategy", or "all"
}

func GetAlertLogs(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetAlertLogsArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		// Default to "all" if no args provided or parsing fails
		args.AlertType = "all"
	}

	// Default to "all" if no alert type specified
	if args.AlertType == "" {
		args.AlertType = "all"
	}

	// Use the new centralized logging system to get alert logs
	alertLogs, err := alerts.GetAlertLogs(conn, userID, args.AlertType)
	if err != nil {
		return nil, fmt.Errorf("querying alert logs: %w", err)
	}

	var logs []GetAlertLogsResult
	for _, log := range alertLogs {
		// Extract ticker and securityId from the payload
		var ticker *string
		var securityID int

		if tickerVal, exists := log.Payload["ticker"]; exists {
			if tickerStr, ok := tickerVal.(string); ok {
				ticker = &tickerStr
			}
		}

		if securityIDVal, exists := log.Payload["securityId"]; exists {
			switch v := securityIDVal.(type) {
			case int:
				securityID = v
			case float64:
				securityID = int(v)
			}
		}

		result := GetAlertLogsResult{
			AlertLogID: log.LogID,
			AlertID:    log.RelatedID, // For price alerts, relatedID is the alertID; for strategy alerts, it's the strategyID
			Timestamp:  log.Timestamp.UnixMilli(),
			SecurityID: securityID,
			Ticker:     ticker,
		}
		logs = append(logs, result)
	}

	return logs, nil
}

/*
   ────────────────────────────────────────────────────────────────────────────────
   New Alert
   ────────────────────────────────────────────────────────────────────────────────
*/

type NewAlertArgs struct {
	// AlertType kept for backward compatibility but ignored (always "price").
	AlertType  string   `json:"alertType,omitempty"`
	Price      *float64 `json:"price,omitempty"`
	SecurityID *int     `json:"securityId,omitempty"`
	Ticker     *string  `json:"ticker,omitempty"`
}

func AgentNewAlert(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	res, err := NewAlert(conn, userID, rawArgs)
	if err != nil {
		return nil, err
	}
	go func() {
		newAlert := res.(Alert)

		alertData := map[string]interface{}{
			"alertId":   newAlert.AlertID,
			"alertType": "price",
			"active":    newAlert.Active,
		}
		// Safely add pointer values, handling nil cases
		if newAlert.Price != nil {
			alertData["alertPrice"] = *newAlert.Price
		}
		if newAlert.SecurityID != nil {
			alertData["securityId"] = *newAlert.SecurityID
		}
		if newAlert.Ticker != nil {
			alertData["ticker"] = *newAlert.Ticker
		}
		if newAlert.Direction != nil {
			alertData["direction"] = *newAlert.Direction
		}

		socket.SendAlertUpdate(userID, "add", alertData)
	}()

	return res, nil
}
func NewAlert(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args NewAlertArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}
	if args.Price == nil || args.SecurityID == nil || args.Ticker == nil {
		return nil, fmt.Errorf("price, securityId and ticker are required")
	}

	// Check if user can create more alerts
	allowed, remaining, err := limits.CheckUsageAllowed(conn, userID, limits.UsageTypeAlert, 0)
	if err != nil {
		return nil, fmt.Errorf("checking alert limits: %w", err)
	}
	if !allowed {
		return nil, fmt.Errorf("alert limit reached - you have %d alerts remaining", remaining)
	}

	// Determine direction relative to the last trade
	lastTrade, err := polygon.GetLastTrade(conn.Polygon, *args.Ticker, true)
	if err != nil {
		return nil, fmt.Errorf("fetching last trade: %w", err)
	}
	dir := *args.Price > lastTrade.Price // true = wait for price to rise up to alert

	var alertID int
	if err := conn.DB.QueryRow(context.Background(), `
		INSERT INTO alerts (userId, active, price, direction, securityId)
		VALUES ($1, true, $2, $3, $4)
		RETURNING alertId`,
		userID, *args.Price, dir, *args.SecurityID).Scan(&alertID); err != nil {
		return nil, fmt.Errorf("inserting alert: %w", err)
	}

	// Increment the active alerts counter
	if err := limits.RecordUsage(conn, userID, limits.UsageTypeAlert, 1, map[string]interface{}{
		"alertId": alertID,
		"ticker":  *args.Ticker,
		"price":   *args.Price,
	}); err != nil {
		// If we can't record usage, we should rollback the alert creation
		if _, rollbackErr := conn.DB.Exec(context.Background(), `DELETE FROM alerts WHERE alertId = $1`, alertID); rollbackErr != nil {
			log.Printf("Warning: failed to rollback alert creation: %v", rollbackErr)
		}
		return nil, fmt.Errorf("recording alert usage: %w", err)
	}

	newAlert := Alert{
		AlertID:    alertID,
		Price:      args.Price,
		SecurityID: args.SecurityID,
		Ticker:     args.Ticker,
		Active:     true,
		Direction:  &dir,
	}
	// Keep in-memory scheduler/store up-to-date
	alerts.AddPriceAlert(conn, alerts.PriceAlert{
		AlertID:    newAlert.AlertID,
		UserID:     userID,
		Price:      newAlert.Price,
		SecurityID: newAlert.SecurityID,
		Direction:  newAlert.Direction,
		Ticker:     newAlert.Ticker,
	})
	return newAlert, nil
}

/*
   ────────────────────────────────────────────────────────────────────────────────
   Update Alert
   ────────────────────────────────────────────────────────────────────────────────
*/

type UpdateAlertArgs struct {
	AlertID int      `json:"alertId"`
	Price   *float64 `json:"price,omitempty"`
}

func AgentUpdateAlert(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	res, err := UpdateAlert(conn, userID, rawArgs)
	if err != nil {
		return nil, err
	}
	go func() {
		updatedAlert := res.(Alert)

		alertData := map[string]interface{}{
			"alertId":   updatedAlert.AlertID,
			"alertType": "price",
			"active":    updatedAlert.Active,
		}
		// Safely add pointer values, handling nil cases
		if updatedAlert.Price != nil {
			alertData["alertPrice"] = *updatedAlert.Price
		}
		if updatedAlert.SecurityID != nil {
			alertData["securityId"] = *updatedAlert.SecurityID
		}
		if updatedAlert.Ticker != nil {
			alertData["ticker"] = *updatedAlert.Ticker
		}
		if updatedAlert.Direction != nil {
			alertData["direction"] = *updatedAlert.Direction
		}

		socket.SendAlertUpdate(userID, "update", alertData)
	}()

	return res, nil
}

func UpdateAlert(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args UpdateAlertArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}
	if args.Price == nil {
		return nil, fmt.Errorf("price is required")
	}

	// First, get the current alert to verify ownership and get the ticker/securityId
	var currentAlert Alert
	var ticker string
	err := conn.DB.QueryRow(context.Background(), `
		SELECT a.alertId, a.price, a.direction, a.securityId, a.active, s.ticker
		FROM alerts a
		LEFT JOIN securities s USING (securityId)
		WHERE a.alertId = $1 AND a.userId = $2`,
		args.AlertID, userID).Scan(
		&currentAlert.AlertID,
		&currentAlert.Price,
		&currentAlert.Direction,
		&currentAlert.SecurityID,
		&currentAlert.Active,
		&ticker)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("alert not found or permission denied")
		}
		return nil, fmt.Errorf("fetching alert: %w", err)
	}

	// Determine new direction relative to the last trade
	lastTrade, err := polygon.GetLastTrade(conn.Polygon, ticker, true)
	if err != nil {
		return nil, fmt.Errorf("fetching last trade: %w", err)
	}
	newDir := *args.Price > lastTrade.Price // true = wait for price to rise up to alert

	// Update the alert in the database
	_, err = conn.DB.Exec(context.Background(), `
		UPDATE alerts 
		SET price = $1, direction = $2
		WHERE alertId = $3 AND userId = $4`,
		*args.Price, newDir, args.AlertID, userID)
	if err != nil {
		return nil, fmt.Errorf("updating alert: %w", err)
	}

	// Create the updated alert object to return
	updatedAlert := Alert{
		AlertID:    currentAlert.AlertID,
		Price:      args.Price,
		SecurityID: currentAlert.SecurityID,
		Ticker:     &ticker,
		Active:     currentAlert.Active,
		Direction:  &newDir,
	}

	// Update the in-memory scheduler/store
	alerts.AddPriceAlert(conn, alerts.PriceAlert{
		AlertID:    updatedAlert.AlertID,
		UserID:     userID,
		Price:      updatedAlert.Price,
		SecurityID: updatedAlert.SecurityID,
		Direction:  updatedAlert.Direction,
		Ticker:     updatedAlert.Ticker,
	})

	return updatedAlert, nil
}

/*
   ────────────────────────────────────────────────────────────────────────────────
   Delete Alert
   ────────────────────────────────────────────────────────────────────────────────
*/

type DeleteAlertArgs struct {
	AlertID int `json:"alertId"`
}

func AgentDeleteAlert(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	res, err := DeleteAlert(conn, userID, rawArgs)
	if err != nil {
		return nil, err
	}
	/*go func() {
		alertData := map[string]interface{}{
			"alertId": args.AlertID,
		}
		socket.SendAlertUpdate(userID, "remove", alertData)
	}()*/
	return res, nil
}
func DeleteAlert(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args DeleteAlertArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}

	// Check if the alert was active before deleting
	var wasActive bool
	err := conn.DB.QueryRow(context.Background(),
		`SELECT active FROM alerts WHERE alertId = $1 AND userId = $2`,
		args.AlertID, userID).Scan(&wasActive)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("alert not found or permission denied")
		}
		return nil, fmt.Errorf("checking alert status: %w", err)
	}

	tag, err := conn.DB.Exec(context.Background(),
		`DELETE FROM alerts WHERE alertId = $1 AND userId = $2`,
		args.AlertID, userID)
	if err != nil {
		return nil, fmt.Errorf("deleting alert: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("alert not found or permission denied")
	}

	// Only decrement the counter if the alert was active
	if wasActive {
		if err := limits.DecrementActiveAlerts(conn, userID, 1); err != nil {
			// Log the error but don't fail the deletion since the alert is already removed
			fmt.Printf("Warning: failed to decrement active alerts counter for user %d: %v\n", userID, err)
		}
	}

	// Remove from memory without decrementing counter (already handled above if needed)
	alerts.RemovePriceAlertFromMemory(args.AlertID)

	return nil, nil
}
