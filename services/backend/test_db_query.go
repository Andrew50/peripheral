package main

import (
	"backend/internal/data"
	"context"
	"fmt"
	"os"
	"time"
)

func main() {
	// Set environment variable to run in container
	if err := os.Setenv("IN_CONTAINER", "true"); err != nil {
		fmt.Printf("Failed to set environment variable: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Connecting to database...")

	// Initialize connection using the existing data package
	conn, cleanup := data.InitConn(true)
	defer cleanup()

	fmt.Println("Connected successfully!")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Test total count first
	var totalRecords int64
	err := conn.DB.QueryRow(ctx, "SELECT COUNT(*) FROM ohlcv_1d;").Scan(&totalRecords)
	if err != nil {
		fmt.Printf("Failed total count query: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Total OHLCV Records: %d\n", totalRecords)

	// Get date range more efficiently
	var earliestDate time.Time
	err = conn.DB.QueryRow(ctx, "SELECT timestamp FROM ohlcv_1d ORDER BY timestamp LIMIT 1;").Scan(&earliestDate)
	if err != nil {
		fmt.Printf("Failed to get earliest date: %v\n", err)
	} else {
		fmt.Printf("Earliest Date: %s\n", earliestDate.Format("2006-01-02"))
	}

	var latestDate time.Time
	err = conn.DB.QueryRow(ctx, "SELECT timestamp FROM ohlcv_1d ORDER BY timestamp DESC LIMIT 1;").Scan(&latestDate)
	if err != nil {
		fmt.Printf("Failed to get latest date: %v\n", err)
	} else {
		fmt.Printf("Latest Date: %s\n", latestDate.Format("2006-01-02"))
	}

	// Count unique securities
	var uniqueSecurities int64
	err = conn.DB.QueryRow(ctx, "SELECT COUNT(DISTINCT securityid) FROM ohlcv_1d;").Scan(&uniqueSecurities)
	if err != nil {
		fmt.Printf("Failed to count unique securities: %v\n", err)
	} else {
		fmt.Printf("Unique Securities: %d\n", uniqueSecurities)
	}

	// Check for MRNA specifically with simpler queries
	fmt.Println("\nMRNA Specific Data:")

	var mrnaRecords int64
	err = conn.DB.QueryRow(ctx, `
		SELECT COUNT(*) 
		FROM ohlcv_1d o
		JOIN securities s ON o.securityid = s.securityid 
		WHERE s.ticker = 'MRNA';
	`).Scan(&mrnaRecords)
	if err != nil {
		fmt.Printf("Failed to query MRNA count: %v\n", err)
	} else {
		fmt.Printf("MRNA Total Records: %d\n", mrnaRecords)
	}

	if mrnaRecords > 0 {
		var mrnaEarliest time.Time
		err = conn.DB.QueryRow(ctx, `
			SELECT timestamp 
			FROM ohlcv_1d o
			JOIN securities s ON o.securityid = s.securityid 
			WHERE s.ticker = 'MRNA'
			ORDER BY timestamp LIMIT 1;
		`).Scan(&mrnaEarliest)
		if err != nil {
			fmt.Printf("Failed to get MRNA earliest: %v\n", err)
		} else {
			fmt.Printf("MRNA Earliest Date: %s\n", mrnaEarliest.Format("2006-01-02"))
		}

		var mrnaLatest time.Time
		err = conn.DB.QueryRow(ctx, `
			SELECT timestamp 
			FROM ohlcv_1d o
			JOIN securities s ON o.securityid = s.securityid 
			WHERE s.ticker = 'MRNA'
			ORDER BY timestamp DESC LIMIT 1;
		`).Scan(&mrnaLatest)
		if err != nil {
			fmt.Printf("Failed to get MRNA latest: %v\n", err)
		} else {
			fmt.Printf("MRNA Latest Date: %s\n", mrnaLatest.Format("2006-01-02"))
		}
	}
}
