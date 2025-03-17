package tasks

import (
	"backend/alerts"
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Alert represents a structure for handling Alert data.
type Alert struct {
	AlertID    int      `json:"alertId"`
	AlertType  string   `json:"alertType"`
	Price      *float64 `json:"alertPrice,omitempty"` // Use pointers to handle nullable fields
	SecurityID *int     `json:"securityId,omitempty"` // Use pointers for nullable fields
	SetupID    *int     `json:"setupId,omitempty"`    // Field for setupId if alert type is 'setup'
	Ticker     *string  `json:"ticker,omitempty"`
	Active     bool     `json:"active"`
	AlgoID     *int     `json:"algoId,omitempty"`
}

// GetAlerts performs operations related to GetAlerts functionality.
func GetAlerts(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	rows, err := conn.DB.Query(context.Background(), `
		SELECT a.alertId, a.alertType, a.price, a.securityID, a.setupId, s.ticker, a.active
		FROM alerts a
		LEFT JOIN securities s ON a.securityID = s.securityID
		WHERE a.userId = $1`, userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []Alert
	for rows.Next() {
		var alert Alert
		err := rows.Scan(&alert.AlertID, &alert.AlertType, &alert.Price, &alert.SecurityID, &alert.SetupID, &alert.Ticker, &alert.Active)
		if err != nil {
			return nil, fmt.Errorf("error scanning alert: %v", err)
		}
		alerts = append(alerts, alert)
	}

	return alerts, nil
}

// GetAlertLogsResult represents a structure for handling GetAlertLogsResult data.
type GetAlertLogsResult struct {
	AlertLogID int     `json:"alertLogId"`
	AlertID    int     `json:"alertId"`
	Timestamp  int64   `json:"timestamp"`
	SecurityID int     `json:"securityId"`
	Ticker     *string `json:"ticker,omitempty"` // Ticker from the securities table
}

// GetAlertLogs performs operations related to GetAlertLogs functionality.
func GetAlertLogs(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	rows, err := conn.DB.Query(context.Background(), `
		SELECT al.alertLogId, al.alertId, al.timestamp, al.securityId, s.ticker
		FROM alertLogs al
		JOIN alerts a ON a.alertId = al.alertId 
		LEFT JOIN securities s ON al.securityID = s.securityID
		WHERE a.userId = $1
		ORDER BY al.timestamp DESC`, userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []GetAlertLogsResult
	for rows.Next() {
		var log GetAlertLogsResult
		var logTime time.Time
		err := rows.Scan(&log.AlertLogID, &log.AlertID, &logTime, &log.SecurityID, &log.Ticker)
		if err != nil {
			return nil, fmt.Errorf("error scanning alert log: %v", err)
		}
		log.Timestamp = logTime.Unix() * 1000
		logs = append(logs, log)
	}
	return logs, nil
}

// NewAlertArgs represents a structure for handling NewAlertArgs data.
type NewAlertArgs struct {
	AlertType  string   `json:"alertType"`
	Price      *float64 `json:"price,omitempty"` // Using pointers to handle nullable fields
	SecurityID *int     `json:"securityId,omitempty"`
	SetupID    *int     `json:"setupId,omitempty"`
	Ticker     *string  `json:"ticker,omitempty"`
	AlgoID     *int     `json:"algoId,omitempty"`
}

// NewAlert performs operations related to NewAlert functionality.
func NewAlert(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args NewAlertArgs
	var err error
	err = json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	var alertID int
	var insertQuery string
	var direction *bool = nil

	if args.AlertType == "price" {
		if args.Price == nil || args.SecurityID == nil {
			return nil, fmt.Errorf("price and securityId are required for 'price' type alerts")
		}

		lastTrade, err := utils.GetLastTrade(conn.Polygon, *args.Ticker)
		if err != nil {
			return nil, fmt.Errorf("error fetching last trade: %v", err)
		}
		currentPrice := lastTrade.Price
		directionValue := *args.Price > currentPrice
		direction = &directionValue

		insertQuery = `
			INSERT INTO alerts (userId, alertType, price, securityID, active, direction) 
			VALUES ($1, $2, $3, $4, true, $5) RETURNING alertId`
		err = conn.DB.QueryRow(context.Background(), insertQuery, userId, args.AlertType, *args.Price, *args.SecurityID, direction).Scan(&alertID)
		if err != nil {
			return nil, fmt.Errorf("failed to insert price alert: %w", err)
		}

	} else if args.AlertType == "setup" {
		if args.SetupID == nil {
			return nil, fmt.Errorf("setupId is required for 'setup' type alerts")
		}
		insertQuery = `
			INSERT INTO alerts (userId, alertType, setupId, active) 
			VALUES ($1, $2, $3, true) RETURNING alertId`
		err = conn.DB.QueryRow(context.Background(), insertQuery, userId, args.AlertType, *args.SetupID).Scan(&alertID)

	} else if args.AlertType == "algo" {
		if args.AlgoID == nil {
			return nil, fmt.Errorf("algoId is required for 'algo' type alerts")
		}
		insertQuery = `
			INSERT INTO alerts (userId, alertType, algoId, active) 
			VALUES ($1, $2, $3, true) RETURNING alertId`
		err = conn.DB.QueryRow(context.Background(), insertQuery, userId, args.AlertType, *args.AlgoID).Scan(&alertID)
	} else {
		return nil, fmt.Errorf("invalid alertType: %s", args.AlertType)
	}
	if err != nil {
		return nil, fmt.Errorf("error creating new alert: %v", err)
	}
	newAlert := Alert{
		AlertID:    alertID,
		AlertType:  args.AlertType,
		Price:      args.Price,      // If setup type, price will be null
		SecurityID: args.SecurityID, // If setup type, securityId will be null
		SetupID:    args.SetupID,    // If price type, setupId will be null
		AlgoID:     args.AlgoID,     // If algo type, algoId will be null
		Active:     true,            // Set to true by default
	}
	// Convert tasks.Alert to alerts.Alert
	alertToAdd := alerts.Alert{
		AlertID:    newAlert.AlertID,
		AlertType:  newAlert.AlertType,
		Price:      newAlert.Price,
		SecurityID: newAlert.SecurityID,
		SetupID:    newAlert.SetupID,
		AlgoID:     newAlert.AlgoID,
		Direction:  direction,
	}
	alerts.AddAlert(conn, alertToAdd)

	return newAlert, nil
}

// DeleteAlertArgs represents a structure for handling DeleteAlertArgs data.
type DeleteAlertArgs struct {
	AlertID int `json:"alertId"`
}

// DeleteAlert performs operations related to DeleteAlert functionality.
func DeleteAlert(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args DeleteAlertArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	cmdTag, err := conn.DB.Exec(context.Background(), `
		DELETE FROM alerts WHERE alertId = $1 AND userId = $2`, args.AlertID, userId)
	if cmdTag.RowsAffected() == 0 {
		return nil, fmt.Errorf("alert not found or permission denied")
	}
	alerts.RemoveAlert(args.AlertID)

	return nil, err
}

/*type SetAlertArgs struct {
	AlertID    int     `json:"alertId"`
	AlertType  string  `json:"alertType"`
	Price      *float64 `json:"price,omitempty"`
	SecurityID *int     `json:"securityId,omitempty"`
	SetupID    *int     `json:"setupId,omitempty"`
}
// SetAlert performs operations related to SetAlert functionality.
func SetAlert(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args SetAlertArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	// Update the alert based on type
	if args.AlertType == "price" {
		if args.Price == nil || args.SecurityID == nil {
			return nil, fmt.Errorf("price and securityId are required for 'price' type alerts")
		}
		_, err = conn.DB.Exec(context.Background(), `
			UPDATE alerts
			SET alertType = $1, price = $2, securityID = $3, setupId = NULL
			WHERE alertId = $4 AND userId = $5`, args.AlertType, *args.Price, *args.SecurityID, args.AlertID, userId)
	} else if args.AlertType == "setup" {
		if args.SetupID == nil {
			return nil, fmt.Errorf("setupId is required for 'setup' type alerts")
		}
		_, err = conn.DB.Exec(context.Background(), `
			UPDATE alerts
			SET alertType = $1, setupId = $2, price = NULL, securityID = NULL
			WHERE alertId = $3 AND userId = $4`, args.AlertType, *args.SetupID, args.AlertID, userId)
	} else {
		return nil, fmt.Errorf("invalid alertType: %s", args.AlertType)
	}

	if err != nil {
		return nil, fmt.Errorf("error updating alert: %v", err)
	}

	return nil, nil
}
*/
