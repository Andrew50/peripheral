package tools

import (
	"backend/utils"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

/*type Algo struct {
	AlgoID   int    `json:"algoId"`
	AlgoName string `json:"algoName"`
}
// GetAlgos performs operations related to GetAlgos functionality.
func GetAlgos(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	rows, err := conn.DB.Query(context.Background(), `
		SELECT algoId, algoName
		FROM algos`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var algos []Algo
	for rows.Next() {
		var algo Algo
		if err := rows.Scan(&algo.AlgoID, &algo.AlgoName); err != nil {
			return nil, err
		}
	}
	return algos, nil
}*/

// StrategyCriteria represents the criteria for a strategy
type StrategyCriteria struct {
	Timeframe string  `json:"timeframe"`
	Bars      int     `json:"bars"`
	Threshold int     `json:"threshold"`
	Dolvol    float64 `json:"dolvol"`
	Adr       float64 `json:"adr"`
	Mcap      float64 `json:"mcap"`
}

// StrategyResult represents a strategy configuration with its evaluation score.
type StrategyResult struct {
	StrategyID int             `json:"strategyId"`
	Name       string          `json:"name"`
	Criteria   StrategyCriteria `json:"criteria"`
	Score      int             `json:"score"`
}

// GetStrategies performs operations related to GetStrategies functionality.
func GetStrategies(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	rows, err := conn.DB.Query(context.Background(), `
    SELECT strategyId, name, criteria
    FROM strategies WHERE userId = $1`, userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var strategies []StrategyResult
	for rows.Next() {
		var strategy StrategyResult
		var criteriaJSON json.RawMessage
		
		if err := rows.Scan(&strategy.StrategyID, &strategy.Name, &criteriaJSON); err != nil {
			return nil, fmt.Errorf("error scanning strategy: %v", err)
		}
		
		// Parse the criteria JSON
		if err := json.Unmarshal(criteriaJSON, &strategy.Criteria); err != nil {
			return nil, fmt.Errorf("error parsing criteria JSON: %v", err)
		}
		
		// Get the score from the studies table (if available)
		var score sql.NullInt32
		err := conn.DB.QueryRow(context.Background(), `
			SELECT COUNT(*) FROM studies 
			WHERE userId = $1 AND strategyId = $2 AND completed = true`, 
			userId, strategy.StrategyID).Scan(&score)
		
		if err == nil && score.Valid {
			strategy.Score = int(score.Int32)
		}
		
		strategies = append(strategies, strategy)
	}
	
	return strategies, nil
}

// NewStrategyArgs represents a structure for handling NewStrategyArgs data.
type NewStrategyArgs struct {
	Name     string          `json:"name"`
	Criteria StrategyCriteria `json:"criteria"`
}

// NewStrategy performs operations related to NewStrategy functionality.
func NewStrategy(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args NewStrategyArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, err
	}
	
	if args.Name == "" || args.Criteria.Timeframe == "" {
		return nil, fmt.Errorf("missing required fields")
	}
	
	// Convert criteria to JSON
	criteriaJSON, err := json.Marshal(args.Criteria)
	if err != nil {
		return nil, fmt.Errorf("error marshaling criteria: %v", err)
	}
	
	var strategyID int
	err = conn.DB.QueryRow(context.Background(), `
		INSERT INTO strategies (name, criteria, userId) 
		VALUES ($1, $2, $3) RETURNING strategyId`,
		args.Name, criteriaJSON, userId,
	).Scan(&strategyID)

	if err != nil {
		return nil, fmt.Errorf("error creating strategy: %v", err)
	}
	
	// Call the equivalent of CheckSampleQueue for strategies
	utils.CheckSampleQueue(conn, strategyID, false)
	
	return StrategyResult{
		StrategyID: strategyID,
		Name:      args.Name,
		Criteria:  args.Criteria,
		Score:     0, // New strategy has no score yet
	}, nil
}

// DeleteStrategyArgs represents a structure for handling DeleteStrategyArgs data.
type DeleteStrategyArgs struct {
	StrategyID int `json:"strategyId"`
}

// DeleteStrategy performs operations related to DeleteStrategy functionality.
func DeleteStrategy(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args DeleteStrategyArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, err
	}
	
	result, err := conn.DB.Exec(context.Background(), `
		DELETE FROM strategies 
		WHERE strategyId = $1 AND userId = $2`, args.StrategyID, userId)

	if err != nil {
		return nil, fmt.Errorf("error deleting strategy: %v", err)
	}

	// Check if any rows were affected
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, fmt.Errorf("strategy not found or you don't have permission to delete it")
	}

	return nil, nil
}

// SetStrategyArgs represents a structure for handling SetStrategyArgs data.
type SetStrategyArgs struct {
	StrategyID int             `json:"strategyId"`
	Name       string          `json:"name"`
	Criteria   StrategyCriteria `json:"criteria"`
}

// SetStrategy performs operations related to SetStrategy functionality.
func SetStrategy(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args SetStrategyArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing args: %v", err)
	}
	
	if args.StrategyID == 0 || args.Name == "" || args.Criteria.Timeframe == "" {
		return nil, fmt.Errorf("missing required fields")
	}
	
	// Convert criteria to JSON
	criteriaJSON, err := json.Marshal(args.Criteria)
	if err != nil {
		return nil, fmt.Errorf("error marshaling criteria: %v", err)
	}
	
	cmdTag, err := conn.DB.Exec(context.Background(), `
		UPDATE strategies 
		SET name = $1, criteria = $2
		WHERE strategyId = $3 AND userId = $4`,
		args.Name, criteriaJSON, args.StrategyID, userId)
		
	if err != nil {
		return nil, fmt.Errorf("error updating strategy: %v", err)
	} else if cmdTag.RowsAffected() != 1 {
		return nil, fmt.Errorf("strategy not found or you don't have permission to update it")
	}
	
	return StrategyResult{
		StrategyID: args.StrategyID,
		Name:      args.Name,
		Criteria:  args.Criteria,
		Score:     0, // We don't have the score here, it would need to be queried separately
	}, nil
}

