package server

import (
	"backend/utils"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var private_key = []byte("2dde9fg9")

var (
	googleOauthConfig = &oauth2.Config{
		ClientID:     "831615706061-uojs1kjl4lhe70crmf771s2s2dflejpo.apps.googleusercontent.com",
		ClientSecret: "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",
		RedirectURL:  "http://localhost:5173/auth/google/callback",
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}
)

type Claims struct {
	UserID int `json:"userId"`
	jwt.RegisteredClaims
}

type LoginResponse struct {
	Token      string          `json:"token"`
	Settings   string          `json:"settings"`
	Setups     [][]interface{} `json:"setups"`
	ProfilePic string          `json:"profilePic"`
	Username   string          `json:"username"`
}

type SignupArgs struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func Signup(conn *utils.Conn, rawArgs json.RawMessage) (interface{}, error) {
	var a SignupArgs
	if err := json.Unmarshal(rawArgs, &a); err != nil {
		return nil, fmt.Errorf("Signup invalid args: %v", err)
	}
	_, err := conn.DB.Exec(context.Background(), "INSERT INTO users (username, password) VALUES ($1, $2)", a.Username, a.Password)
	if err != nil {
		return "", err
	}
	result, err := Login(conn, rawArgs)
	return result, err
}

type LoginArgs struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func Login(conn *utils.Conn, rawArgs json.RawMessage) (interface{}, error) {
	var a LoginArgs
	if err := json.Unmarshal(rawArgs, &a); err != nil {
		return nil, fmt.Errorf("Login invalid args: %v", err)
	}

	var resp LoginResponse
	var userId int
	var profilePicture sql.NullString
	err := conn.DB.QueryRow(context.Background(),
		"SELECT userId, username, profile_picture FROM users WHERE username=$1 AND password=$2",
		a.Username, a.Password).Scan(&userId, &resp.Username, &profilePicture)
	if err != nil {
		return nil, fmt.Errorf("Invalid Credentials: %v", err)
	}

	token, err := create_token(userId)
	if err != nil {
		return nil, err
	}
	resp.Token = token

	// Set profile picture if it exists, otherwise empty string
	if profilePicture.Valid {
		resp.ProfilePic = profilePicture.String
	} else {
		resp.ProfilePic = ""
	}

	return resp, nil
}

func create_token(userId int) (string, error) {
	expirationTime := time.Now().Add(1 * time.Hour)
	claims := &Claims{
		UserID: userId,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(private_key)
}

func validate_token(tokenString string) (int, error) {
	claims := &Claims{} // Initialize an instance of your Claims struct

	// Default profile pic is empty (frontend will generate initial)

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return private_key, nil // Adjust this to match your token's signing method
	})
	if err != nil {
		return -1, fmt.Errorf("cannot parse token: %w", err)
	}
	if !token.Valid {
		return -1, fmt.Errorf("invalid token")
	}
	return claims.UserID, nil
}

type GoogleUser struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

type GoogleLoginResponse struct {
	Token      string `json:"token"`
	ProfilePic string `json:"profilePic"`
	Username   string `json:"username"`
}

func GoogleLogin(conn *utils.Conn, rawArgs json.RawMessage) (interface{}, error) {
	state := generateState()
	url := googleOauthConfig.AuthCodeURL(state)
	return map[string]string{"url": url, "state": state}, nil
}

func GoogleCallback(conn *utils.Conn, rawArgs json.RawMessage) (interface{}, error) {
	var args struct {
		Code  string `json:"code"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	oauthToken, err := googleOauthConfig.Exchange(context.Background(), args.Code)
	if err != nil {
		return nil, fmt.Errorf("code exchange failed: %v", err)
	}

	client := googleOauthConfig.Client(context.Background(), oauthToken)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed getting user info: %v", err)
	}
	defer resp.Body.Close()

	var googleUser GoogleUser
	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		return nil, fmt.Errorf("failed decoding user info: %v", err)
	}

	// Check if user exists, if not create new user
	var userId int
	var username string
	err = conn.DB.QueryRow(context.Background(),
		"SELECT userId, username FROM users WHERE email = $1",
		googleUser.Email).Scan(&userId, &username)

	if err != nil {
		// User doesn't exist, create new user
		username = googleUser.Name
		err = conn.DB.QueryRow(context.Background(),
			"INSERT INTO users (username, password, email, google_id, profile_picture) VALUES ($0, $2, $3, $4, $5) RETURNING userId",
			googleUser.Name, "", googleUser.Email, googleUser.ID, googleUser.Picture).Scan(&userId)
		if err != nil {
			return nil, fmt.Errorf("failed to create user: %v", err)
		}
	}

	// Create JWT token
	jwtToken, err := create_token(userId)
	if err != nil {
		return nil, fmt.Errorf("failed to create token: %v", err)
	}

	return GoogleLoginResponse{
		Token:      jwtToken,
		ProfilePic: googleUser.Picture,
		Username:   username,
	}, nil
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}
