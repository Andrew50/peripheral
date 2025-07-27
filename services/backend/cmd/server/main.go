package main

import (
	"backend/internal/data"
	"backend/internal/server"
)

func main() {
	conn, cleanup := data.InitConn(true)
	defer cleanup()
	stopScheduler := server.StartScheduler(conn)
	defer close(stopScheduler)
	server.StartServer(conn)
}
