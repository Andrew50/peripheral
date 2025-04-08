package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func main() {
	// Get client ID and secret from environment or use existing values
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	if clientID == "" {
		clientID = "831615706061-uojs1kjl4lhe70crmf771s2s2dflejpo.apps.googleusercontent.com"
	}

	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	if clientSecret == "" {
		clientSecret = "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"
	}

	// Set up OAuth2 config with expanded scopes for Google Workspace
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		RedirectURL:  "http://localhost:5173/oauth2callback",
		Scopes: []string{
			"https://mail.google.com/",                      // Full mail access
			"https://www.googleapis.com/auth/gmail.send",    // Send email only
			"https://www.googleapis.com/auth/gmail.compose", // Create and send emails
			"https://www.googleapis.com/auth/gmail.modify",  // Access to modify emails
			"email",   // Get user email address
			"profile", // Get user profile info
		},
	}

	// Channel to receive the authorization code
	codeChan := make(chan string)

	// HTTP handler for the callback
	http.HandleFunc("/oauth2callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "Missing code parameter", http.StatusBadRequest)
			return
		}

		// Send the code to the main goroutine
		codeChan <- code

		// Show success page
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `
			<html>
				<body>
					<h1>Authorization Successful</h1>
					<p>You can close this window now.</p>
				</body>
			</html>
		`)
	})

	// Start HTTP server
	server := &http.Server{Addr: ":5173"}
	go func() {
		fmt.Println("Starting server on http://localhost:5173")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Generate the authorization URL with expanded scopes and offline access
	authURL := config.AuthCodeURL("state",
		oauth2.AccessTypeOffline,                    // Get refresh token
		oauth2.ApprovalForce,                        // Force approval screen (to ensure getting a refresh token)
		oauth2.SetAuthURLParam("prompt", "consent"), // Force consent screen
	)

	fmt.Printf("\n=== IMPORTANT ===\n")
	fmt.Printf("Visit this URL to authorize the application with ALL required scopes:\n%s\n", authURL)
	fmt.Printf("This will create a new refresh token with expanded permissions for Google Workspace.\n")
	fmt.Printf("=================\n\n")

	// Set up graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Wait for authorization code or interrupt
	var code string
	select {
	case code = <-codeChan:
		fmt.Println("Received authorization code")
	case <-stop:
		fmt.Println("Interrupted")
		server.Shutdown(context.Background())
		os.Exit(1)
	}

	// Exchange the code for a token
	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		log.Fatalf("Error exchanging code for token: %v", err)
	}

	// Ensure we got a refresh token
	if token.RefreshToken == "" {
		fmt.Println("\n⚠️  WARNING: No refresh token received!")
		fmt.Println("This can happen if you've already granted these permissions.")
		fmt.Println("Try revoking the app permissions in your Google account and try again.")
		fmt.Println("Go to: https://myaccount.google.com/permissions")
	}

	// Shutdown the server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(ctx)

	// Print the refresh token
	fmt.Printf("\n=== REFRESH TOKEN ===\n%s\n", token.RefreshToken)
	fmt.Println("\nAdd this refresh token to your .env file as GOOGLE_REFRESH_TOKEN")

	// Also write to a file for backup
	if token.RefreshToken != "" {
		err := os.WriteFile("refresh_token.txt", []byte(token.RefreshToken), 0600)
		if err != nil {
			fmt.Printf("Error writing token to file: %v\n", err)
		} else {
			fmt.Println("Refresh token also saved to refresh_token.txt")
		}
	}
}
