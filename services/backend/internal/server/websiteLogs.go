package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"backend/internal/data"
)

// LogSplashScreenViewArgs represents the data sent from frontend server for page view logging
type LogSplashScreenViewArgs struct {
	Path      string `json:"path"`
	Referrer  string `json:"referrer"`
	UserAgent string `json:"user_agent"`
	IPAddress string `json:"ip_address"`
	Timestamp string `json:"timestamp"`
}

// LogSplashScreenView handles splash screen view analytics from frontend server
func LogSplashScreenView(conn *data.Conn, args json.RawMessage) (interface{}, error) {
	var req LogSplashScreenViewArgs
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid splash screen view request: %w", err)
	}

	// Basic validation
	if req.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	// Insert into splash_screen_logs table
	_, err := conn.DB.Exec(context.Background(), `
		INSERT INTO splash_screen_logs(ip_address, user_agent, referrer, path, timestamp)
		VALUES ($1, $2, $3, $4, $5)
	`, req.IPAddress, req.UserAgent, req.Referrer, req.Path, req.Timestamp)

	if err != nil {
		// Log error but also log to console as fallback
		log.Printf("Failed to insert page view to DB: %v. Data: IP=%s, Path=%s",
			err, req.IPAddress, req.Path)
		return nil, fmt.Errorf("failed to log page view: %w", err)
	}

	// Also log to console for debugging (can remove in production)
	log.Printf("Page view logged: IP=%s, Path=%s, UserAgent=%s, Referrer=%s",
		req.IPAddress, req.Path, req.UserAgent, req.Referrer)
	return map[string]interface{}{
		"success": true,
		"message": "Page view logged successfully",
	}, nil
}
