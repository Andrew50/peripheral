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
	Path           string `json:"path"`
	Referrer       string `json:"referrer"`
	UserAgent      string `json:"user_agent"`
	IPAddress      string `json:"ip_address"`
	Timestamp      string `json:"timestamp"`
	CloudflareIPv6 string `json:"cloudflare_ipv6"`
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
		INSERT INTO splash_screen_logs(ip_address, user_agent, referrer, path, timestamp, cloudflare_ipv6)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, req.IPAddress, req.UserAgent, req.Referrer, req.Path, req.Timestamp, req.CloudflareIPv6)

	if err != nil {
		// log to console so this issue doesn't go unnoticed lmao
		log.Printf("Failed to insert page view to DB: %v. Data: IP=%s, Path=%s",
			err, req.IPAddress, req.Path)
		return nil, fmt.Errorf("failed to log page view: %w", err)
	}

	return map[string]interface{}{
		"success": true,
		"message": "Page view logged successfully",
	}, nil
}
