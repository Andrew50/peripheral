package alerts

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// AlertLogEntry represents the structure of an alert log entry
type AlertLogEntry struct {
	LogID     int                    `json:"logId"`
	UserID    int                    `json:"userId"`
	AlertType string                 `json:"alertType"`
	RelatedID int                    `json:"relatedId"`
	Timestamp time.Time              `json:"timestamp"`
	Message   string                 `json:"message"`
	Payload   map[string]interface{} `json:"payload"`
}

// LogAlert logs an alert event to the unified alert_logs table
func LogAlert(conn *data.Conn, userID int, alertType string, relatedID int, message string, payload map[string]interface{}) error {
	if alertType != "price" && alertType != "strategy" {
		return fmt.Errorf("invalid alert type: %s, must be 'price' or 'strategy'", alertType)
	}

	// Convert payload to JSON
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Determine ticker value for column
	tickerValue := ""
	if t, ok := payload["ticker"].(string); ok {
		tickerValue = t
	}

	// Insert the log entry into the database
	query := `
		INSERT INTO alert_logs (user_id, alert_type, related_id, ticker, message, payload)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err = data.ExecWithRetry(context.Background(), conn.DB, query, userID, alertType, relatedID, tickerValue, message, string(payloadJSON))
	if err != nil {
		return fmt.Errorf("failed to log alert: %w", err)
	}

	return nil
}

// GetAlertLogs retrieves alert logs for a user from the unified alert_logs table
func GetAlertLogs(conn *data.Conn, userID int, alertType string) ([]AlertLogEntry, error) {
	var query string
	var args []interface{}

	if alertType != "" && alertType != "all" {
		// Filter by alert type if specified and not "all"
		query = `
			SELECT log_id, user_id, alert_type, related_id, timestamp, message, payload
			FROM alert_logs
			WHERE user_id = $1 AND alert_type = $2
			ORDER BY timestamp DESC
		`
		args = []interface{}{userID, alertType}
	} else {
		// Get all alert types if not specified or "all" is specified
		query = `
			SELECT log_id, user_id, alert_type, related_id, timestamp, message, payload
			FROM alert_logs
			WHERE user_id = $1
			ORDER BY timestamp DESC
		`
		args = []interface{}{userID}
	}

	rows, err := conn.DB.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query alert logs: %w", err)
	}
	defer rows.Close()

	var logs []AlertLogEntry
	for rows.Next() {
		var log AlertLogEntry
		var payloadJSON string

		err := rows.Scan(&log.LogID, &log.UserID, &log.AlertType, &log.RelatedID, &log.Timestamp, &log.Message, &payloadJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to scan alert log row: %w", err)
		}

		// Parse the JSON payload
		if err := json.Unmarshal([]byte(payloadJSON), &log.Payload); err != nil {
			// If JSON parsing fails, create an empty payload
			log.Payload = make(map[string]interface{})
		}

		logs = append(logs, log)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating alert log rows: %w", err)
	}

	return logs, nil
}

// LogPriceAlert is a convenience function for logging price alerts
func LogPriceAlert(conn *data.Conn, userID, alertID int, ticker string, securityID int, message string) error {
	payload := map[string]interface{}{
		"ticker":     ticker,
		"securityId": securityID,
	}
	return LogAlert(conn, userID, "price", alertID, message, payload)
}

// LogStrategyAlert is a convenience function for logging strategy alerts
func LogStrategyAlert(conn *data.Conn, userID, strategyID int, strategyName string, message string, additionalData map[string]interface{}) error {
	payload := map[string]interface{}{
		"strategyName": strategyName,
	}

	// Merge additional data into payload
	for key, value := range additionalData {
		payload[key] = value
	}

	return LogAlert(conn, userID, "strategy", strategyID, message, payload)
}
