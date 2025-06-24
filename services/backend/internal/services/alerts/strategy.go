package alerts

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// StrategyAlert represents an active strategy that should be monitored
type StrategyAlert struct {
	StrategyID int    `json:"strategyId"`
	UserID     int    `json:"userId"`
	Name       string `json:"name"`
}

// processStrategyAlert submits active strategy alerts to the worker queue
func processStrategyAlert(conn *data.Conn, alert Alert) error {
	// Get all active strategy alerts from the database
	strategies, err := getActiveStrategyAlerts(conn)
	if err != nil {
		log.Printf("Error getting active strategy alerts: %v", err)
		return err
	}

	// Submit each active strategy to the worker queue
	for _, strategy := range strategies {
		err := submitStrategyToWorkerQueue(conn, strategy)
		if err != nil {
			log.Printf("Error submitting strategy %d to worker queue: %v", strategy.StrategyID, err)
			// Continue with other strategies even if one fails
			continue
		}
		log.Printf("Successfully submitted strategy %d (%s) to worker queue", strategy.StrategyID, strategy.Name)
	}

	return nil
}

// getActiveStrategyAlerts queries the database for all strategies with isAlertActive = true
func getActiveStrategyAlerts(conn *data.Conn) ([]StrategyAlert, error) {
	query := `
		SELECT strategyId, userId, name 
		FROM strategies 
		WHERE isAlertActive = true 
		ORDER BY strategyId
	`

	rows, err := conn.DB.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("querying active strategy alerts: %w", err)
	}
	defer rows.Close()

	var strategies []StrategyAlert
	for rows.Next() {
		var strategy StrategyAlert
		err := rows.Scan(&strategy.StrategyID, &strategy.UserID, &strategy.Name)
		if err != nil {
			return nil, fmt.Errorf("scanning strategy alert row: %w", err)
		}
		strategies = append(strategies, strategy)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating strategy alert rows: %w", err)
	}

	return strategies, nil
}

// submitStrategyToWorkerQueue submits a strategy alert task to the Redis worker queue
func submitStrategyToWorkerQueue(conn *data.Conn, strategy StrategyAlert) error {
	// Generate unique task ID
	taskID := fmt.Sprintf("strategy_alert_%d_%d", strategy.StrategyID, time.Now().UnixNano())

	// Prepare strategy alert task payload
	task := map[string]interface{}{
		"task_id":   taskID,
		"task_type": "alert",
		"args": map[string]interface{}{
			"strategy_id": fmt.Sprintf("%d", strategy.StrategyID),
			"user_id":     fmt.Sprintf("%d", strategy.UserID),
		},
		"created_at": time.Now().UTC().Format(time.RFC3339),
	}

	// Convert task to JSON
	taskJSON, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("error marshaling task: %v", err)
	}

	// Submit task to Redis worker queue (fire and forget - no need to wait for results)
	err = conn.Cache.RPush(context.Background(), "strategy_queue", string(taskJSON)).Err()
	if err != nil {
		return fmt.Errorf("error submitting task to queue: %v", err)
	}

	return nil
}
