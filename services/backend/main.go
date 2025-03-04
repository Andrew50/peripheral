package main

import (
	"backend/server"
	//"backend/utils"
//	"backend/jobs"
	"os"
)

func main() {
	args := os.Args
	if len(args) > 2 {
//		conn, close := utils.InitConn(false)

	//	defer close()
//		jobs.PolygonDataToRedis(conn)
	}
	if len(args) > 1 {
		//test func
//		conn, close := utils.InitConn(false)
//		defer close()

	} else {
		server.StartServer()
	}

}
