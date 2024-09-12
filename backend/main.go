package main

import (
	"api/data"
	"api/server"
	"os"
)

func main() {
	args := os.Args
	if len(args) > 1 {
		//test func
		data.ManualTest()

	} else if len(args) >= 2 {
		data.WSTest()
	} else {
		server.StartServer()
	}

}
