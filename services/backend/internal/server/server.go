package server

import (
	httpServer "backend/internal/server/http"
)

// StartServer initializes and starts the server
func StartServer() {
	httpServer.StartServer()
}
