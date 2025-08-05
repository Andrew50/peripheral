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
	"backend/internal/app/pricing"
	"backend/internal/services/telegram"

	"github.com/golang-jwt/jwt/v4"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// JWT secret is mandatory – crash early if it is missing so that bad tokens are never issued.
var privateKey = []byte(mustGetEnv("JWT_SECRET"))

// mustGetEnv behaves like os.Getenv but terminates the process if the variable is empty.
func mustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("%s environment variable is required but not set", key)
	}
	return v
}

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
}

// SignupArgs represents a structure for handling SignupArgs data.
type SignupArgs struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	InviteCode string `json:"inviteCode,omitempty"` // Optional invite code for trial subscriptions
}

// Signup performs operations related to Signup functionality.
func Signup(conn *data.Conn, rawArgs json.RawMessage) (interface{}, error) {
	log.Println("Signup attempt started")

	var a SignupArgs
	if err := json.Unmarshal(rawArgs, &a); err != nil {
		return nil, fmt.Errorf("%w: invalid signup args: %v", ErrInvalidInput, err)
	}

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
		return nil, fmt.Errorf("%w: email already registered", ErrEmailExists)
	}

	// Handle invite code if provided
	var invite *data.Invite
	if a.InviteCode != "" {
		log.Printf("Processing signup with invite code: %s", a.InviteCode)

		// Verify the invite code is valid and unused
		invite, err = data.GetInviteByCode(conn, a.InviteCode)
		if err != nil {
			log.Printf("ERROR: Invalid invite code %s: %v", a.InviteCode, err)
			return nil, fmt.Errorf("invalid invite code")
		}

		if invite.Used {
			log.Printf("ERROR: Invite code %s already used", a.InviteCode)
			return nil, fmt.Errorf("invite code already used")
		}
	}

	// Insert new user with auth_type='password' and record invite code if present
	var userID int
	var inviteCodeVal string
	if invite != nil {
		inviteCodeVal = invite.Code
	} else {
		inviteCodeVal = ""
	}

	// Note: invite_code_used column will accept NULL/empty string when no invite was used
	err = tx.QueryRow(ctx,
		"INSERT INTO users (email, password, auth_type, invite_code_used) VALUES ($1, $2, $3, NULLIF($4, '')) RETURNING userId",
		a.Email, a.Password, "password", inviteCodeVal).Scan(&userID)
	if err != nil {
		log.Printf("ERROR: Failed to create user: %v", err)
		return nil, fmt.Errorf("error creating user: %v", err)
	}

	// Handle invite marking within the transaction before committing
	if invite != nil {
		// Mark invite as used within the transaction
		if err := data.MarkInviteUsedTx(ctx, tx, invite.Code); err != nil {
			log.Printf("ERROR: Failed to mark invite %s as used in transaction: %v", invite.Code, err)
			return nil, fmt.Errorf("failed to process invite: %v", err)
		}
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		log.Printf("ERROR: Failed to commit signup transaction: %v", err)
		return nil, fmt.Errorf("error committing signup transaction: %v", err)
	}
	txClosed = true

	// Handle invite-based trial subscription or regular free plan
	if invite != nil {
		log.Printf("Creating trial subscription for user %d with invite %s for plan %s", userID, invite.Code, invite.PlanName)

		// Get the Stripe price ID for the plan (defaulting to monthly billing for trials)
		priceID, err := pricing.GetStripePriceIDForProduct(conn, invite.PlanName, "monthly")
		if err != nil {
			log.Printf("ERROR: Failed to get price ID for plan %s: %v", invite.PlanName, err)
			return nil, fmt.Errorf("invalid plan in invite: %v", err)
		}

		// Create trial subscription via Stripe
		customer, subscription, err := StripeCreateTrialSubscription(userID, priceID, a.Email, invite.TrialDays)
		if err != nil {
			log.Printf("ERROR: Failed to create trial subscription for user %d: %v", userID, err)
			// Note: User is already created, so we continue but log the error
			// In production, you might want to handle this differently
		} else {
			log.Printf("Created trial subscription %s for user %d", subscription.ID, userID)

			// Persist Stripe identifiers and initial trial status in our database so that
			// later webhook events (e.g. invoice.payment_succeeded) can correctly map
			// back to this user.
			ctxStripe, cancelStripe := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancelStripe()

			_, dbErr := conn.DB.Exec(ctxStripe, `
				UPDATE users
				SET stripe_customer_id     = $1,
				    stripe_subscription_id = $2,
				    subscription_status    = 'trialing',
				    subscription_plan      = $3,
				    invite_code_used       = $4,
				    updated_at             = CURRENT_TIMESTAMP
				WHERE userId = $5`,
				customer.ID,
				subscription.ID,
				invite.PlanName,
				invite.Code,
				userID,
			)
			if dbErr != nil {
				log.Printf("ERROR: Failed to persist Stripe IDs for trial user %d: %v", userID, dbErr)
			}
		}

		// Note: Invite is already marked as used within the signup transaction

		// For trial users, we might want to allocate different credits or skip the free plan
		// For now, we'll still allocate free plan credits as a baseline
		if err := limits.UpdateUserCreditsForPlan(conn, userID, "Free"); err != nil {
			log.Printf("ERROR: Failed to allocate free credits for trial user %d: %v", userID, err)
			// Continue, as trial subscription is the main benefit
		}
	} else {
		// Allocate free plan credits for the new user (idempotent if called again)
		if err := limits.UpdateUserCreditsForPlan(conn, userID, "Free"); err != nil {
			// Propagate the error so signup fails clearly if free credits cannot be allocated
			return nil, fmt.Errorf("failed to allocate free credits: %v", err)
		}
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
		return nil, fmt.Errorf("%w: login invalid args: %v", ErrInvalidInput, err)
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
		`SELECT userId, password, profile_picture, auth_type
		 FROM users WHERE email=$1`,
		a.Email).Scan(&userID, &storedPw, &profilePicture, &authType)

	switch {
	case err == pgx.ErrNoRows:
		log.Printf("Login failed: No user found for email: %s", a.Email)
		return nil, fmt.Errorf("%w", ErrIncorrectEmail)
	case err != nil:
		log.Printf("ERROR: Database query failed during login check: %v", err)
		return nil, fmt.Errorf("database error: %v", err)
	}

	// 2) Is this a Google-only account?
	if authType == "google" {
		log.Printf("Login failed: Google-only user attempting password login for email: %s", a.Email)
		return nil, fmt.Errorf("%w", ErrGoogleAuthRequired)
	}

	// 3) Wrong password? (Only check for 'password' or 'both' auth types)
	if authType == "password" || authType == "both" {
		if storedPw != a.Password {
			log.Printf("Login failed: Password mismatch for email: %s", a.Email)
			return nil, fmt.Errorf("%w", ErrIncorrectPassword)
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

	if err := telegram.SendTelegramUserUsageMessage(fmt.Sprintf("%s logged in to the website", a.Email)); err != nil {
		log.Printf("Warning: failed to send telegram notification: %v", err)
	}
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
}

// GoogleLogin performs operations related to GoogleLogin functionality.
func GoogleLogin(_ *data.Conn, rawArgs json.RawMessage) (interface{}, error) {
	var args struct {
		RedirectOrigin string `json:"redirectOrigin"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("%w: invalid args: %v", ErrInvalidInput, err)
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
		Code       string `json:"code"`
		State      string `json:"state"`
		InviteCode string `json:"inviteCode,omitempty"` // Optional invite code
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("%w: invalid args: %v", ErrInvalidInput, err)
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

	// Handle invite code if provided
	var invite *data.Invite
	if args.InviteCode != "" {
		log.Printf("Processing Google signup with invite code: %s", args.InviteCode)

		// Verify the invite code is valid and unused
		invite, err = data.GetInviteByCode(conn, args.InviteCode)
		if err != nil {
			log.Printf("ERROR: Invalid invite code %s: %v", args.InviteCode, err)
			return nil, fmt.Errorf("invalid invite code")
		}

		if invite.Used {
			log.Printf("ERROR: Invite code %s already used", args.InviteCode)
			return nil, fmt.Errorf("invite code already used")
		}
	}

	// Check if user exists, if not create new user
	var userID int
	var authType string

	err = conn.DB.QueryRow(context.Background(),
		"SELECT userId, auth_type FROM users WHERE email = $1",
		googleUser.Email).Scan(&userID, &authType)

	if err != nil {
		// User doesn't exist, create new user with auth_type='google'
		// Start a transaction for the Google user creation process
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		tx, err := conn.DB.Begin(ctx)
		if err != nil {
			log.Printf("ERROR: Failed to start Google user creation transaction: %v", err)
			return nil, fmt.Errorf("error starting transaction: %v", err)
		}

		// Ensure transaction is either committed or rolled back
		var txClosed bool
		defer func() {
			if !txClosed && tx != nil {
				log.Println("Rolling back Google user creation transaction due to error")
				_ = tx.Rollback(context.Background())
			}
		}()

		var inviteCodeVal string
		if invite != nil {
			inviteCodeVal = invite.Code
		} else {
			inviteCodeVal = ""
		}

		err = tx.QueryRow(ctx,
			"INSERT INTO users (password, email, google_id, profile_picture, auth_type, invite_code_used) VALUES ($1, $2, $3, $4, $5, NULLIF($6, '')) RETURNING userId",
			"", googleUser.Email, googleUser.ID, googleUser.Picture, "google", inviteCodeVal).Scan(&userID)
		if err != nil {
			log.Printf("ERROR: Failed to create Google user: %v", err)

			// Check if this is a constraint violation error
			if pgErr, ok := err.(*pgconn.PgError); ok {
				// Check for unique constraint violations
				if pgErr.Code == "23505" { // unique_violation
					if strings.Contains(pgErr.ConstraintName, "users_email_key") {
						log.Printf("Email already exists for Google user: %s", googleUser.Email)
						return nil, fmt.Errorf("%w", ErrEmailExists)
					}
				}
			}

			return nil, fmt.Errorf("failed to create user: %v", err)
		}

		// Handle invite marking within the transaction before committing
		if invite != nil {
			// Mark invite as used within the transaction
			if err := data.MarkInviteUsedTx(ctx, tx, invite.Code); err != nil {
				log.Printf("ERROR: Failed to mark invite %s as used in Google transaction: %v", invite.Code, err)
				return nil, fmt.Errorf("failed to process invite: %v", err)
			}
		}

		// Commit the transaction
		if err := tx.Commit(ctx); err != nil {
			log.Printf("ERROR: Failed to commit Google user creation transaction: %v", err)
			return nil, fmt.Errorf("error committing Google user creation transaction: %v", err)
		}
		txClosed = true

		// Handle invite-based trial subscription or regular free plan
		if invite != nil {
			log.Printf("Creating trial subscription for Google user %d with invite %s for plan %s", userID, invite.Code, invite.PlanName)

			// Get the Stripe price ID for the plan (defaulting to monthly billing for trials)
			priceID, err := pricing.GetStripePriceIDForProduct(conn, invite.PlanName, "monthly")
			if err != nil {
				log.Printf("ERROR: Failed to get price ID for plan %s: %v", invite.PlanName, err)
				return nil, fmt.Errorf("invalid plan in invite: %v", err)
			}

			// Create trial subscription via Stripe
			customer, subscription, err := StripeCreateTrialSubscription(userID, priceID, googleUser.Email, invite.TrialDays)
			if err != nil {
				log.Printf("ERROR: Failed to create trial subscription for Google user %d: %v", userID, err)
				// Note: User is already created, so we continue but log the error
			} else {
				log.Printf("Created trial subscription %s for Google user %d", subscription.ID, userID)

				// Persist Stripe identifiers and initial trial status in our database
				ctxStripe, cancelStripe := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancelStripe()

				_, dbErr := conn.DB.Exec(ctxStripe, `
					UPDATE users
					SET stripe_customer_id     = $1,
					    stripe_subscription_id = $2,
					    subscription_status    = 'trialing',
					    subscription_plan      = $3,
					    invite_code_used       = $4,
					    updated_at             = CURRENT_TIMESTAMP
					WHERE userId = $5`,
					customer.ID,
					subscription.ID,
					invite.PlanName,
					invite.Code,
					userID,
				)
				if dbErr != nil {
					log.Printf("ERROR: Failed to persist Stripe IDs for trial Google user %d: %v", userID, dbErr)
				}
			}

			// Note: Invite is already marked as used within the Google user creation transaction

			// For trial users, we might want to allocate different credits or skip the free plan
			// For now, we'll still allocate free plan credits as a baseline
			if err := limits.UpdateUserCreditsForPlan(conn, userID, "Free"); err != nil {
				log.Printf("ERROR: Failed to allocate free credits for trial Google user %d: %v", userID, err)
				// Continue, as trial subscription is the main benefit
			}
		} else {
			// Allocate free plan credits for the newly created Google user
			if err := limits.UpdateUserCreditsForPlan(conn, userID, "Free"); err != nil {
				log.Printf("ERROR: Failed to allocate free credits for Google user %d: %v", userID, err)
				return nil, fmt.Errorf("failed to allocate free credits: %v", err)
			}
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

			// Check if this is a constraint violation error
			if pgErr, ok := err.(*pgconn.PgError); ok {
				if pgErr.Code == "23505" { // unique_violation
					if strings.Contains(pgErr.ConstraintName, "users_google_id") {
						log.Printf("Google ID already linked to another account: %s", googleUser.ID)
						return nil, fmt.Errorf("this Google account is already linked to another user")
					}
				}
			}

			return nil, fmt.Errorf("failed to update user: %v", err)
		}
		log.Printf("Linked Google account to existing user ID: %d", userID)

		// Handle invite code for existing user linking Google account
		if invite != nil {
			log.Printf("Processing invite %s for existing user %d linking Google account", invite.Code, userID)

			// Get the Stripe price ID for the plan (defaulting to monthly billing for trials)
			priceID, err := pricing.GetStripePriceIDForProduct(conn, invite.PlanName, "monthly")
			if err != nil {
				log.Printf("ERROR: Failed to get price ID for plan %s: %v", invite.PlanName, err)
				return nil, fmt.Errorf("invalid plan in invite: %v", err)
			}

			// Create trial subscription via Stripe
			customer, subscription, err := StripeCreateTrialSubscription(userID, priceID, googleUser.Email, invite.TrialDays)
			if err != nil {
				log.Printf("ERROR: Failed to create trial subscription for existing user %d: %v", userID, err)
				// Note: User linking is already complete, so we continue but log the error
			} else {
				log.Printf("Created trial subscription %s for existing user %d", subscription.ID, userID)

				// Persist Stripe identifiers and initial trial status in our database
				ctxStripe, cancelStripe := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancelStripe()

				_, dbErr := conn.DB.Exec(ctxStripe, `
					UPDATE users
					SET stripe_customer_id     = $1,
					    stripe_subscription_id = $2,
					    subscription_status    = 'trialing',
					    subscription_plan      = $3,
					    invite_code_used       = $4,
					    updated_at             = CURRENT_TIMESTAMP
					WHERE userId = $5`,
					customer.ID,
					subscription.ID,
					invite.PlanName,
					invite.Code,
					userID,
				)
				if dbErr != nil {
					log.Printf("ERROR: Failed to persist Stripe IDs for existing user %d: %v", userID, dbErr)
				}
			}

			// Mark invite as used (non-transactional for existing user linking)
			if err := data.MarkInviteUsed(conn, invite.Code); err != nil {
				log.Printf("ERROR: Failed to mark invite %s as used: %v", invite.Code, err)
				// This is a critical error for existing user linking - the invite should be marked as used
				return nil, fmt.Errorf("failed to mark invite as used: %v", err)
			}
		}
	}

	// Create JWT token
	jwtToken, err := createToken(userID)
	if err != nil {
		log.Printf("ERROR: Failed to create token for Google user ID %d: %v", userID, err)
		return nil, fmt.Errorf("failed to create token: %v", err)
	}

	if err := telegram.SendTelegramUserUsageMessage(fmt.Sprintf("%s logged in to the website [google auth]", googleUser.Email)); err != nil {
		log.Printf("Warning: failed to send telegram notification: %v", err)
	}
	return GoogleLoginResponse{
		Token:      jwtToken,
		ProfilePic: googleUser.Picture,
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
func DeleteAccount(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	log.Println("Delete account attempt started")

	// Validate userID
	if userID <= 0 {
		return nil, fmt.Errorf("%w: invalid user ID", ErrInvalidInput)
	}

	// Parse arguments to get confirmation
	var args struct {
		Confirmation string `json:"confirmation,omitempty"` // Required for both public and private calls
	}

	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("%w: invalid args: %v", ErrInvalidInput, err)
	}

	// Check confirmation string for regular delete account
	if args.Confirmation != "DELETE" {
		return nil, fmt.Errorf("%w: confirmation must be 'DELETE'", ErrInvalidInput)
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

	// Get auth type for logging purposes
	var authType string
	err = tx.QueryRow(ctx, "SELECT auth_type FROM users WHERE userId = $1", userID).Scan(&authType)
	if err != nil {
		log.Printf("ERROR: Failed to get user account type for deletion: %v", err)
		return nil, fmt.Errorf("failed to get user account: %v", err)
	}

	log.Printf("Deleting account with ID: %d, type: %s", userID, authType)

	// Delete watchlists
	_, err = tx.Exec(ctx, "DELETE FROM watchlists WHERE userId = $1", userID)
	if err != nil {
		log.Printf("ERROR: Failed to delete watchlists for user %d: %v", userID, err)
		return nil, err
	}

	// Delete studies
	_, err = tx.Exec(ctx, "DELETE FROM studies WHERE userId = $1", userID)
	if err != nil {
		log.Printf("ERROR: Failed to delete studies for user %d: %v", userID, err)
		return nil, err
	}

	// Delete strategies
	_, err = tx.Exec(ctx, "DELETE FROM strategies WHERE userId = $1", userID)
	if err != nil {
		log.Printf("ERROR: Failed to delete strategies for user %d: %v", userID, err)
		return nil, err
	}

	// Delete horizontal lines
	_, err = tx.Exec(ctx, "DELETE FROM horizontal_lines WHERE userId = $1", userID)
	if err != nil {
		log.Printf("ERROR: Failed to delete horizontal lines for user %d: %v", userID, err)
		return nil, err
	}

	// Delete trades
	_, err = tx.Exec(ctx, "DELETE FROM trades WHERE userId = $1", userID)
	if err != nil {
		log.Printf("ERROR: Failed to delete trades for user %d: %v", userID, err)
		return nil, err
	}

	// Delete trade executions
	_, err = tx.Exec(ctx, "DELETE FROM trade_executions WHERE userId = $1", userID)
	if err != nil {
		log.Printf("ERROR: Failed to delete trade executions for user %d: %v", userID, err)
		return nil, err
	}

	// Delete alerts
	_, err = tx.Exec(ctx, "DELETE FROM alerts WHERE userId = $1", userID)
	if err != nil {
		log.Printf("ERROR: Failed to delete alerts for user %d: %v", userID, err)
		return nil, err
	}

	// Delete conversations
	_, err = tx.Exec(ctx, "DELETE FROM conversations WHERE userId = $1", userID)
	if err != nil {
		log.Printf("ERROR: Failed to delete conversations for user %d: %v", userID, err)
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

// CreateInviteArgs represents arguments for creating an invite
type CreateInviteArgs struct {
	PlanName  string `json:"planName"`
	TrialDays int    `json:"trialDays"`
}

// CreateInviteResponse represents the response for creating an invite
type CreateInviteResponse struct {
	Code       string `json:"code"`
	PlanName   string `json:"planName"`
	TrialDays  int    `json:"trialDays"`
	InviteLink string `json:"inviteLink"`
}

// CreateInvite creates a new invite code (admin only function)
func CreateInvite(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	log.Printf("CreateInvite called by user %d", userID)

	var args CreateInviteArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("%w: invalid args: %v", ErrInvalidInput, err)
	}

	// Validate required fields
	if args.PlanName == "" {
		return nil, fmt.Errorf("%w: planName is required", ErrInvalidInput)
	}

	// Set default trial days if not provided
	if args.TrialDays <= 0 {
		args.TrialDays = 30
	}

	// Create the invite using the data layer
	invite, err := data.CreateInvite(conn, args.PlanName, args.TrialDays)
	if err != nil {
		log.Printf("Error creating invite: %v", err)
		return nil, fmt.Errorf("failed to create invite: %v", err)
	}

	// Construct invite link (frontend URL can be made configurable via env var)
	frontendURL := getEnvOrDefault("FRONTEND_URL", "https://peripheral.io")
	inviteLink := fmt.Sprintf("%s/invite/%s", frontendURL, invite.Code)

	response := CreateInviteResponse{
		Code:       invite.Code,
		PlanName:   args.PlanName,
		TrialDays:  invite.TrialDays,
		InviteLink: inviteLink,
	}

	log.Printf("Successfully created invite %s for plan %s (%d trial days)", invite.Code, args.PlanName, invite.TrialDays)
	return response, nil
}

// ValidateInvite validates an invite code and returns its details so the frontend can confirm
// the code before showing the signup modal. It is exposed via the /public endpoint.
func ValidateInvite(conn *data.Conn, rawArgs json.RawMessage) (interface{}, error) {
	var args struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("%w: invalid args: %v", ErrInvalidInput, err)
	}

	if args.Code == "" {
		return nil, fmt.Errorf("%w: code is required", ErrInvalidInput)
	}

	invite, err := data.GetInviteByCode(conn, args.Code)
	if err != nil {
		return nil, fmt.Errorf("invalid invite code")
	}

	if invite.Used {
		return nil, fmt.Errorf("invite code already used")
	}

	// Return minimal details the UI may show.
	return map[string]interface{}{
		"code":      invite.Code,
		"planName":  invite.PlanName,
		"trialDays": invite.TrialDays,
	}, nil
}
