package server

import (
	"backend/utils"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"

	"fmt"
	"log"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// Get JWT secret from environment variable or use a fallback for development
var privateKey = []byte(getEnvOrDefault("JWT_SECRET", "2dde9fg9"))

// Get OAuth configuration from environment variables
var (
	googleOauthConfig = &oauth2.Config{
		ClientID:     getEnvOrDefault("GOOGLE_CLIENT_ID", ""),
		ClientSecret: getEnvOrDefault("GOOGLE_CLIENT_SECRET", ""),
		RedirectURL:  getEnvOrDefault("GOOGLE_REDIRECT_URL", ""),
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}
)

// Helper function to get environment variables with defaults
func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// Claims represents a structure for handling Claims data.
type Claims struct {
	UserID int `json:"userId"`
	jwt.RegisteredClaims
}

// LoginResponse represents a structure for handling LoginResponse data.
type LoginResponse struct {
	Token      string          `json:"token"`
	Settings   string          `json:"settings"`
	Setups     [][]interface{} `json:"setups"`
	ProfilePic string          `json:"profilePic"`
	Username   string          `json:"username"`
}

// SignupArgs represents a structure for handling SignupArgs data.
type SignupArgs struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// Signup performs operations related to Signup functionality.
func Signup(conn *utils.Conn, rawArgs json.RawMessage) (interface{}, error) {
	fmt.Println("=== SIGNUP ATTEMPT STARTED ===")
	fmt.Printf("Connection pool stats - Max: %d, Total: %d, Idle: %d, Acquired: %d\n",
		conn.DB.Stat().MaxConns(),
		conn.DB.Stat().TotalConns(),
		conn.DB.Stat().IdleConns(),
		conn.DB.Stat().AcquiredConns())

	var a SignupArgs
	if err := json.Unmarshal(rawArgs, &a); err != nil {
		fmt.Printf("ERROR: Failed to unmarshal signup args: %v\n", err)
		return nil, fmt.Errorf("Signup invalid args: %v", err)
	}

	fmt.Printf("Attempting to create account for email: %s, username: %s\n", a.Email, a.Username)

	// Create a timeout context to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check if email already exists
	var count int
	fmt.Println("Checking if email exists...")
	err := conn.DB.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE email=$1", a.Email).Scan(&count)
	if err != nil {
		fmt.Printf("ERROR: Database query failed while checking email: %v\n", err)
		// Print connection pool stats after error
		fmt.Printf("Connection pool stats after error - Max: %d, Total: %d, Idle: %d, Acquired: %d\n",
			conn.DB.Stat().MaxConns(),
			conn.DB.Stat().TotalConns(),
			conn.DB.Stat().IdleConns(),
			conn.DB.Stat().AcquiredConns())
		return nil, fmt.Errorf("error checking email: %v", err)
	}

	if count > 0 {
		fmt.Printf("Email already registered: %s\n", a.Email)
		return nil, fmt.Errorf("email already registered")
	}

	// Insert new user with auth_type='password'
	var userID int
	fmt.Println("Inserting new user record...")
	err = conn.DB.QueryRow(ctx,
		"INSERT INTO users (username, email, password, auth_type) VALUES ($1, $2, $3, $4) RETURNING userId",
		a.Username, a.Email, a.Password, "password").Scan(&userID)
	if err != nil {
		fmt.Printf("ERROR: Failed to create user: %v\n", err)
		// Print connection pool stats after error
		fmt.Printf("Connection pool stats after error - Max: %d, Total: %d, Idle: %d, Acquired: %d\n",
			conn.DB.Stat().MaxConns(),
			conn.DB.Stat().TotalConns(),
			conn.DB.Stat().IdleConns(),
			conn.DB.Stat().AcquiredConns())
		return nil, fmt.Errorf("error creating user: %v", err)
	}

	fmt.Printf("User created successfully with ID: %d\n", userID)

	// Create a journal entry for the new user using the current timestamp
	currentTime := time.Now().UTC()
	fmt.Println("Creating initial journal entry...")
	_, err = conn.DB.Exec(ctx,
		"INSERT INTO journals (timestamp, userId, entry) VALUES ($1, $2, $3)",
		currentTime.Unix(), userID, "{}")
	if err != nil {
		// Log the error but continue with signup process
		fmt.Printf("WARNING: Error creating initial journal for user %d: %v\n", userID, err)
	} else {
		fmt.Println("Journal entry created successfully")
	}

	// Create modified login args with the email
	fmt.Println("Preparing login...")
	loginArgs, err := json.Marshal(map[string]string{
		"email":    a.Email,
		"password": a.Password,
	})
	if err != nil {
		fmt.Printf("ERROR: Failed to prepare login: %v\n", err)
		return nil, fmt.Errorf("error preparing login: %v", err)
	}

	// Log in the new user
	fmt.Println("Attempting to log in new user...")
	result, err := Login(conn, loginArgs)
	if err != nil {
		fmt.Printf("ERROR: Login after signup failed: %v\n", err)
	} else {
		fmt.Println("Login successful")
	}

	// Print final connection pool stats
	fmt.Printf("Connection pool stats at end of signup - Max: %d, Total: %d, Idle: %d, Acquired: %d\n",
		conn.DB.Stat().MaxConns(),
		conn.DB.Stat().TotalConns(),
		conn.DB.Stat().IdleConns(),
		conn.DB.Stat().AcquiredConns())
	fmt.Println("=== SIGNUP ATTEMPT COMPLETED ===")

	return result, err
}

// LoginArgs represents a structure for handling LoginArgs data.
type LoginArgs struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login performs operations related to Login functionality.
func Login(conn *utils.Conn, rawArgs json.RawMessage) (interface{}, error) {
	fmt.Println("=== LOGIN ATTEMPT STARTED ===")
	fmt.Printf("Connection pool stats - Max: %d, Total: %d, Idle: %d, Acquired: %d\n",
		conn.DB.Stat().MaxConns(),
		conn.DB.Stat().TotalConns(),
		conn.DB.Stat().IdleConns(),
		conn.DB.Stat().AcquiredConns())

	var a LoginArgs
	if err := json.Unmarshal(rawArgs, &a); err != nil {
		fmt.Printf("ERROR: Failed to unmarshal login args: %v\n", err)
		return nil, fmt.Errorf("login invalid args: %v", err)
	}

	fmt.Printf("Login attempt for email: %s\n", a.Email)

	// Create a timeout context to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var resp LoginResponse
	var userID int
	var profilePicture sql.NullString
	var authType string

	// First check if the user exists and get their auth_type
	fmt.Println("Querying user info...")
	err := conn.DB.QueryRow(ctx,
		"SELECT userId, username, profile_picture, auth_type FROM users WHERE email=$1",
		a.Email).Scan(&userID, &resp.Username, &profilePicture, &authType)

	if err != nil {
		fmt.Printf("ERROR: User lookup failed: %v\n", err)
		// Print connection pool stats after error
		fmt.Printf("Connection pool stats after error - Max: %d, Total: %d, Idle: %d, Acquired: %d\n",
			conn.DB.Stat().MaxConns(),
			conn.DB.Stat().TotalConns(),
			conn.DB.Stat().IdleConns(),
			conn.DB.Stat().AcquiredConns())
		return nil, fmt.Errorf("invalid credentials: %v", err)
	}

	fmt.Printf("Found user with ID: %d, username: %s, auth_type: %s\n", userID, resp.Username, authType)

	// Check if this is a Google-only auth user trying to use password login
	if authType == "google" {
		fmt.Println("ERROR: Google-only user attempting password login")
		return nil, fmt.Errorf("this account uses Google Sign-In. Please login with Google")
	}

	// Now verify the password for password-based or both auth types
	fmt.Println("Verifying password...")
	var passwordMatch bool
	err = conn.DB.QueryRow(ctx,
		"SELECT (password = $1) FROM users WHERE userId=$2 AND (auth_type='password' OR auth_type='both')",
		a.Password, userID).Scan(&passwordMatch)

	if err != nil || !passwordMatch {
		if err != nil {
			fmt.Printf("ERROR: Password verification query failed: %v\n", err)
		} else {
			fmt.Println("ERROR: Password mismatch")
		}

		// Print connection pool stats after error
		fmt.Printf("Connection pool stats after error - Max: %d, Total: %d, Idle: %d, Acquired: %d\n",
			conn.DB.Stat().MaxConns(),
			conn.DB.Stat().TotalConns(),
			conn.DB.Stat().IdleConns(),
			conn.DB.Stat().AcquiredConns())

		return nil, fmt.Errorf("invalid credentials")
	}

	fmt.Println("Password verified, creating authentication token...")
	token, err := createToken(userID)
	if err != nil {
		fmt.Printf("ERROR: Token creation failed: %v\n", err)
		return nil, err
	}
	resp.Token = token
	fmt.Println("Token created successfully")

	// Set profile picture if it exists, otherwise empty string
	if profilePicture.Valid {
		resp.ProfilePic = profilePicture.String
		fmt.Println("Using stored profile picture")
	} else {
		resp.ProfilePic = ""
		fmt.Println("No profile picture found")
	}

	// Print final connection pool stats
	fmt.Printf("Connection pool stats at end of login - Max: %d, Total: %d, Idle: %d, Acquired: %d\n",
		conn.DB.Stat().MaxConns(),
		conn.DB.Stat().TotalConns(),
		conn.DB.Stat().IdleConns(),
		conn.DB.Stat().AcquiredConns())
	fmt.Println("=== LOGIN ATTEMPT COMPLETED ===")

	return resp, nil
}

// GuestLogin performs a login for a guest user without requiring credentials
func GuestLogin(conn *utils.Conn, rawArgs json.RawMessage) (interface{}, error) {
	fmt.Println("=== GUEST LOGIN ATTEMPT STARTED ===")

	// Create a timeout context to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var resp LoginResponse
	var userID int

	// Check if a guest user already exists
	var count int
	err := conn.DB.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE email='guest@atlantis.local'").Scan(&count)

	if err != nil {
		fmt.Printf("ERROR: Failed to check for existing guest user: %v\n", err)
		return nil, fmt.Errorf("guest login failed: %v", err)
	}

	if count == 0 {
		// No guest user exists, create one
		fmt.Println("Creating new guest user...")
		err = conn.DB.QueryRow(ctx,
			"INSERT INTO users (username, email, password, auth_type) VALUES ($1, $2, $3, $4) RETURNING userId",
			"Guest", "guest@atlantis.local", "guest-password", "guest").Scan(&userID)

		if err != nil {
			fmt.Printf("ERROR: Failed to create guest user: %v\n", err)
			return nil, fmt.Errorf("failed to create guest account: %v", err)
		}

		// Set username for response
		resp.Username = "Guest"

		// Create initial journal entry for guest user
		currentTime := time.Now().UTC()
		_, err = conn.DB.Exec(ctx,
			"INSERT INTO journals (timestamp, userId, entry) VALUES ($1, $2, $3)",
			currentTime.Unix(), userID, "{}")

		if err != nil {
			// Just log the error but continue
			fmt.Printf("WARNING: Error creating initial journal for guest user: %v\n", err)
		}
	} else {
		// Guest user exists, get the user ID
		fmt.Println("Using existing guest user...")
		err = conn.DB.QueryRow(ctx,
			"SELECT userId, username FROM users WHERE email='guest@atlantis.local'").Scan(&userID, &resp.Username)

		if err != nil {
			fmt.Printf("ERROR: Failed to get existing guest user: %v\n", err)
			return nil, fmt.Errorf("guest login failed: %v", err)
		}
	}

	// Create authentication token for guest user
	fmt.Printf("Creating token for guest user ID: %d\n", userID)
	token, err := createToken(userID)
	if err != nil {
		fmt.Printf("ERROR: Token creation failed: %v\n", err)
		return nil, err
	}

	resp.Token = token
	resp.ProfilePic = "" // Guest users don't have a profile picture

	fmt.Println("=== GUEST LOGIN COMPLETED ===")
	return resp, nil
}

func createToken(userId int) (string, error) {
	expirationTime := time.Now().Add(1 * time.Hour)
	claims := &Claims{
		UserID: userId,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(privateKey)
}

func validateToken(tokenString string) (int, error) {
	claims := &Claims{} // Initialize an instance of your Claims struct

	// Default profile pic is empty (frontend will generate initial)

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return privateKey, nil // Adjust this to match your token's signing method
	})
	if err != nil {
		return -1, fmt.Errorf("cannot parse token: %w", err)
	}
	if !token.Valid {
		return -1, fmt.Errorf("invalid token")
	}
	return claims.UserID, nil
}

