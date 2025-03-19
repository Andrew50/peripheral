package server

import (
	"backend/internal/server/handlers/health"
	"backend/pkg/utils"
	"fmt"
	"log"
	"net/http"
)

// StartServer initializes and starts the HTTP server
func StartServer() {
	conn := utils.NewConn()

	http.HandleFunc("/api/public", publicHandler(conn))
	http.HandleFunc("/api/private", privateHandler(conn))
	http.HandleFunc("/api/upload", privateUploadHandler(conn))
	http.HandleFunc("/api/queue", queueHandler(conn))
	http.HandleFunc("/api/poll", pollHandler(conn))
	http.HandleFunc("/api/ws", WSHandler(conn))
	http.HandleFunc("/health", health.Handler())

	// Determine port from environment or use default
	port := utils.GetEnvWithDefault("PORT", "8080")
	serverAddr := fmt.Sprintf(":%s", port)

	log.Printf("Starting server on %s", serverAddr)
	log.Fatal(http.ListenAndServe(serverAddr, nil))
}

// Handler functions will be implemented in their respective files
func publicHandler(conn *utils.Conn) http.HandlerFunc {
	// Implementation will be moved here
	return nil
}

func privateHandler(conn *utils.Conn) http.HandlerFunc {
	// Implementation will be moved here
	return nil
}

func privateUploadHandler(conn *utils.Conn) http.HandlerFunc {
	// Implementation will be moved here
	return nil
}

func queueHandler(conn *utils.Conn) http.HandlerFunc {
	// Implementation will be moved here
	return nil
}

func pollHandler(conn *utils.Conn) http.HandlerFunc {
	// Implementation will be moved here
	return nil
}

func WSHandler(conn *utils.Conn) http.HandlerFunc {
	// Implementation will be moved here
	return nil
}
