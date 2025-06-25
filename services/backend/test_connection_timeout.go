package main

import (
	"fmt"
	"os"
	"time"

	"backend/internal/data"
)

func main() {
	// Set environment variables to invalid values to test timeout
	os.Setenv("DB_HOST", "invalid-host")
	os.Setenv("REDIS_HOST", "invalid-redis-host")

	fmt.Println("Testing database connection timeout mechanism...")
	start := time.Now()

	defer func() {
		if r := recover(); r != nil {
			elapsed := time.Since(start)
			fmt.Printf("✅ Connection failed as expected after %v: %v\n", elapsed, r)
			if elapsed < 95*time.Second && elapsed > 85*time.Second {
				fmt.Println("✅ Timeout mechanism working correctly (failed within expected timeframe)")
				os.Exit(0)
			} else {
				fmt.Printf("❌ Timeout took unexpected time: %v (expected ~30s)\n", elapsed)
				os.Exit(1)
			}
		}
	}()

	// This should timeout and panic after ~30 seconds
	_, cleanup := data.InitConn(false)
	defer cleanup()

	// If we reach here, something went wrong
	fmt.Println("❌ Connection succeeded unexpectedly")
	os.Exit(1)
}
