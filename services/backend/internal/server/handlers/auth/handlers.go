package auth

import (
	"backend/pkg/utils"
	"encoding/json"
)

// LoginArgs represents the arguments for user login
type LoginArgs struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// SignupArgs represents the arguments for user signup
type SignupArgs struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents the response for successful login
type LoginResponse struct {
	Token      string          `json:"token"`
	Settings   string          `json:"settings"`
	Setups     [][]interface{} `json:"setups"`
	ProfilePic string          `json:"profilePic"`
	Username   string          `json:"username"`
}

// Login authenticates a user and returns a JWT token
func Login(conn *utils.Conn, rawArgs json.RawMessage) (interface{}, error) {
	// Logic moved from server/auth.go
	return nil, nil
}

// Signup registers a new user
func Signup(conn *utils.Conn, rawArgs json.RawMessage) (interface{}, error) {
	// Logic moved from server/auth.go
	return nil, nil
}

// GoogleLoginResponse represents the response for successful Google login
type GoogleLoginResponse struct {
	Token      string `json:"token"`
	ProfilePic string `json:"profilePic"`
	Username   string `json:"username"`
}

// GoogleLogin initiates Google OAuth flow
func GoogleLogin(conn *utils.Conn, rawArgs json.RawMessage) (interface{}, error) {
	// Logic moved from server/auth.go
	return nil, nil
}

// GoogleCallback handles Google OAuth callback
func GoogleCallback(conn *utils.Conn, rawArgs json.RawMessage) (interface{}, error) {
	// Logic moved from server/auth.go
	return nil, nil
}