// GoogleUser represents a structure for handling GoogleUser data.
type GoogleUser struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// GoogleLoginResponse represents a structure for handling GoogleLoginResponse data.
type GoogleLoginResponse struct {
	Token      string `json:"token"`
	ProfilePic string `json:"profilePic"`
	Username   string `json:"username"`
}

// GoogleLogin performs operations related to GoogleLogin functionality.
func GoogleLogin(conn *utils.Conn, rawArgs json.RawMessage) (interface{}, error) {
	var args struct {
		RedirectOrigin string `json:"redirectOrigin"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	// Validate that we have the required OAuth configuration
	if googleOauthConfig.ClientID == "" || googleOauthConfig.ClientSecret == "" {
		return nil, fmt.Errorf("google OAuth is not configured properly. Missing client ID or secret")
	}

	// Print debug information
	fmt.Printf("Received redirectOrigin: %s\n", args.RedirectOrigin)

	// Update the redirect URL based on the origin if no environment variable is set
	if os.Getenv("GOOGLE_REDIRECT_URL") == "" {
		googleOauthConfig.RedirectURL = args.RedirectOrigin + "/auth/google/callback"
	}

	// Print the configured redirect URL for debugging
	fmt.Printf("Using RedirectURL: %s\n", googleOauthConfig.RedirectURL)

	state := generateState()
	url := googleOauthConfig.AuthCodeURL(state)
	return map[string]string{"url": url, "state": state}, nil
}

// GoogleCallback performs operations related to GoogleCallback functionality.
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
	var userID int
	var username string
	var authType string

	err = conn.DB.QueryRow(context.Background(),
		"SELECT userId, username, auth_type FROM users WHERE email = $1",
		googleUser.Email).Scan(&userID, &username, &authType)

	if err != nil {
		// User doesn't exist, create new user with auth_type='google'
		username = googleUser.Name
		err = conn.DB.QueryRow(context.Background(),
			"INSERT INTO users (username, password, email, google_id, profile_picture, auth_type) VALUES ($1, $2, $3, $4, $5, $6) RETURNING userId",
			googleUser.Name, "", googleUser.Email, googleUser.ID, googleUser.Picture, "google").Scan(&userID)
		if err != nil {
			return nil, fmt.Errorf("failed to create user: %v", err)
		}
	} else if authType == "password" {
		// If this was a password user who is now using Google, update their account to link Google
		// but change auth_type to "both" to preserve password login ability
		_, err = conn.DB.Exec(context.Background(),
			"UPDATE users SET google_id = $1, profile_picture = $2, auth_type = $3 WHERE userId = $4",
			googleUser.ID, googleUser.Picture, "both", userID)
		if err != nil {
			return nil, fmt.Errorf("failed to update user: %v", err)
		}
	}

	// Create JWT token
	jwtToken, err := createToken(userID)
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
	if _, err := rand.Read(b); err != nil {
		// If there's an error reading random bytes, log it and return a fallback value
		log.Printf("Error generating random state: %v", err)
		// Use current time as fallback for some randomness
		return base64.URLEncoding.EncodeToString([]byte(time.Now().String()))
	}
	return base64.URLEncoding.EncodeToString(b)
}
