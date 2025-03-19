package main

import (
	"backend/internal/server"
)

func main() {
	// Application configuration is handled through environment variables
	// Command line arguments may be added in the future if needed
	server.StartServer()
}
