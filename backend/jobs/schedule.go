package jobs

import (
	"backend/socket"
	//"backend/alerts"
	"backend/alerts"
	"backend/utils"
	"fmt"
	"sync"
	"time"
)

var eOpenRun = false
var eCloseRun = false

var (
	polygonInitialized bool
	polygonInitMutex   sync.Mutex
	alertsInitialized  bool
	alertsInitMutex    sync.Mutex
)

func StartScheduler(conn *utils.Conn) chan struct{} {
	//go initialize(conn)
	//eventLoop(time.Now(), conn)
	location, err := time.LoadLocation("EST")
	go eventLoop(time.Now().In(location), conn)
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

	alertsInitMutex.Lock()
	if !alertsInitialized {
		err := alerts.StartAlertLoop(conn)
		if err != nil {
			fmt.Println("schedule issue", err)
		}
		alertsInitialized = true
	}
	alertsInitMutex.Unlock()
	polygonInitMutex.Lock()
	if !polygonInitialized {
		socket.StartPolygonWS(conn)
		polygonInitialized = true
	}
	polygonInitMutex.Unlock()
}

func eventLoop(now time.Time, conn *utils.Conn) {
	year, month, day := now.Date()
	eOpen := time.Date(year, month, day, 4, 0, 0, 0, now.Location())
	eClose := time.Date(year, month, day, 16, 0, 0, 0, now.Location())
	//open := time.Date(year, month, day, 9, 30, 0, 0, now.Location())
	//close_ := time.Date(year, month, day, 16, 0, 0, 0, now.Location())
	if !eOpenRun && now.After(eOpen) && now.Before(eClose) {
		eOpenRun = true
		eCloseRun = false
		fmt.Println("running open schedule ----------------------")
		//socket.StartPolygonWS(conn)
		initialize(conn)
		pushJournals(conn, year, month, day)
	}
	if !eCloseRun && now.After(eClose) {
		eOpenRun = false
		eCloseRun = true
		fmt.Println("running close schedule ----------------------")
		updateSecurities(conn, false)
	}
}
