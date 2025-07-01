package main

import (
	"backend/internal/data"
	"backend/internal/metrics"
	"backend/internal/server"
)

func main() {
	conn, cleanup := data.InitConn(true)
	defer cleanup()

	_ = metrics.FunctionCalls // ensures metrics are created

	stopScheduler := server.StartScheduler(conn)
	defer close(stopScheduler)
	server.StartServer(conn)
}
