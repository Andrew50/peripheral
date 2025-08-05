// Package jobs provides email sending functionality using OAuth2 authentication
package jobs

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// getAccessToken obtains an access token using the refresh token
func getAccessToken() (string, error) {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	refreshToken := os.Getenv("GOOGLE_REFRESH_TOKEN")

	if clientID == "" || clientSecret == "" || refreshToken == "" {
		return "", fmt.Errorf("missing OAuth2 credentials (GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, GOOGLE_REFRESH_TOKEN)")
	}

	config := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		Scopes: []string{
			"https://mail.google.com/",
			"https://www.googleapis.com/auth/gmail.send",
		},
	}

	token := &oauth2.Token{
		RefreshToken: refreshToken,
	}

	tokenSource := config.TokenSource(context.Background(), token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return "", err
	}

	return newToken.AccessToken, nil
}

// xoauth2Auth implements smtp.Auth for OAuth2
type xoauth2Auth struct {
	username string
	token    string
}

func (a *xoauth2Auth) Start(_ *smtp.ServerInfo) (string, []byte, error) {
	auth := fmt.Sprintf("user=%s\001auth=Bearer %s\001\001", a.username, a.token)
	return "XOAUTH2", []byte(auth), nil
}

func (a *xoauth2Auth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		return nil, fmt.Errorf("unexpected server challenge: %s", string(fromServer))
	}
	return nil, nil
}

// SendEmail sends an email with the given subject and body to the specified email address
// using OAuth2 authentication over SSL (port 465)
func SendEmail(to, subject, body string) error {
	// Get email configuration
	smtpHost := getEnvWithDefault("SMTP_HOST", "smtp.gmail.com")
	smtpPort := getEnvWithDefault("SMTP_PORT", "465")
	from := os.Getenv("EMAIL_FROM_ADDRESS")

	if from == "" {
		return fmt.Errorf("EMAIL_FROM_ADDRESS environment variable not set")
	}

	// Get OAuth2 access token
	accessToken, err := getAccessToken()
	if err != nil {
		return fmt.Errorf("failed to get access token: %v", err)
	}

	// Create email message with HTML support
	msg := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"\r\n"+
		"%s\r\n", from, to, subject, body)

	// Create SSL connection
	conn, err := tls.Dial("tcp", smtpHost+":"+smtpPort, &tls.Config{
		ServerName: smtpHost,
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		return fmt.Errorf("TLS connection error: %v", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			fmt.Printf("error closing connection: %v\n", err)
		}
	}()

	// Create SMTP client
	client, err := smtp.NewClient(conn, smtpHost)
	if err != nil {
		return fmt.Errorf("SMTP client creation error: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			fmt.Printf("error closing SMTP client: %v\n", err)
		}
	}()

	// Authenticate with OAuth2
	auth := &xoauth2Auth{
		username: from,
		token:    accessToken,
	}

	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("authentication error: %v", err)
	}

	// Set the sender and recipient
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("MAIL FROM error: %v", err)
	}

	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("RCPT TO error: %v", err)
	}

	// Send the email body
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA command error: %v", err)
	}

	_, err = writer.Write([]byte(msg))
	if err != nil {
		return fmt.Errorf("error writing email body: %v", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("error closing writer: %v", err)
	}

	// Close the connection
	err = client.Quit()
	if err != nil {
		return fmt.Errorf("error closing connection: %v", err)
	}

	return nil
}

// getEnvWithDefault returns the value of an environment variable or a default value if not set
func getEnvWithDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
