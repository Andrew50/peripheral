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

	// Start a transaction for the signup process
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

	// Check if email already exists
	var count int
	fmt.Println("Checking if email exists...")
	err = tx.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE email=$1", a.Email).Scan(&count)
	if err != nil {
		fmt.Printf("ERROR: Database query failed while checking email: %v\n", err)
		return nil, fmt.Errorf("error checking email: %v", err)
	}

	if count > 0 {
		fmt.Printf("Email already registered: %s\n", a.Email)
		return nil, fmt.Errorf("email already registered")
	}

	// Insert new user with auth_type='password'
	var userID int
	fmt.Println("Inserting new user record...")
	err = tx.QueryRow(ctx,
		"INSERT INTO users (username, email, password, auth_type) VALUES ($1, $2, $3, $4) RETURNING userId",
		a.Username, a.Email, a.Password, "password").Scan(&userID)
	if err != nil {
		fmt.Printf("ERROR: Failed to create user: %v\n", err)
		return nil, fmt.Errorf("error creating user: %v", err)
	}

	fmt.Printf("User created successfully with ID: %d\n", userID)

	// Create a journal entry for the new user using the current timestamp
	currentTime := time.Now().UTC()
	fmt.Println("Creating initial journal entry...")
	_, err = tx.Exec(ctx,
		"INSERT INTO journals (timestamp, userId, entry) VALUES ($1, $2, $3)",
		currentTime, userID, "{}")
	if err != nil {
		fmt.Printf("ERROR: Failed to create initial journal: %v\n", err)
		return nil, fmt.Errorf("error creating initial journal: %v", err)
	}
	fmt.Println("Journal entry created successfully")

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		fmt.Printf("ERROR: Failed to commit transaction: %v\n", err)
		return nil, fmt.Errorf("error committing signup transaction: %v", err)
	}
	txClosed = true
	fmt.Println("Signup transaction committed successfully")

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

	// Start a transaction for creating a guest user
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

	// Generate a random string for the guest user
	// This ensures we create a unique guest user for each session
	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %v", err)
	}
	randomStr := base64.URLEncoding.EncodeToString(randomBytes)
	guestEmail := fmt.Sprintf("guest_%s@temporary.guest", randomStr)
	guestUsername := fmt.Sprintf("Guest_%s", randomStr[:6])

	// Insert the guest user
	var userID int
	fmt.Println("Creating new guest user...")
	err = tx.QueryRow(ctx,
		"INSERT INTO users (username, email, password, auth_type) VALUES ($1, $2, $3, $4) RETURNING userId",
		guestUsername, guestEmail, "", "guest").Scan(&userID)

	if err != nil {
		fmt.Printf("ERROR: Failed to create guest user: %v\n", err)
		return nil, fmt.Errorf("failed to create guest account: %v", err)
	}

	resp.Username = guestUsername

	// Copy data from default guest account (userId 0) to new guest account
	err = copyGuestData(ctx, tx, userID)
	if err != nil {
		fmt.Printf("ERROR: Failed to copy guest data: %v\n", err)
		// Continue despite errors to ensure the guest can still log in
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		fmt.Printf("ERROR: Failed to commit transaction: %v\n", err)
		return nil, fmt.Errorf("error committing transaction: %v", err)
	}
	txClosed = true

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

