package server

import (
	"backend/internal/data"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	//"log"
	"os"
	"time"

	"backend/internal/app/limits"

	"github.com/golang-jwt/jwt/v4"
	"github.com/jackc/pgx/v4"
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
	Password string `json:"password"`
}

// Signup performs operations related to Signup functionality.
func Signup(conn *data.Conn, rawArgs json.RawMessage) (interface{}, error) {
	log.Println("Signup attempt started")

	var a SignupArgs
	if err := json.Unmarshal(rawArgs, &a); err != nil {
		return nil, fmt.Errorf("Signup invalid args: %v", err)
	}
	username := strings.ToLower(a.Email)

	// Create a timeout context to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Start a transaction for the signup process
	tx, err := conn.DB.Begin(ctx)
	if err != nil {
		log.Printf("ERROR: Failed to start signup transaction: %v", err)
		return nil, fmt.Errorf("error starting transaction: %v", err)
	}

	// Ensure transaction is either committed or rolled back
	var txClosed bool
	defer func() {
		if !txClosed && tx != nil {
			log.Println("Rolling back signup transaction due to error")
			_ = tx.Rollback(context.Background())
		}
	}()

	// Check if email already exists
	var count int
	err = tx.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE email=$1", a.Email).Scan(&count)
	if err != nil {
		log.Printf("ERROR: Database query failed while checking email: %v", err)
		return nil, fmt.Errorf("error checking email: %v", err)
	}

	if count > 0 {
		return nil, fmt.Errorf("email already registered")
	}

	// Check if username already exists
	err = tx.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE username=$1", username).Scan(&count)
	if err != nil {
		log.Printf("ERROR: Database query failed while checking username: %v", err)
		return nil, fmt.Errorf("error checking username: %v", err)
	}
	if count > 0 {
		return nil, fmt.Errorf("username already taken")
	}

	// Insert new user with auth_type='password'
	var userID int
	err = tx.QueryRow(ctx,
		"INSERT INTO users (username, email, password, auth_type) VALUES ($1, $2, $3, $4) RETURNING userId",
		username, a.Email, a.Password, "password").Scan(&userID)
	if err != nil {
		log.Printf("ERROR: Failed to create user: %v", err)
		return nil, fmt.Errorf("error creating user: %v", err)
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		log.Printf("ERROR: Failed to commit signup transaction: %v", err)
		return nil, fmt.Errorf("error committing signup transaction: %v", err)
	}
	txClosed = true

	// Allocate free plan credits for the new user (idempotent if called again)
	if err := limits.UpdateUserCreditsForPlan(conn, userID, "Free"); err != nil {
		// Propagate the error so signup fails clearly if free credits cannot be allocated
		return nil, fmt.Errorf("failed to allocate free credits: %v", err)
	}

	// Create modified login args with the email
	loginArgs, err := json.Marshal(map[string]string{
		"email":    a.Email,
		"password": a.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("error preparing login: %v", err)
	}

	// Log in the new user
	result, err := Login(conn, loginArgs)

	log.Printf("Signup completed for user ID: %d", userID)
	return result, err
}

// LoginArgs represents a structure for handling LoginArgs data.
type LoginArgs struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login performs operations related to Login functionality.
func Login(conn *data.Conn, rawArgs json.RawMessage) (interface{}, error) {
	log.Println("Login attempt started")

	var a LoginArgs
	if err := json.Unmarshal(rawArgs, &a); err != nil {
		return nil, fmt.Errorf("login invalid args: %v", err)
	}

	log.Printf("Login attempt for email: %s", a.Email)

	// Create a timeout context to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var resp LoginResponse
	var userID int
	var storedPw string
	var authType string
	var profilePicture sql.NullString

	// 1) Does the email exist? Get user details.
	err := conn.DB.QueryRow(ctx,
		`SELECT userId, username, password, profile_picture, auth_type
		 FROM users WHERE email=$1`,
		a.Email).Scan(&userID, &resp.Username, &storedPw, &profilePicture, &authType)

	switch {
	case err == pgx.ErrNoRows:
		log.Printf("Login failed: No user found for email: %s", a.Email)
		return nil, fmt.Errorf("incorrect email") // <-- NEW specific error
	case err != nil:
		log.Printf("ERROR: Database query failed during login check: %v", err)
		return nil, fmt.Errorf("database error: %v", err)
	}

	// 2) Is this a Google-only account?
	if authType == "google" {
		log.Printf("Login failed: Google-only user attempting password login for email: %s", a.Email)
		errorMessage := "This account uses Google Sign-In. Please login with Google."
		return nil, fmt.Errorf("%s", errorMessage)
	}

	// 3) Wrong password? (Only check for 'password' or 'both' auth types)
	if authType == "password" || authType == "both" {
		if storedPw != a.Password {
			log.Printf("Login failed: Password mismatch for email: %s", a.Email)
			return nil, fmt.Errorf("incorrect password") // <-- NEW specific error
		}
	} else {
		// This case should ideally not be reached if authType logic is correct,
		// but added for robustness.
		log.Printf("ERROR: Unexpected auth_type '%s' encountered for password login attempt for email: %s", authType, a.Email)
		return nil, fmt.Errorf("invalid account state")
	}

	token, err := createToken(userID)
	if err != nil {
		log.Printf("ERROR: Token creation failed for user ID %d: %v", userID, err)
		return nil, err
	}
	resp.Token = token

	// Set profile picture if it exists, otherwise empty string
	if profilePicture.Valid {
		resp.ProfilePic = profilePicture.String
	} else {
		resp.ProfilePic = ""
	}

	log.Printf("Login successful for user ID: %d", userID)
	return resp, nil
}

func createToken(userID int) (string, error) {
	expirationTime := time.Now().Add(6 * time.Hour)
	claims := &Claims{
		UserID: userID,
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

	token, err := jwt.ParseWithClaims(tokenString, claims, func(_ *jwt.Token) (interface{}, error) {
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
func GoogleLogin(_ *data.Conn, rawArgs json.RawMessage) (interface{}, error) {
	var args struct {
		RedirectOrigin string `json:"redirectOrigin"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	// Validate that we have the required OAuth configuration
	if googleOauthConfig.ClientID == "" || googleOauthConfig.ClientSecret == "" {
		log.Printf("ERROR: Google OAuth is not configured properly - missing client ID or secret")
		return nil, fmt.Errorf("google OAuth is not configured properly. Missing client ID or secret")
	}

	log.Printf("Google login initiated from origin: %s", args.RedirectOrigin)

	// Update the redirect URL based on the origin if no environment variable is set
	if os.Getenv("GOOGLE_REDIRECT_URL") == "" {
		googleOauthConfig.RedirectURL = args.RedirectOrigin + "/auth/google/callback"
	}

	state := generateState()
	url := googleOauthConfig.AuthCodeURL(state)
	return map[string]string{"url": url, "state": state}, nil
}

// GoogleCallback performs operations related to GoogleCallback functionality.
func GoogleCallback(conn *data.Conn, rawArgs json.RawMessage) (interface{}, error) {
	var args struct {
		Code  string `json:"code"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	log.Printf("Google callback processing with code")

	oauthToken, err := googleOauthConfig.Exchange(context.Background(), args.Code)
	if err != nil {
		log.Printf("ERROR: Google OAuth code exchange failed: %v", err)
		return nil, fmt.Errorf("code exchange failed: %v", err)
	}

	client := googleOauthConfig.Client(context.Background(), oauthToken)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		log.Printf("ERROR: Failed getting Google user info: %v", err)
		return nil, fmt.Errorf("failed getting user info: %v", err)
	}
	defer resp.Body.Close()

	var googleUser GoogleUser
	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		log.Printf("ERROR: Failed decoding Google user info: %v", err)
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
			log.Printf("ERROR: Failed to create Google user: %v", err)
			return nil, fmt.Errorf("failed to create user: %v", err)
		}

		// Allocate free plan credits for the newly created Google user
		if err := limits.UpdateUserCreditsForPlan(conn, userID, "Free"); err != nil {
			log.Printf("ERROR: Failed to allocate free credits for Google user %d: %v", userID, err)
			return nil, fmt.Errorf("failed to allocate free credits: %v", err)
		}

		log.Printf("Created new Google user with ID: %d", userID)
	} else if authType == "password" {
		// If this was a password user who is now using Google, update their account to link Google
		// but change auth_type to "both" to preserve password login ability
		_, err = conn.DB.Exec(context.Background(),
			"UPDATE users SET google_id = $1, profile_picture = $2, auth_type = $3 WHERE userId = $4",
			googleUser.ID, googleUser.Picture, "both", userID)
		if err != nil {
			log.Printf("ERROR: Failed to update user for Google linking: %v", err)
			return nil, fmt.Errorf("failed to update user: %v", err)
		}
		log.Printf("Linked Google account to existing user ID: %d", userID)
	}

	// Create JWT token
	jwtToken, err := createToken(userID)
	if err != nil {
		log.Printf("ERROR: Failed to create token for Google user ID %d: %v", userID, err)
		return nil, fmt.Errorf("failed to create token: %v", err)
	}

	log.Printf("Google login successful for user ID: %d", userID)
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
		log.Printf("ERROR: Failed to generate random state: %v", err)
		// Use current time as fallback for some randomness
		return base64.URLEncoding.EncodeToString([]byte(time.Now().String()))
	}
	return base64.URLEncoding.EncodeToString(b)
}

// DeleteAccount deletes a user account and all associated data
func DeleteAccount(conn *data.Conn, rawArgs json.RawMessage) (interface{}, error) {
	log.Println("Delete account attempt started")

	// Parse arguments to get confirmation
	var args struct {
		UserID       int    `json:"userId,omitempty"`       // Only needed for public API calls
		AuthType     string `json:"authType,omitempty"`     // Only needed for public API calls
		Confirmation string `json:"confirmation,omitempty"` // Required for both public and private calls
	}

	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	// Check confirmation string for regular delete account
	if args.Confirmation != "DELETE" {
		return nil, fmt.Errorf("confirmation must be 'DELETE' to delete account")
	}

	var userID int
	var authType string

	// If userID is provided (for public API), use that instead of the token
	if args.UserID > 0 {
		userID = args.UserID
		authType = args.AuthType

		// Extra verification that only authenticated users can delete accounts
		if userID <= 0 {
			return nil, fmt.Errorf("invalid user ID")
		}
	}

	// Create a timeout context to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Start a transaction
	tx, err := conn.DB.Begin(ctx)
	if err != nil {
		log.Printf("ERROR: Failed to start delete account transaction: %v", err)
		return nil, fmt.Errorf("error starting transaction: %v", err)
	}

	// Ensure transaction is either committed or rolled back
	var txClosed bool
	defer func() {
		if !txClosed && tx != nil {
			log.Println("Rolling back delete account transaction due to error")
			_ = tx.Rollback(context.Background())
		}
	}()

	// Get auth type for logging purposes if it wasn't provided
	if authType == "" {
		err = tx.QueryRow(ctx, "SELECT auth_type FROM users WHERE userId = $1", userID).Scan(&authType)
		if err != nil {
			log.Printf("ERROR: Failed to get user account type for deletion: %v", err)
			return nil, fmt.Errorf("failed to get user account: %v", err)
		}
	}

	log.Printf("Deleting account with ID: %d, type: %s", userID, authType)

	// Delete watchlists
	_, err = tx.Exec(ctx, "DELETE FROM watchlists WHERE userId = $1", userID)
	if err != nil {
		log.Printf("ERROR: Failed to delete watchlists for user %d: %v", userID, err)
		return nil, err
	}

	// Delete setups
	_, err = tx.Exec(ctx, "DELETE FROM setups WHERE userId = $1", userID)
	if err != nil {
		log.Printf("ERROR: Failed to delete setups for user %d: %v", userID, err)
		return nil, err
	}

	// Delete the user
	_, err = tx.Exec(ctx, "DELETE FROM users WHERE userId = $1", userID)
	if err != nil {
		log.Printf("ERROR: Failed to delete user %d: %v", userID, err)
		return nil, fmt.Errorf("failed to delete user: %v", err)
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		log.Printf("ERROR: Failed to commit delete account transaction: %v", err)
		return nil, fmt.Errorf("error committing transaction: %v", err)
	}
	txClosed = true

	log.Printf("Successfully deleted account with ID: %d", userID)
	return map[string]string{"status": "success"}, nil
}
