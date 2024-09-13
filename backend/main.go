package main

import (
	"backend/data"
	"backend/server"
    "backend/utils"
	"os"
    "fmt"
)

func main() {
	args := os.Args
	if len(args) > 1 {
        fmt.Println("testing")
		conn, close := utils.InitConn(false)
		defer close()
		err := data.InitTickerDatabase(conn)
        fmt.Printf("ERROR: %v",err)
		if err != nil {
			panic(err)
		}

	} else {
		server.StartServer()
	}

}
