package alerts

import (
	"backend/internal/data"
	"backend/internal/data/polygon"
	"backend/internal/services/alerts"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
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

func GetAlertLogs(conn *data.Conn, userID int, _ json.RawMessage) (interface{}, error) {
	rows, err := conn.DB.Query(context.Background(), `
		SELECT a.alertId,
		       a.alertId AS alertLogId,
		       a.triggeredTimestamp,
		       a.securityId,
		       s.ticker
		FROM alerts a
		LEFT JOIN securities s USING (securityId)
		WHERE a.userId = $1
		  AND a.triggeredTimestamp IS NOT NULL
		ORDER BY a.triggeredTimestamp DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("querying fired alerts: %w", err)
	}
	defer rows.Close()

	var logs []GetAlertLogsResult
	for rows.Next() {
		var (
			l     GetAlertLogsResult
			fired time.Time
		)
		if err := rows.Scan(&l.AlertID, &l.AlertLogID, &fired, &l.SecurityID, &l.Ticker); err != nil {
			return nil, fmt.Errorf("scanning alert log: %w", err)
		}
		l.Timestamp = fired.UnixMilli()
		logs = append(logs, l)
	}
	return logs, rows.Err()
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

func NewAlert(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args NewAlertArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}
	if args.Price == nil || args.SecurityID == nil || args.Ticker == nil {
		return nil, fmt.Errorf("price, securityId and ticker are required")
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

	newAlert := Alert{
		AlertID:    alertID,
		Price:      args.Price,
		SecurityID: args.SecurityID,
		Ticker:     args.Ticker,
		Active:     true,
		Direction:  &dir,
	}
	// Keep in-memory scheduler/store up-to-date
	alerts.AddAlert(conn, alerts.Alert{
		AlertID:    newAlert.AlertID,
		Price:      newAlert.Price,
		SecurityID: newAlert.SecurityID,
		Direction:  newAlert.Direction,
	})

	return newAlert, nil
}

/*
   ────────────────────────────────────────────────────────────────────────────────
   Delete Alert
   ────────────────────────────────────────────────────────────────────────────────
*/

type DeleteAlertArgs struct {
	AlertID int `json:"alertId"`
}

func DeleteAlert(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args DeleteAlertArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
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

	alerts.RemoveAlert(args.AlertID)
	return nil, nil
}
