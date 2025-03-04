package main

import (
	"backend/utils"
	"context"
	"fmt"
	"log"
)

func main() {
	log.Println("Checking initialization flag...")

	// Create a database connection
	conn, cleanup := utils.InitConn(true)
	defer cleanup()

	// Check the INITIALIZED flag in Redis
	initialized, err := conn.Cache.Get(context.Background(), "INITIALIZED").Result()
	if err != nil {
		log.Printf("INITIALIZED flag not found in Redis or error occurred: %v", err)
		fmt.Println("Status: System will initialize on next startup")
	} else {
		log.Printf("INITIALIZED flag value: %s", initialized)
		if initialized == "true" {
			fmt.Println("Status: System is already initialized and will not reinitialize on next startup")
		} else {
			fmt.Println("Status: System will initialize on next startup")
		}
	}
}
