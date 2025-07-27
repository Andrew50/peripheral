package main

import (
	"backend/internal/data"
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	conn, cleanup := data.InitConn(true)
	defer cleanup()

	fmt.Println("Connected successfully!")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Get all users with unhashed passwords
	rows, err := conn.DB.Query(ctx, `SELECT userId, password FROM users`)
	if err != nil {
		log.Fatalf("Query error: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var userID int
		var password string

		err := rows.Scan(&userID, &password)
		if err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}

		// Skip if already hashed
		if strings.HasPrefix(password, "$2b$") {
			continue
		}

		hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("Hashing error for user %d: %v", userID, err)
			continue
		}

		_, err = conn.DB.Exec(ctx, `UPDATE users SET password = $1 WHERE userId = $2`, string(hashed), userID)
		if err != nil {
			log.Printf("Update error for user %d: %v", userID, err)
		} else {
			fmt.Printf("User %d password hashed and updated.\n", userID)
		}
	}

	if err = rows.Err(); err != nil {
		log.Printf("Row error: %v", err)
	}

	fmt.Println("Migration complete.")
}
