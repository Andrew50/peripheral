package settings

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
)

// GetSettings performs operations related to GetSettings functionality.
func GetSettings(conn *data.Conn, userID int, _ json.RawMessage) (interface{}, error) {
	var settings json.RawMessage
	err := conn.DB.QueryRow(context.Background(), "SELECT settings from users where userId = $1", userID).Scan(&settings)
	if err != nil {
		return nil, err
	}
	return settings, nil
}

// SetSettingsArgs represents a structure for handling SetSettingsArgs data.
type SetSettingsArgs struct {
	Settings json.RawMessage `json:"settings"`
}

// SetSettings performs operations related to SetSettings functionality.
func SetSettings(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args SetSettingsArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("m0ivn0d %v", err)
	}
	cmdTag, err := conn.DB.Exec(context.Background(), "UPDATE users SET settings = $1 where userId = $2", args.Settings, userID)
	if err != nil {
		return nil, fmt.Errorf("nv20v %v", err)
	}
	if cmdTag.RowsAffected() != 1 {
		return nil, fmt.Errorf("o2inv")
	}
	return nil, nil
}

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
