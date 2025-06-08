package strategy

import (
	"backend/internal/data"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PythonStrategy represents a Python-based trading strategy
type PythonStrategy struct {
	ID             int       `json:"id"`
	StrategyID     int       `json:"strategyId"`
	UserID         int       `json:"userId"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	PythonCode     string    `json:"pythonCode"`
	Libraries      []string  `json:"libraries"`
	DataPrepSQL    *string   `json:"dataPrepSql,omitempty"`
	ExecutionMode  string    `json:"executionMode"` // "python", "hybrid", "notebook"
	TimeoutSeconds int       `json:"timeoutSeconds"`
	MemoryLimitMB  int       `json:"memoryLimitMb"`
	CPULimitCores  float64   `json:"cpuLimitCores"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	Version        int       `json:"version"`
	IsActive       bool      `json:"isActive"`
}

// PythonExecution represents an execution instance of a Python strategy
type PythonExecution struct {
	ID                int                    `json:"id"`
	PythonStrategyID  int                    `json:"pythonStrategyId"`
	ExecutionID       uuid.UUID              `json:"executionId"`
	UserID            int                    `json:"userId"`
	Status            string                 `json:"status"` // "pending", "running", "completed", "failed", "timeout", "cancelled"
	StartedAt         *time.Time             `json:"startedAt,omitempty"`
	CompletedAt       *time.Time             `json:"completedAt,omitempty"`
	ExecutionTimeMS   *int                   `json:"executionTimeMs,omitempty"`
	MemoryUsedMB      *int                   `json:"memoryUsedMb,omitempty"`
	CPUUsedPercent    *float64               `json:"cpuUsedPercent,omitempty"`
	InputData         map[string]interface{} `json:"inputData,omitempty"`
	OutputData        map[string]interface{} `json:"outputData,omitempty"`
	ErrorMessage      *string                `json:"errorMessage,omitempty"`
	Logs              *string                `json:"logs,omitempty"`
	WorkerNode        *string                `json:"workerNode,omitempty"`
	CreatedAt         time.Time              `json:"createdAt"`
}

// PythonEnvironment represents a Python execution environment
type PythonEnvironment struct {
	ID                   int                    `json:"id"`
	Name                 string                 `json:"name"`
	Description          string                 `json:"description"`
	BaseImage            string                 `json:"baseImage"`
	PythonVersion        string                 `json:"pythonVersion"`
	Libraries            []string               `json:"libraries"`
	EnvironmentVariables map[string]interface{} `json:"environmentVariables"`
	IsDefault            bool                   `json:"isDefault"`
	IsActive             bool                   `json:"isActive"`
	CreatedAt            time.Time              `json:"createdAt"`
	UpdatedAt            time.Time              `json:"updatedAt"`
}

// CreatePythonStrategyArgs represents arguments for creating a Python strategy
type CreatePythonStrategyArgs struct {
	StrategyID     int      `json:"strategyId"`
	Name           string   `json:"name"`
	Description    string   `json:"description,omitempty"`
	PythonCode     string   `json:"pythonCode"`
	Libraries      []string `json:"libraries,omitempty"`
	DataPrepSQL    *string  `json:"dataPrepSql,omitempty"`
	ExecutionMode  string   `json:"executionMode,omitempty"`
	TimeoutSeconds int      `json:"timeoutSeconds,omitempty"`
	MemoryLimitMB  int      `json:"memoryLimitMb,omitempty"`
	CPULimitCores  float64  `json:"cpuLimitCores,omitempty"`
}

// ExecutePythonStrategyArgs represents arguments for executing a Python strategy
type ExecutePythonStrategyArgs struct {
	PythonStrategyID int                    `json:"pythonStrategyId,omitempty"`
	StrategyID       int                    `json:"strategyId,omitempty"` // Alternative to pythonStrategyId
	InputData        map[string]interface{} `json:"inputData,omitempty"`
	AsyncExecution   bool                   `json:"asyncExecution,omitempty"`
}

// GetPythonStrategiesArgs represents arguments for getting Python strategies
type GetPythonStrategiesArgs struct {
	UserID     int  `json:"userId,omitempty"`
	StrategyID int  `json:"strategyId,omitempty"`
	ActiveOnly bool `json:"activeOnly,omitempty"`
}

// GetPythonExecutionsArgs represents arguments for getting Python executions
type GetPythonExecutionsArgs struct {
	PythonStrategyID int    `json:"pythonStrategyId,omitempty"`
	ExecutionID      string `json:"executionId,omitempty"`
	Status           string `json:"status,omitempty"`
	Limit            int    `json:"limit,omitempty"`
	Offset           int    `json:"offset,omitempty"`
}

// CreatePythonStrategy creates a new Python strategy
func CreatePythonStrategy(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args CreatePythonStrategyArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	// Validate required fields
	if args.StrategyID == 0 {
		return nil, fmt.Errorf("strategyId is required")
	}
	if args.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if args.PythonCode == "" {
		return nil, fmt.Errorf("pythonCode is required")
	}

	// Set defaults
	if args.ExecutionMode == "" {
		args.ExecutionMode = "python"
	}
	if args.TimeoutSeconds == 0 {
		args.TimeoutSeconds = 300
	}
	if args.MemoryLimitMB == 0 {
		args.MemoryLimitMB = 512
	}
	if args.CPULimitCores == 0 {
		args.CPULimitCores = 1.0
	}
	if args.Libraries == nil {
		args.Libraries = []string{}
	}

	// Verify strategy exists and belongs to user
	var strategyExists bool
	err := conn.DB.QueryRow(context.Background(),
		"SELECT EXISTS(SELECT 1 FROM strategies WHERE strategyId = $1 AND userId = $2)",
		args.StrategyID, userID).Scan(&strategyExists)
	if err != nil {
		return nil, fmt.Errorf("error checking strategy existence: %v", err)
	}
	if !strategyExists {
		return nil, fmt.Errorf("strategy not found or access denied")
	}

	// Convert libraries to JSON
	librariesJSON, err := json.Marshal(args.Libraries)
	if err != nil {
		return nil, fmt.Errorf("error marshaling libraries: %v", err)
	}

	// Insert Python strategy
	var pythonStrategy PythonStrategy
	query := `
		INSERT INTO python_strategies (
			strategy_id, user_id, name, description, python_code, libraries, 
			data_prep_sql, execution_mode, timeout_seconds, memory_limit_mb, cpu_limit_cores
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, strategy_id, user_id, name, description, python_code, libraries, 
				  data_prep_sql, execution_mode, timeout_seconds, memory_limit_mb, 
				  cpu_limit_cores, created_at, updated_at, version, is_active`

	var librariesStr string
	err = conn.DB.QueryRow(context.Background(), query,
		args.StrategyID, userID, args.Name, args.Description, args.PythonCode,
		librariesJSON, args.DataPrepSQL, args.ExecutionMode,
		args.TimeoutSeconds, args.MemoryLimitMB, args.CPULimitCores,
	).Scan(
		&pythonStrategy.ID, &pythonStrategy.StrategyID, &pythonStrategy.UserID,
		&pythonStrategy.Name, &pythonStrategy.Description, &pythonStrategy.PythonCode,
		&librariesStr, &pythonStrategy.DataPrepSQL, &pythonStrategy.ExecutionMode,
		&pythonStrategy.TimeoutSeconds, &pythonStrategy.MemoryLimitMB,
		&pythonStrategy.CPULimitCores, &pythonStrategy.CreatedAt,
		&pythonStrategy.UpdatedAt, &pythonStrategy.Version, &pythonStrategy.IsActive,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating Python strategy: %v", err)
	}

	// Parse libraries JSON
	if err := json.Unmarshal([]byte(librariesStr), &pythonStrategy.Libraries); err != nil {
		return nil, fmt.Errorf("error parsing libraries: %v", err)
	}

	return pythonStrategy, nil
}

// GetPythonStrategies retrieves Python strategies
func GetPythonStrategies(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetPythonStrategiesArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	// Build query
	query := `
		SELECT ps.id, ps.strategy_id, ps.user_id, ps.name, ps.description, 
			   ps.python_code, ps.libraries, ps.data_prep_sql, ps.execution_mode,
			   ps.timeout_seconds, ps.memory_limit_mb, ps.cpu_limit_cores,
			   ps.created_at, ps.updated_at, ps.version, ps.is_active
		FROM python_strategies ps
		WHERE ps.user_id = $1`

	var queryArgs []interface{}
	queryArgs = append(queryArgs, userID)
	argIndex := 2

	if args.StrategyID != 0 {
		query += fmt.Sprintf(" AND ps.strategy_id = $%d", argIndex)
		queryArgs = append(queryArgs, args.StrategyID)
		argIndex++
	}

	if args.ActiveOnly {
		query += fmt.Sprintf(" AND ps.is_active = $%d", argIndex)
		queryArgs = append(queryArgs, true)
		argIndex++
	}

	query += " ORDER BY ps.created_at DESC"

	rows, err := conn.DB.Query(context.Background(), query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("error querying Python strategies: %v", err)
	}
	defer rows.Close()

	var strategies []PythonStrategy
	for rows.Next() {
		var strategy PythonStrategy
		var librariesStr string

		err := rows.Scan(
			&strategy.ID, &strategy.StrategyID, &strategy.UserID,
			&strategy.Name, &strategy.Description, &strategy.PythonCode,
			&librariesStr, &strategy.DataPrepSQL, &strategy.ExecutionMode,
			&strategy.TimeoutSeconds, &strategy.MemoryLimitMB,
			&strategy.CPULimitCores, &strategy.CreatedAt,
			&strategy.UpdatedAt, &strategy.Version, &strategy.IsActive,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning Python strategy: %v", err)
		}

		// Parse libraries JSON
		if err := json.Unmarshal([]byte(librariesStr), &strategy.Libraries); err != nil {
			return nil, fmt.Errorf("error parsing libraries: %v", err)
		}

		strategies = append(strategies, strategy)
	}

	return strategies, nil
}

// ExecutePythonStrategy executes a Python strategy
func ExecutePythonStrategy(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args ExecutePythonStrategyArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	var pythonStrategyID int
	if args.PythonStrategyID != 0 {
		pythonStrategyID = args.PythonStrategyID
	} else if args.StrategyID != 0 {
		// Find Python strategy by strategy ID
		err := conn.DB.QueryRow(context.Background(),
			"SELECT id FROM python_strategies WHERE strategy_id = $1 AND user_id = $2 AND is_active = true",
			args.StrategyID, userID).Scan(&pythonStrategyID)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, fmt.Errorf("no active Python strategy found for strategy ID %d", args.StrategyID)
			}
			return nil, fmt.Errorf("error finding Python strategy: %v", err)
		}
	} else {
		return nil, fmt.Errorf("either pythonStrategyId or strategyId is required")
	}

	// Create execution record
	executionID := uuid.New()
	inputDataJSON, err := json.Marshal(args.InputData)
	if err != nil {
		return nil, fmt.Errorf("error marshaling input data: %v", err)
	}

	var execution PythonExecution
	query := `
		INSERT INTO python_executions (
			python_strategy_id, execution_id, user_id, status, input_data
		)
		VALUES ($1, $2, $3, 'pending', $4)
		RETURNING id, python_strategy_id, execution_id, user_id, status, 
				  input_data, created_at`

	var inputDataStr string
	err = conn.DB.QueryRow(context.Background(), query,
		pythonStrategyID, executionID, userID, inputDataJSON,
	).Scan(
		&execution.ID, &execution.PythonStrategyID, &execution.ExecutionID,
		&execution.UserID, &execution.Status, &inputDataStr, &execution.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating execution record: %v", err)
	}

	// Parse input data
	if err := json.Unmarshal([]byte(inputDataStr), &execution.InputData); err != nil {
		return nil, fmt.Errorf("error parsing input data: %v", err)
	}

	if args.AsyncExecution {
		// TODO: Queue the execution for async processing
		// For now, return the execution record
		return execution, nil
	} else {
		// TODO: Execute synchronously
		// For now, just return the execution record
		return execution, nil
	}
}

