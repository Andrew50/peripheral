package tools 

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
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
