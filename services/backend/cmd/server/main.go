package main

import (
    "backend/internal/server"
    "backend/internal/data"
)

func main() {
	conn, cleanup := data.InitConn(true)
	defer cleanup()
	stopScheduler := server.StartScheduler(conn)
	defer close(stopScheduler)
	server.StartServer(conn)
}
