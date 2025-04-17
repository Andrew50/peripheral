package tools

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
    "time"
)

// UpdateProfilePictureArgs represents a structure for handling UpdateProfilePictureArgs data.
type UpdateProfilePictureArgs struct {
	ProfilePicture string `json:"profilePicture"`
}

// UpdateProfilePicture performs operations related to UpdateProfilePicture functionality.
func UpdateProfilePicture(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args UpdateProfilePictureArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	// Update the user's profile picture in the database
	_, err := conn.DB.Exec(
		context.Background(),
		"UPDATE users SET profile_picture = $1 WHERE userId = $2",
		args.ProfilePicture,
		userId,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update profile picture: %v", err)
	}

	return map[string]string{"status": "success"}, nil
}
// DeleteAccount deletes a user account and all associated data
func DeleteAccount(conn *utils.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	fmt.Println("=== DELETE ACCOUNT ATTEMPT STARTED ===")

	// Parse arguments to get confirmation
	var args struct {
		Confirmation string `json:"confirmation"`
	}

	if err := json.Unmarshal(rawArgs, &args); err != nil {
		fmt.Printf("ERROR: Failed to unmarshal delete account args: %v\n", err)
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	// Verify confirmation
	if args.Confirmation != "DELETE" {
		return nil, fmt.Errorf("confirmation text must be 'DELETE' to proceed with account deletion")
	}

	// Create a timeout context to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Start a transaction
	tx, err := conn.DB.Begin(ctx)
	if err != nil {
		fmt.Printf("ERROR: Failed to start transaction: %v\n", err)
		return nil, fmt.Errorf("error starting transaction: %v", err)
	}

	// Ensure transaction is either committed or rolled back
	var txClosed bool
	defer func() {
		if !txClosed && tx != nil {
			fmt.Println("Rolling back transaction due to error or incomplete process")
			_ = tx.Rollback(context.Background())
		}
	}()

	// Get auth type for logging purposes
	var authType string
	err = tx.QueryRow(ctx, "SELECT auth_type FROM users WHERE userId = $1", userID).Scan(&authType)
	if err != nil {
		fmt.Printf("ERROR: Failed to get user account type: %v\n", err)
		return nil, fmt.Errorf("failed to get user account: %v", err)
	}

	fmt.Printf("Deleting account with ID: %d, type: %s\n", userID, authType)

	// Delete watchlists
	_, err = tx.Exec(ctx, "DELETE FROM watchlists WHERE userId = $1", userID)
	if err != nil {
		fmt.Printf("ERROR: Failed to delete watchlists: %v\n", err)
		// Continue despite error
	}

	// Delete setups
	_, err = tx.Exec(ctx, "DELETE FROM setups WHERE userId = $1", userID)
	if err != nil {
		fmt.Printf("ERROR: Failed to delete setups: %v\n", err)
		// Continue despite error
	}

	// Delete journal entries
	_, err = tx.Exec(ctx, "DELETE FROM journals WHERE userId = $1", userID)
	if err != nil {
		fmt.Printf("ERROR: Failed to delete journal entries: %v\n", err)
		// Continue despite error
	}

	// Delete the user
	_, err = tx.Exec(ctx, "DELETE FROM users WHERE userId = $1", userID)
	if err != nil {
		fmt.Printf("ERROR: Failed to delete user: %v\n", err)
		return nil, fmt.Errorf("failed to delete user: %v", err)
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		fmt.Printf("ERROR: Failed to commit transaction: %v\n", err)
		return nil, fmt.Errorf("error committing transaction: %v", err)
	}
	txClosed = true

	fmt.Printf("Successfully deleted account with ID: %d\n", userID)
	fmt.Println("=== DELETE ACCOUNT COMPLETED ===")

	return map[string]string{"status": "success"}, nil
}
