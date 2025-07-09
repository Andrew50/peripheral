package main

import (
	"backend/internal/data"
	"backend/internal/server"
	"time"
)

func main() {
	conn, cleanup := data.InitConn(true)
	defer cleanup()

	// Start the scheduler after a 30-second delay to allow the server to finish
	// initializing. Previously this delay was 10 minutes; it is now reduced
	// to improve startup time.
	go func() {
		time.Sleep(30 * time.Second)
		server.StartScheduler(conn)
	}()

	// Start the HTTP server (blocks)
	server.StartServer(conn)
}
