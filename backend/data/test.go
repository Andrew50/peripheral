package data

import (
	"log"
)

func ManualTest() {
	conn, close := InitConn(false)
	defer close()
	err := initTickerDatabase(conn)

	if err != nil {
		log.Fatal(err)
	}
}
