package jobs

import (
	"backend/socket"
	"backend/telegram"
	"backend/utils"
	"fmt"
	"time"
)

var eOpenRun = false
var eCloseRun = false

func StartScheduler(conn *utils.Conn) chan struct{} {
	go initialize(conn)
	location, err := time.LoadLocation("EST")
	ticker := time.NewTicker(1 * time.Minute)
	quit := make(chan struct{})
	if err != nil {
		panic(fmt.Errorf("219jv %v", err))
	}
	go func() {
		for {
			select {
			case <-ticker.C:
				now := time.Now().In(location)
				eventLoop(now, conn)
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
	return quit
}

func initialize(conn *utils.Conn) {
	socket.StartPolygonWS(conn)
	err := telegram.InitBot()
	if err != nil {
		fmt.Println("issue init telegram")
	}
	telegram.SendMessageInternal("TESTING!", -1002428678944)
}

func eventLoop(now time.Time, conn *utils.Conn) {
	year, month, day := now.Date()
	eOpen := time.Date(year, month, day, 4, 0, 0, 0, now.Location())
	eClose := time.Date(year, month, day, 16, 0, 0, 0, now.Location())
	//open := time.Date(year, month, day, 9, 30, 0, 0, now.Location())
	//close_ := time.Date(year, month, day, 16, 0, 0, 0, now.Location())
	if !eOpenRun && now.After(eOpen) && now.Before(eClose) {
		fmt.Println("running open update")
		socket.StartPolygonWS(conn)
		pushJournals(conn, year, month, day)
		eOpenRun = true
		eCloseRun = false
	}
	if !eCloseRun && now.After(eClose) {
		fmt.Println("running close update")
		updateSecurities(conn, false)
		eOpenRun = false
		eCloseRun = true
	}
}
