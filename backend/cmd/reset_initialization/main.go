package main

import (
	"backend/utils"
	"context"
	"fmt"
	"log"
)

func main() {
	log.Println("Resetting initialization flag...")

	// Create a database connection
	conn, cleanup := utils.InitConn(true)
	defer cleanup()

	// Reset the INITIALIZED flag in Redis
	err := conn.Cache.Set(context.Background(), "INITIALIZED", "false", 0).Err()
	if err != nil {
		log.Fatalf("Failed to reset INITIALIZED flag in Redis: %v", err)
	}

	fmt.Println("INITIALIZED flag has been reset to 'false'. The system will initialize again on next startup.")
}