// GetPythonExecutions retrieves Python strategy executions
func GetPythonExecutions(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetPythonExecutionsArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	// Set defaults
	if args.Limit == 0 {
		args.Limit = 50
	}

	// Build query
	query := `
		SELECT pe.id, pe.python_strategy_id, pe.execution_id, pe.user_id, pe.status,
			   pe.started_at, pe.completed_at, pe.execution_time_ms, pe.memory_used_mb,
			   pe.cpu_used_percent, pe.input_data, pe.output_data, pe.error_message,
			   pe.logs, pe.worker_node, pe.created_at
		FROM python_executions pe
		JOIN python_strategies ps ON pe.python_strategy_id = ps.id
		WHERE ps.user_id = $1`

	var queryArgs []interface{}
	queryArgs = append(queryArgs, userID)
	argIndex := 2

	if args.PythonStrategyID != 0 {
		query += fmt.Sprintf(" AND pe.python_strategy_id = $%d", argIndex)
		queryArgs = append(queryArgs, args.PythonStrategyID)
		argIndex++
	}

	if args.ExecutionID != "" {
		query += fmt.Sprintf(" AND pe.execution_id = $%d", argIndex)
		queryArgs = append(queryArgs, args.ExecutionID)
		argIndex++
	}

	if args.Status != "" {
		query += fmt.Sprintf(" AND pe.status = $%d", argIndex)
		queryArgs = append(queryArgs, args.Status)
		argIndex++
	}

	query += fmt.Sprintf(" ORDER BY pe.created_at DESC LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	queryArgs = append(queryArgs, args.Limit, args.Offset)

	rows, err := conn.DB.Query(context.Background(), query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("error querying Python executions: %v", err)
	}
	defer rows.Close()

	var executions []PythonExecution
	for rows.Next() {
		var execution PythonExecution
		var inputDataStr, outputDataStr sql.NullString

		err := rows.Scan(
			&execution.ID, &execution.PythonStrategyID, &execution.ExecutionID,
			&execution.UserID, &execution.Status, &execution.StartedAt,
			&execution.CompletedAt, &execution.ExecutionTimeMS, &execution.MemoryUsedMB,
			&execution.CPUUsedPercent, &inputDataStr, &outputDataStr,
			&execution.ErrorMessage, &execution.Logs, &execution.WorkerNode,
			&execution.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning Python execution: %v", err)
		}

		// Parse JSON data
		if inputDataStr.Valid {
			if err := json.Unmarshal([]byte(inputDataStr.String), &execution.InputData); err != nil {
				return nil, fmt.Errorf("error parsing input data: %v", err)
			}
		}
		if outputDataStr.Valid {
			if err := json.Unmarshal([]byte(outputDataStr.String), &execution.OutputData); err != nil {
				return nil, fmt.Errorf("error parsing output data: %v", err)
			}
		}

		executions = append(executions, execution)
	}

	return executions, nil
}

// GetPythonEnvironments retrieves available Python environments
func GetPythonEnvironments(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	query := `
		SELECT id, name, description, base_image, python_version, libraries,
			   environment_variables, is_default, is_active, created_at, updated_at
		FROM python_environments
		WHERE is_active = true
		ORDER BY is_default DESC, name ASC`

	rows, err := conn.DB.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("error querying Python environments: %v", err)
	}
	defer rows.Close()

	var environments []PythonEnvironment
	for rows.Next() {
		var env PythonEnvironment
		var librariesStr, envVarsStr string

		err := rows.Scan(
			&env.ID, &env.Name, &env.Description, &env.BaseImage,
			&env.PythonVersion, &librariesStr, &envVarsStr,
			&env.IsDefault, &env.IsActive, &env.CreatedAt, &env.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning Python environment: %v", err)
		}

		// Parse JSON data
		if err := json.Unmarshal([]byte(librariesStr), &env.Libraries); err != nil {
			return nil, fmt.Errorf("error parsing libraries: %v", err)
		}
		if err := json.Unmarshal([]byte(envVarsStr), &env.EnvironmentVariables); err != nil {
			return nil, fmt.Errorf("error parsing environment variables: %v", err)
		}

		environments = append(environments, env)
	}

	return environments, nil
}