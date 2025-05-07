package tools

import (
	"backend/utils"
	"context"
    "log"
	"database/sql"
	"encoding/json"
	"fmt"
)



type GetStrategySpecArgs struct {
	StrategyId int `json:"strategyId"`
}

func GetStrategySpec(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetStrategySpecArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}
	return _getStrategySpec(conn, args.StrategyId, userId)
}

// _getStrategySpec retrieves the raw spec JSON, unmarshals it, converts IDs to names,
// and then remarshals it for the API response.
func _getStrategySpec(conn *utils.Conn, strategyId int, userId int) (json.RawMessage, error) {
	var rawSpec json.RawMessage
	err := conn.DB.QueryRow(context.Background(), `
		SELECT spec
		FROM strategies WHERE strategyId = $1 AND userId = $2`, strategyId, userId).Scan(&rawSpec)
	if err != nil {
		if err == sql.ErrNoRows {

			return nil, fmt.Errorf("strategy not found or permission denied")
		}
		return nil, fmt.Errorf("error querying strategy spec: %w", err)
	}

	// Unmarshal the raw JSON into the Spec struct
	var spec Spec
	if err := json.Unmarshal(rawSpec, &spec); err != nil {
		// Log the problematic JSON for debugging
		log.Printf("Error unmarshaling spec JSON from DB for strategy %d: %v. JSON: %s", strategyId, err, string(rawSpec))
		return nil, fmt.Errorf("error parsing stored strategy spec: %w", err)
	}

	// Convert IDs back to names for API response
	if err := convertSpecIdsToNames(&spec); err != nil {
		// Log the spec that caused the error
		log.Printf("Error converting spec IDs to names for strategy %d: %v. Spec: %+v", strategyId, err, spec)
		return nil, fmt.Errorf("error processing strategy spec for display: %w", err)
	}

	// Marshal the modified spec (with names) back to JSON
	responseJSON, err := json.Marshal(spec)
	if err != nil {
		// This should be unlikely if the struct is valid
		log.Printf("Error marshaling spec back to JSON after ID-to-name conversion for strategy %d: %v", strategyId, err)
		return nil, fmt.Errorf("internal error preparing strategy spec response: %w", err)
	}

	return responseJSON, nil
}

