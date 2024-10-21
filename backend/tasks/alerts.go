package tasks

import (
    "backend/utils"
    "encoding/json"
    "time"
    "fmt"
    "context"
)

type GetAlertsResult struct {
	AlertId    int      `json:"alertId"`
	AlertType  string   `json:"alertType"`
	Price      *float64 `json:"price,omitempty"`      // Use pointers to handle nullable fields
	SecurityId *int     `json:"securityId,omitempty"` // Use pointers for nullable fields
	SetupId    *int     `json:"setupId,omitempty"`    // Field for setupId if alert type is 'setup'
}

func GetAlerts(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	rows, err := conn.DB.Query(context.Background(), `
		SELECT alertId, alertType, price, securityID, setupId 
		FROM alerts 
		WHERE userId = $1`, userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []GetAlertsResult
	for rows.Next() {
		var alert GetAlertsResult
		err := rows.Scan(&alert.AlertId, &alert.AlertType, &alert.Price, &alert.SecurityId, &alert.SetupId)
		if err != nil {
			return nil, fmt.Errorf("error scanning alert: %v", err)
		}
		alerts = append(alerts, alert)
	}

	return alerts, nil
}

type GetAlertLogsResult struct {
	AlertLogId  int    `json:"alertLogId"`
	AlertId     int    `json:"alertId"`
	Timestamp   int64  `json:"timestamp"`
	SecurityId  int    `json:"securityId"`
}

func GetAlertLogs(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	rows, err := conn.DB.Query(context.Background(), `
		SELECT al.alertLogId, al.alertId, al.timestamp, al.securityId 
		FROM alertLogs al
		JOIN alerts a ON a.alertId = al.alertId 
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
		err := rows.Scan(&log.AlertLogId, &log.AlertId, &logTime, &log.SecurityId)
		if err != nil {
			return nil, fmt.Errorf("error scanning alert log: %v", err)
		}
		log.Timestamp = logTime.Unix() * 1000
		logs = append(logs, log)
	}
	return logs, nil
}

type NewAlertArgs struct {
	AlertType  string  `json:"alertType"`
	Price      *float64 `json:"price,omitempty"` // Using pointers to handle nullable fields
	SecurityId *int     `json:"securityId,omitempty"`
	SetupId    *int     `json:"setupId,omitempty"`
}

func NewAlert(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args NewAlertArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}
	if args.AlertType == "price" {
		if args.Price == nil || args.SecurityId == nil {
			return nil, fmt.Errorf("price and securityId are required for 'price' type alerts")
		}
		_, err = conn.DB.Exec(context.Background(), `
			INSERT INTO alerts (userId, alertType, price, securityID) 
			VALUES ($1, $2, $3, $4)`, userId, args.AlertType, *args.Price, *args.SecurityId)
	} else if args.AlertType == "setup" {
		if args.SetupId == nil {
			return nil, fmt.Errorf("setupId is required for 'setup' type alerts")
		}
		_, err = conn.DB.Exec(context.Background(), `
			INSERT INTO alerts (userId, alertType, setupId) 
			VALUES ($1, $2, $3)`, userId, args.AlertType, *args.SetupId)
	} else {
		return nil, fmt.Errorf("invalid alertType: %s", args.AlertType)
	}

	if err != nil {
		return nil, fmt.Errorf("error creating new alert: %v", err)
	}

	return nil, nil
}


type DeleteAlertArgs struct {
	AlertId int `json:"alertId"`
}

func DeleteAlert(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args DeleteAlertArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	cmdTag, err := conn.DB.Exec(context.Background(), `
		DELETE FROM alerts WHERE alertId = $1 AND userId = $2`, args.AlertId, userId)
	if cmdTag.RowsAffected() == 0 {
		return nil, fmt.Errorf("alert not found or permission denied")
	}

	return nil, err
}
type SetAlertArgs struct {
	AlertId    int     `json:"alertId"`
	AlertType  string  `json:"alertType"`
	Price      *float64 `json:"price,omitempty"`
	SecurityId *int     `json:"securityId,omitempty"`
	SetupId    *int     `json:"setupId,omitempty"`
}

func SetAlert(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args SetAlertArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	// Update the alert based on type
	if args.AlertType == "price" {
		if args.Price == nil || args.SecurityId == nil {
			return nil, fmt.Errorf("price and securityId are required for 'price' type alerts")
		}
		_, err = conn.DB.Exec(context.Background(), `
			UPDATE alerts 
			SET alertType = $1, price = $2, securityID = $3, setupId = NULL 
			WHERE alertId = $4 AND userId = $5`, args.AlertType, *args.Price, *args.SecurityId, args.AlertId, userId)
	} else if args.AlertType == "setup" {
		if args.SetupId == nil {
			return nil, fmt.Errorf("setupId is required for 'setup' type alerts")
		}
		_, err = conn.DB.Exec(context.Background(), `
			UPDATE alerts 
			SET alertType = $1, setupId = $2, price = NULL, securityID = NULL 
			WHERE alertId = $3 AND userId = $4`, args.AlertType, *args.SetupId, args.AlertId, userId)
	} else {
		return nil, fmt.Errorf("invalid alertType: %s", args.AlertType)
	}

	if err != nil {
		return nil, fmt.Errorf("error updating alert: %v", err)
	}

	return nil, nil
}

