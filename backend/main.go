package main

import (
	"backend/data"
	"backend/server"
    "backend/utils"
	"os"
)

func main() {
	args := os.Args
	if len(args) == 1 {
		//test func
        conn, close := utils.InitConn(false)
        defer close()
        err := data.InitTickerDatabase(conn)
        if err != nil {
//            panic(err)
        }
		//data.ManualTest()

	} else if len(args) >= 2 {
		//data.WSTest()
	} else {
		server.StartServer()
	}

}