// GetStrategies performs operations related to GetStrategies functionality.
func GetStrategies(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	rows, err := conn.DB.Query(context.Background(), `
    SELECT strategyId, name
    FROM strategies WHERE userId = $1`, userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var strategies []Strategy
	for rows.Next() {
		var strategy Strategy

		if err := rows.Scan(&strategy.StrategyId, &strategy.Name); err != nil {
			return nil, fmt.Errorf("error scanning strategy: %v", err)
		}

		// Get the score from the studies table (if available)
		var score sql.NullInt32
		err := conn.DB.QueryRow(context.Background(), `
			SELECT COUNT(*) FROM studies 
			WHERE userId = $1 AND strategyId = $2 AND completed = true`,
			userId, strategy.StrategyId).Scan(&score)

		if err == nil && score.Valid {
			strategy.Score = int(score.Int32)
		}

		strategies = append(strategies, strategy)
	}

	return strategies, nil
}

// No longer needed:
// type NewStrategyArgs struct { ... }

// _newStrategy saves a validated strategy spec to the database.
// It now takes userId, name, and the validated Spec object directly.
func NewStrategyUtil(conn *utils.Conn, userId int, name string, spec Spec) (int, error) {
	if name == "" {
		return -1, fmt.Errorf("strategy name cannot be empty")
	}
	// userId is assumed to be validated by the caller function's context

	// Convert names to IDs before storing
	if err := convertSpecNamesToIds(&spec); err != nil {
		return -1, fmt.Errorf("error converting spec names to IDs: %w", err)
	}

	// Convert the modified spec object (with IDs) back to JSON for database storage
	specJSON, err := json.Marshal(spec)
	if err != nil {
		// This should ideally not happen if the spec was correctly constructed/converted
		return -1, fmt.Errorf("internal error marshaling spec with IDs: %w", err)
	}

	var strategyID int
	// Ensure the userId from the function argument is used
	err = conn.DB.QueryRow(context.Background(), `
		INSERT INTO strategies (name, spec, userId)
		VALUES ($1, $2, $3) RETURNING strategyId`,
		name, specJSON, userId, // Use the passed userId
	).Scan(&strategyID)

	if err != nil {
		// Consider checking for specific DB errors (e.g., unique constraint violation) if needed
		return -1, fmt.Errorf("error inserting strategy into database: %w", err)
	}
	fmt.Printf("Successfully created strategy with ID: %d for user %d\n", strategyID, userId)
	return strategyID, nil
}

// NewStrategy performs operations related to NewStrategy functionality.
// It expects a JSON object with "name" and "spec" fields.
func NewStrategy(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	// Use the new helper function to unmarshal and validate the input
	name, spec, err := UnmarshalAndValidateNewStrategyInput(rawArgs)
	if err != nil {
		// Error message from helper is already descriptive
		return nil, fmt.Errorf("invalid new strategy input: %w", err)
	}

	// Call _newStrategy with validated data
	strategyId, err := NewStrategyUtil(conn, userId, name, spec)
	if err != nil {
		return nil, err // _newStrategy already formats the error
	}

	// Return the created strategy details using the main Strategy struct
	return Strategy{
		StrategyId: strategyId,
		UserId:     userId, // Reflect the correct user ID
		Name:       name,
		Spec:       spec, // Return the validated spec
		Score:      0,    // New strategy has no score yet
		// Other fields like CreationTimestamp, AlertActive etc., would be set by DB defaults or other logic
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
// Note: We'll parse into the main Strategy struct for consistency.
// type SetStrategyArgs struct {
// 	StrategyID int          `json:"strategyId"`
// 	Name       string       `json:"name"`
// 	Spec   Spec `json:"spec"`
// }

// _setStrategy updates an existing strategy in the database after validation.
func _setStrategy(conn *utils.Conn, userId int, strategyId int, name string, spec Spec) error {
	if name == "" {
		return fmt.Errorf("strategy name cannot be empty")
	}
	if strategyId <= 0 {
		return fmt.Errorf("invalid strategy ID")
	}

	// Convert names to IDs before storing
	if err := convertSpecNamesToIds(&spec); err != nil {
		return fmt.Errorf("error converting spec names to IDs: %w", err)
	}

	// Convert the modified spec object (with IDs) back to JSON for database storage
	specJSON, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("internal error marshaling spec with IDs: %w", err)
	}

	// Update the strategy, ensuring the userId matches for authorization
	cmdTag, err := conn.DB.Exec(context.Background(), `
		UPDATE strategies
		SET name = $1, spec = $2
		WHERE strategyId = $3 AND userId = $4`,
		name, specJSON, strategyId, userId) // Use userId from context

	if err != nil {
		return fmt.Errorf("error updating strategy in database: %w", err)
	}
	if cmdTag.RowsAffected() != 1 {
		// This means either the strategyId didn't exist or it didn't belong to the user
		return fmt.Errorf("strategy not found or permission denied")
	}
	fmt.Printf("Successfully updated strategy ID: %d for user %d\n", strategyId, userId)
	return nil
}

// SetStrategy performs operations related to SetStrategy functionality.
// It expects a JSON object containing the strategyId, new Name, and new Spec.
func SetStrategy(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	// Use the new helper function to unmarshal and validate the input
	strategyId, name, spec, err := UnmarshalAndValidateSetStrategyInput(rawArgs)
	if err != nil {
		// Error message from helper is already descriptive
		return nil, fmt.Errorf("invalid set strategy input: %w", err)
	}

	// Call _setStrategy with validated data and userId from context
	err = _setStrategy(conn, userId, strategyId, name, spec)
	if err != nil {
		return nil, err // _setStrategy already formats the error
	}

	// Return the updated strategy details using the main Strategy struct
	return Strategy{
		StrategyId: strategyId,
		UserId:     userId, // Reflect the correct user ID
		Name:       name,
		Spec:       spec, // Return the validated spec
		// Score is not updated here, would need separate logic/query if needed
	}, nil
}