// copyGuestData copies all data from the default guest user (userId 0) to a new guest user account
// This allows new guest accounts to have the same starting data as the template guest account
func copyGuestData(ctx context.Context, tx pgx.Tx, newUserID int) error {
	// Default guest user ID is 0
	const defaultGuestID = 0

	fmt.Printf("Copying data from default guest (ID: %d) to new guest (ID: %d)\n", defaultGuestID, newUserID)

	// 1. Copy settings from default guest user if any
	var settings sql.NullString
	err := tx.QueryRow(ctx, "SELECT settings FROM users WHERE userId = $1", defaultGuestID).Scan(&settings)
	if err == nil && settings.Valid {
		_, err = tx.Exec(ctx, "UPDATE users SET settings = $1 WHERE userId = $2", settings.String, newUserID)
		if err != nil {
			fmt.Printf("WARNING: Error copying settings: %v\n", err)
			// Continue despite error
		} else {
			fmt.Println("Copied user settings")
		}
	}

	// 2. Copy setups
	setupIDs := make(map[int]int) // Maps original setupIds to new setupIds
	rows, err := tx.Query(ctx, `
		SELECT name, timeframe, bars, threshold, modelVersion, score, sampleSize, 
		       untrainedSamples, dolvol, adr, mcap 
		FROM setups WHERE userId = $1`, defaultGuestID)
	if err != nil {
		fmt.Printf("WARNING: Error querying setups: %v\n", err)
	} else {
		defer rows.Close()

		for rows.Next() {
			var name, timeframe string
			var bars, threshold, modelVersion, score, sampleSize, untrainedSamples int
			var dolvol, adr, mcap float64

			err := rows.Scan(&name, &timeframe, &bars, &threshold, &modelVersion,
				&score, &sampleSize, &untrainedSamples, &dolvol, &adr, &mcap)
			if err != nil {
				fmt.Printf("WARNING: Error scanning setup row: %v\n", err)
				continue
			}

			// Insert the setup for the new user
			var oldSetupID, newSetupID int
			err = tx.QueryRow(ctx, "SELECT setupId FROM setups WHERE userId = $1 AND name = $2",
				defaultGuestID, name).Scan(&oldSetupID)
			if err != nil {
				fmt.Printf("WARNING: Error getting original setupId: %v\n", err)
				continue
			}

			err = tx.QueryRow(ctx, `
				INSERT INTO setups (userId, name, timeframe, bars, threshold, modelVersion, 
					score, sampleSize, untrainedSamples, dolvol, adr, mcap)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
				RETURNING setupId`,
				newUserID, name, timeframe, bars, threshold, modelVersion,
				score, sampleSize, untrainedSamples, dolvol, adr, mcap).Scan(&newSetupID)

			if err != nil {
				fmt.Printf("WARNING: Error inserting new setup: %v\n", err)
				continue
			}

			setupIDs[oldSetupID] = newSetupID
			fmt.Printf("Copied setup '%s' (ID: %d -> %d)\n", name, oldSetupID, newSetupID)
		}
	}

	// 3. Copy samples for each setup
	for oldSetupID, newSetupID := range setupIDs {
		_, err := tx.Exec(ctx, `
			INSERT INTO samples (setupId, securityId, timestamp, label)
			SELECT $1, securityId, timestamp, label
			FROM samples
			WHERE setupId = $2`,
			newSetupID, oldSetupID)

		if err != nil {
			fmt.Printf("WARNING: Error copying samples for setup %d: %v\n", oldSetupID, err)
		} else {
			fmt.Printf("Copied samples for setup ID: %d -> %d\n", oldSetupID, newSetupID)
		}
	}

	// 4. Copy studies
	_, err = tx.Exec(ctx, `
		INSERT INTO studies (userId, securityId, setupId, timestamp, completed, entry)
		SELECT $1, securityId, 
			CASE 
				WHEN setupId IN (SELECT setupId FROM setups WHERE userId = 0) 
				THEN (SELECT s2.setupId FROM setups s1 
					JOIN setups s2 ON s1.name = s2.name AND s2.userId = $1 
					WHERE s1.setupId = studies.setupId AND s1.userId = 0)
				ELSE setupId
			END,
			timestamp, completed, entry
		FROM studies
		WHERE userId = $2`,
		newUserID, defaultGuestID)

	if err != nil {
		fmt.Printf("WARNING: Error copying studies: %v\n", err)
	} else {
		fmt.Println("Copied studies")
	}

	// 5. Copy journals
	_, err = tx.Exec(ctx, `
		INSERT INTO journals (userId, timestamp, completed, entry)
		SELECT $1, timestamp, completed, entry
		FROM journals
		WHERE userId = $2`,
		newUserID, defaultGuestID)

	if err != nil {
		fmt.Printf("WARNING: Error copying journals: %v\n", err)
	} else {
		fmt.Println("Copied journals")
	}

	// 6. Copy watchlists and their items
	watchlistIDs := make(map[int]int) // Maps original watchlistIds to new watchlistIds
	rows, err = tx.Query(ctx, "SELECT watchlistId, watchlistName FROM watchlists WHERE userId = $1", defaultGuestID)
	if err != nil {
		fmt.Printf("WARNING: Error querying watchlists: %v\n", err)
	} else {
		defer rows.Close()

		for rows.Next() {
			var oldWatchlistID int
			var watchlistName string

			err := rows.Scan(&oldWatchlistID, &watchlistName)
			if err != nil {
				fmt.Printf("WARNING: Error scanning watchlist row: %v\n", err)
				continue
			}

			var newWatchlistID int
			err = tx.QueryRow(ctx, `
				INSERT INTO watchlists (userId, watchlistName)
				VALUES ($1, $2)
				RETURNING watchlistId`,
				newUserID, watchlistName).Scan(&newWatchlistID)

			if err != nil {
				fmt.Printf("WARNING: Error inserting new watchlist: %v\n", err)
				continue
			}

			watchlistIDs[oldWatchlistID] = newWatchlistID
			fmt.Printf("Copied watchlist '%s' (ID: %d -> %d)\n", watchlistName, oldWatchlistID, newWatchlistID)

			// Copy watchlist items
			_, err = tx.Exec(ctx, `
				INSERT INTO watchlistItems (watchlistId, securityId)
				SELECT $1, securityId
				FROM watchlistItems
				WHERE watchlistId = $2`,
				newWatchlistID, oldWatchlistID)

			if err != nil {
				fmt.Printf("WARNING: Error copying watchlist items for watchlist %d: %v\n", oldWatchlistID, err)
			} else {
				fmt.Println("Copied watchlist items")
			}
		}
	}

	// 7. Copy alerts
	_, err = tx.Exec(ctx, `
		INSERT INTO alerts (userId, active, alertType, setupId, algoId, price, direction, securityID)
		SELECT $1, active, alertType, 
			CASE 
				WHEN setupId IN (SELECT setupId FROM setups WHERE userId = 0) 
				THEN (SELECT s2.setupId FROM setups s1 
					JOIN setups s2 ON s1.name = s2.name AND s2.userId = $1 
					WHERE s1.setupId = alerts.setupId AND s1.userId = 0)
				ELSE setupId
			END,
			algoId, price, direction, securityID
		FROM alerts
		WHERE userId = $2`,
		newUserID, defaultGuestID)

	if err != nil {
		fmt.Printf("WARNING: Error copying alerts: %v\n", err)
	} else {
		fmt.Println("Copied alerts")
	}

	// 8. Copy horizontal lines
	_, err = tx.Exec(ctx, `
		INSERT INTO horizontal_lines (userId, securityId, price, color, line_width)
		SELECT $1, securityId, price, color, line_width
		FROM horizontal_lines
		WHERE userId = $2`,
		newUserID, defaultGuestID)

	if err != nil {
		fmt.Printf("WARNING: Error copying horizontal lines: %v\n", err)
	} else {
		fmt.Println("Copied horizontal lines")
	}

	// 9. Copy trades
	_, err = tx.Exec(ctx, `
		INSERT INTO trades (userId, securityId, ticker, tradeDirection, date, status, 
			openQuantity, closedPnL, entry_times, entry_prices, entry_shares, 
			exit_times, exit_prices, exit_shares)
		SELECT $1, securityId, ticker, tradeDirection, date, status, 
			openQuantity, closedPnL, entry_times, entry_prices, entry_shares, 
			exit_times, exit_prices, exit_shares
		FROM trades
		WHERE userId = $2`,
		newUserID, defaultGuestID)

	if err != nil {
		fmt.Printf("WARNING: Error copying trades: %v\n", err)
	} else {
		fmt.Println("Copied trades")
	}

	// 10. Copy notes
	_, err = tx.Exec(ctx, `
		INSERT INTO notes (userId, title, content, category, tags, created_at, updated_at, is_pinned, is_archived)
		SELECT $1, title, content, category, tags, created_at, updated_at, is_pinned, is_archived
		FROM notes
		WHERE userId = $2`,
		newUserID, defaultGuestID)

	if err != nil {
		fmt.Printf("WARNING: Error copying notes: %v\n", err)
	} else {
		fmt.Println("Copied notes")
	}

	fmt.Println("Completed copying guest data")
	return nil
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

// DeleteAccount deletes a user account and all associated data
func DeleteAccount(conn *utils.Conn, rawArgs json.RawMessage) (interface{}, error) {
	fmt.Println("=== DELETE ACCOUNT ATTEMPT STARTED ===")

	// Parse arguments to get confirmation
	var args struct {
		UserID       int    `json:"userId,omitempty"`       // Only needed for public API calls
		AuthType     string `json:"authType,omitempty"`     // Only needed for public API calls
		Confirmation string `json:"confirmation,omitempty"` // Required for both public and private calls
	}

	if err := json.Unmarshal(rawArgs, &args); err != nil {
		fmt.Printf("ERROR: Failed to unmarshal delete account args: %v\n", err)
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	// Check confirmation string for regular delete account
	if args.Confirmation != "DELETE" {
		fmt.Println("ERROR: Missing confirmation text")
		return nil, fmt.Errorf("confirmation must be 'DELETE' to delete account")
	}

	var userID int
	var authType string

	// If userID is provided (for public API), use that instead of the token
	if args.UserID > 0 {
		userID = args.UserID
		authType = args.AuthType

		// Extra verification that public call only works for guest accounts
		if authType != "guest" {
			return nil, fmt.Errorf("cannot delete non-guest account through public API")
		}
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

	// Get auth type for logging purposes if it wasn't provided
	if authType == "" {
		err = tx.QueryRow(ctx, "SELECT auth_type FROM users WHERE userId = $1", userID).Scan(&authType)
		if err != nil {
			fmt.Printf("ERROR: Failed to get user account type: %v\n", err)
			return nil, fmt.Errorf("failed to get user account: %v", err)
		}
	}

	fmt.Printf("Deleting account with ID: %d, type: %s\n", userID, authType)

	// Delete journal entries for the user
	_, err = tx.Exec(ctx, "DELETE FROM journals WHERE userId = $1", userID)
	if err != nil {
		fmt.Printf("ERROR: Failed to delete journal entries: %v\n", err)
		// Continue despite error
	}

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
