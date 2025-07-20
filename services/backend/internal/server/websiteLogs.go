package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"backend/internal/data"
	"backend/internal/services/telegram"
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

// GeoLocationData represents the geolocation data we store and return
type GeoLocationData struct {
	City    string `json:"city"`
	Region  string `json:"region"`
	Country string `json:"country"`
	Org     string `json:"org"`
}

// getOrFetchGeoLocation checks if we have geolocation data for the IP in splash_screen_logs, if not fetches it from ipinfo.io
func getOrFetchGeoLocation(conn *data.Conn, ipAddress string) (*GeoLocationData, error) {
	if ipAddress == "" {
		return nil, nil
	}

	// First, check if we already have geolocation data for this IP in splash_screen_logs
	var geoData GeoLocationData
	err := conn.DB.QueryRow(context.Background(), `
		SELECT geo_city, geo_region, geo_country, geo_org 
		FROM splash_screen_logs 
		WHERE cloudflare_ipv6 = $1 
		AND geo_city IS NOT NULL
		LIMIT 1
	`, ipAddress).Scan(&geoData.City, &geoData.Region, &geoData.Country, &geoData.Org)

	if err == nil {
		return &geoData, nil
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	url := fmt.Sprintf("https://ipinfo.io/%s/json", ipAddress)
	resp, err := client.Get(url)
	if err != nil {
		log.Printf("Failed to fetch geolocation for IP %s: %v", ipAddress, err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("ipinfo.io returned status %d for IP %s", resp.StatusCode, ipAddress)
		return nil, fmt.Errorf("ipinfo.io API returned status %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(&geoData); err != nil {
		log.Printf("Failed to decode ipinfo.io response for IP %s: %v", ipAddress, err)
		return nil, err
	}

	return &geoData, nil
}

// LogSplashScreenView handles splash screen view analytics from frontend server
func LogSplashScreenView(conn *data.Conn, args json.RawMessage) (interface{}, error) {
	var req LogSplashScreenViewArgs
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid splash screen view request: %w", err)
	}

	// Get or fetch geolocation data for the cloudflare IPv6
	geoData, err := getOrFetchGeoLocation(conn, req.CloudflareIPv6)
	if err != nil {
		log.Printf("Failed to get geolocation for IP %s: %v", req.CloudflareIPv6, err)

	}

	// Prepare geolocation values (nullable)
	var city, region, country, org *string
	if geoData != nil {
		city = &geoData.City
		region = &geoData.Region
		country = &geoData.Country
		org = &geoData.Org
	}
	if filterOutWebsiteBots(req, *org) {
		return map[string]interface{}{
			"success": true,
		}, nil
	}

	// Insert and check for recent duplicates in one query
	var wasRecentDuplicate bool
	err = conn.DB.QueryRow(context.Background(), `
		INSERT INTO splash_screen_logs(ip_address, user_agent, referrer, path, timestamp, cloudflare_ipv6, city, region, country, org)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING (
			SELECT COUNT(*) > 1 
			FROM splash_screen_logs 
			WHERE cloudflare_ipv6 = $6
			AND path = $4 
			AND timestamp > NOW() - INTERVAL '30 minutes'
		) AS was_recent_duplicate
	`, req.IPAddress, req.UserAgent, req.Referrer, req.Path, req.Timestamp, req.CloudflareIPv6, city, region, country, org).Scan(&wasRecentDuplicate)

	if err != nil {
		// log to console so this issue doesn't go unnoticed lmao
		log.Printf("Failed to insert page view to DB: %v. Data: IP=%s, Path=%s",
			err, req.IPAddress, req.Path)
		return nil, fmt.Errorf("failed to log page view: %w", err)
	}

	// Only send Telegram message if this wasn't a recent duplicate
	go func() {
		if !wasRecentDuplicate {
			var path string
			if req.Path == "/" { //removing logging for splash
				return
			}
			err = telegram.SendTelegramUserUsageMessage(fmt.Sprintf("User from %s, %s, %s visited %s. Org: %s", *city, *region, *country, path, *org))
			if err != nil {
				log.Printf("Failed to send Telegram message: %v", err)
			}
		}
	}()
	return map[string]interface{}{
		"success": true,
	}, nil
}
func filterOutWebsiteBots(req LogSplashScreenViewArgs, org string) bool {
	if strings.Contains(req.UserAgent, "Twitterbot") {
		return true
	}
	if strings.Contains(org, "Google LLC") {
		return true
	} else if strings.Contains(org, "Microsoft Corporation") {
		return true
	}
	return false
}
