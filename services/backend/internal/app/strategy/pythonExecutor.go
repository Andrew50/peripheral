package strategy

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// PythonExecutionJob represents a job for the Python worker
type PythonExecutionJob struct {
	ExecutionID    string                 `json:"execution_id"`
	PythonCode     string                 `json:"python_code"`
	InputData      map[string]interface{} `json:"input_data"`
	TimeoutSeconds int                    `json:"timeout_seconds"`
	MemoryLimitMB  int                    `json:"memory_limit_mb"`
	Libraries      []string               `json:"libraries"`
	CreatedAt      string                 `json:"created_at"`
}

// PythonExecutionResult represents the result from Python execution
type PythonExecutionResult struct {
	ExecutionID     string                 `json:"execution_id"`
	Status          string                 `json:"status"`
	OutputData      map[string]interface{} `json:"output_data,omitempty"`
	ErrorMessage    string                 `json:"error_message,omitempty"`
	ExecutionTimeMs int                    `json:"execution_time_ms,omitempty"`
	WorkerID        string                 `json:"worker_id,omitempty"`
	CompletedAt     string                 `json:"completed_at,omitempty"`
}

// PythonExecutor handles executing Python strategies via Redis queue
type PythonExecutor struct {
	conn *data.Conn
}

// NewPythonExecutor creates a new Python executor
func NewPythonExecutor(conn *data.Conn) *PythonExecutor {
	return &PythonExecutor{conn: conn}
}

// ExecuteStrategy executes a Python strategy for a specific symbol and date
func (pe *PythonExecutor) ExecuteStrategy(ctx context.Context, pythonCode string, symbol string, date time.Time) (bool, map[string]interface{}, error) {
	// Create execution job
	executionID := fmt.Sprintf("backtest_%s_%s_%d", symbol, date.Format("20060102"), time.Now().UnixNano())

	inputData := map[string]interface{}{
		"symbol":    symbol,
		"date":      date.Format("2006-01-02"),
		"timestamp": date.UnixMilli(),
	}

	// Wrap the strategy code to execute the classify_symbol function
	wrappedCode := fmt.Sprintf(`
# Original strategy code
%s

# Execute the strategy for the given symbol
try:
    # Get symbol from input data
    symbol = input_data.get('symbol', 'AAPL')
    
    # Execute the classify_symbol function
    try:
        result = classify_symbol(symbol)
        save_result('classification', result)
    except NameError:
        save_result('classification', False)
        save_result('error', 'classify_symbol function not found in strategy code')
        
except Exception as e:
    save_result('classification', False)
    save_result('error', str(e))
`, pythonCode)

	job := PythonExecutionJob{
		ExecutionID:    executionID,
		PythonCode:     wrappedCode,
		InputData:      inputData,
		TimeoutSeconds: 30, // 30 seconds timeout for individual executions
		MemoryLimitMB:  128,
		Libraries:      []string{"numpy", "pandas"},
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
	}

	// Submit job to Redis queue
	err := pe.submitJob(ctx, job)
	if err != nil {
		return false, nil, fmt.Errorf("failed to submit Python job: %w", err)
	}

	// Wait for result
	result, err := pe.waitForResult(ctx, executionID, 60*time.Second)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get Python execution result: %w", err)
	}

	if result.Status != "completed" {
		log.Printf("Python execution failed for %s: %s", symbol, result.ErrorMessage)
		return false, nil, nil // Return no error but false classification
	}

	// Extract classification result
	classification := false
	strategyResults := make(map[string]interface{})

	if result.OutputData != nil {
		// Look for classification result
		if classificationVal, exists := result.OutputData["classification"]; exists {
			if classificationBool, ok := classificationVal.(bool); ok {
				classification = classificationBool
			}
		}

		// Copy all strategy results except internal ones
		for key, value := range result.OutputData {
			if key != "success" && key != "_execution_stats" {
				strategyResults[key] = value
			}
		}
	}

	return classification, strategyResults, nil
}

// submitJob submits a job to the Python execution queue
func (pe *PythonExecutor) submitJob(ctx context.Context, job PythonExecutionJob) error {
	jobJSON, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	// Push job to Redis queue
	err = pe.conn.Cache.RPush(ctx, "python_execution_queue", string(jobJSON)).Err()
	if err != nil {
		return fmt.Errorf("failed to push job to queue: %w", err)
	}

	return nil
}

// waitForResult waits for the execution result via Redis pubsub
func (pe *PythonExecutor) waitForResult(ctx context.Context, executionID string, timeout time.Duration) (*PythonExecutionResult, error) {
	// Subscribe to execution updates
	pubsub := pe.conn.Cache.Subscribe(ctx, "python_execution_updates")
	defer pubsub.Close()

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ch := pubsub.Channel()

	for {
		select {
		case <-timeoutCtx.Done():
			return nil, fmt.Errorf("timeout waiting for execution result")
		case msg := <-ch:
			if msg == nil {
				continue
			}

			var result PythonExecutionResult
			err := json.Unmarshal([]byte(msg.Payload), &result)
			if err != nil {
				log.Printf("Failed to unmarshal execution update: %v", err)
				continue
			}

			if result.ExecutionID == executionID {
				if result.Status == "completed" || result.Status == "failed" || result.Status == "timeout" {
					return &result, nil
				}
			}
		}
	}
}

// ExecuteStrategyBatch executes a strategy for multiple symbols efficiently
func (pe *PythonExecutor) ExecuteStrategyBatch(ctx context.Context, pythonCode string, symbols []string, date time.Time) (map[string]bool, error) {
	results := make(map[string]bool)

	// For now, execute sequentially. Could be optimized to run in parallel batches
	for _, symbol := range symbols {
		classification, _, err := pe.ExecuteStrategy(ctx, pythonCode, symbol, date)
		if err != nil {
			log.Printf("Error executing strategy for %s: %v", symbol, err)
			continue
		}
		results[symbol] = classification
	}

	return results, nil
}
