package settings

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
)

// UpdateProfilePictureArgs represents a structure for handling UpdateProfilePictureArgs data.
type UpdateProfilePictureArgs struct {
	ProfilePicture string `json:"profilePicture"`
}

// UpdateProfilePicture updates the user's profile picture.
func UpdateProfilePicture(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args struct {
		ProfilePicture string `json:"profilePicture"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	// Update the user's profile picture in the database
	_, err := conn.DB.Exec(
		context.Background(),
		"UPDATE users SET profile_picture = $1 WHERE userId = $2",
		args.ProfilePicture,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update profile picture: %v", err)
	}

	return map[string]string{"status": "success"}, nil
}
