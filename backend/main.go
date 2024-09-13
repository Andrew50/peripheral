package main

import (
	"backend/data"
	"backend/server"
	"fmt"
	"os"
)

func main() {
	args := os.Args
	if len(args) > 2 {
		conn, close := utils.InitConn(false)
		defer close()
		utils.PolygonDataToRedis(conn)
	}
	if len(args) > 1 {
		//test func
		conn, close := utils.InitConn(false)
		defer close()
		err := data.InitTickerDatabase(conn)
		fmt.Printf("ERROR: %v", err)
		if err != nil {
			panic(err)
		}

	} else {
		server.StartServer()
	}

}
