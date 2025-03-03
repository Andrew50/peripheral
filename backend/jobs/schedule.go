package jobs

import (
	"backend/socket"
	//"backend/alerts"
	"backend/alerts"
	"backend/utils"
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

var eOpenRun = false
var eCloseRun = false

var useBS = true //alerts, securityUpdate, marketMetrics, sectorUpdate

var (
	polygonInitialized bool
	polygonInitMutex   sync.Mutex
	alertsInitialized  bool
	alertsInitMutex    sync.Mutex
)

func StartScheduler(conn *utils.Conn) chan struct{} {

	go initialize(conn)
	//eventLoop(time.Now(), conn)

	updateSectors(conn)

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
	// Clear worker queue on initialization to prevent backlog
	err := conn.Cache.Del(context.Background(), "queue").Err()
	if err != nil {
		fmt.Println("Failed to clear worker queue:", err)
	} else {
		fmt.Println("Worker queue cleared successfully during initialization")
	}

	// Queue sector update on first init

	if useBS {
		socket.InitAggregatesAsync(conn)

		alertsInitMutex.Lock()
		if !alertsInitialized {
			err := alerts.StartAlertLoop(conn)
			if err != nil {
				fmt.Println("schedule issue: k0w0c", err)
			}
			alertsInitialized = true
		}
		alertsInitMutex.Unlock()
	} else {
		fmt.Println("not using alerts !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
	}
	polygonInitMutex.Lock()
	if !polygonInitialized {
		err := socket.StartPolygonWS(conn, useBS)
		if err != nil {
			log.Printf("Failed to start Polygon WebSocket: %v", err)
			// Continue with initialization even if Polygon WS fails
			// You might want to add retry logic or better error handling here
		}
		polygonInitialized = true
	}
	polygonInitMutex.Unlock()
}

func updateSectors(conn *utils.Conn) error {
	_, err := utils.Queue(conn, "update_sectors", map[string]interface{}{})
	if err != nil {
		return fmt.Errorf("error queueing sector update: %w", err)
	}
	return nil
}

func updateMarketMetrics(conn *utils.Conn) error {
	_, err := utils.Queue(conn, "update_active", map[string]interface{}{})
	if err != nil {
		return fmt.Errorf("error queueing market metrics update: %w", err)
	}
	return nil

}

func isWeekend(now time.Time) bool {
	weekday := now.Weekday()
	return weekday == time.Saturday || weekday == time.Sunday
}

func eventLoop(now time.Time, conn *utils.Conn) {
	year, month, day := now.Date()

	eOpen := time.Date(year, month, day, 4, 0, 0, 0, now.Location())
	eClose := time.Date(year, month, day, 20, 0, 0, 0, now.Location())
	//open := time.Date(year, month, day, 9, 30, 0, 0, now.Location())
	//close_ := time.Date(year, month, day, 16, 0, 0, 0, now.Location())
	fmt.Printf("\n\nStarting EdgarFilingsService\n\n")
	utils.StartEdgarFilingsService(conn)
	go func() {
		for filing := range utils.NewFilingsChannel {
			fmt.Printf("\n\nBroadcasting global SEC filing\n\n")
			socket.BroadcastGlobalSECFiling(filing)
		}
	}()
	if !eOpenRun && now.After(eOpen) && now.Before(eClose) && !isWeekend(now) {
		eOpenRun = true
		eCloseRun = false
		fmt.Println("running open schedule ----------------------")
		//socket.StartPolygonWS(conn)
		initialize(conn)
		pushJournals(conn, year, month, day)
	}
	if (!eCloseRun && now.After(eClose)) || isWeekend(now) {
		eOpenRun = false
		eCloseRun = true
		alerts.StopAlertLoop()
		if err := socket.StopPolygonWS(); err != nil {
			log.Printf("Failed to stop Polygon WebSocket: %v", err)
			// Continue execution even if stopping the websocket fails
		}
		fmt.Println("running close schedule ----------------------")
		if useBS {
			err := updateSecurityCik(conn)
			if err != nil {
				fmt.Println("schedule issue: updating ticker ciks l44lgkkvv", err)
			}
			fmt.Println("updating market metrics !!!!!!!!!!!!!!!!!!!!!!!!!!")
			err = updateMarketMetrics(conn)
			if err != nil {
				fmt.Println("schedule issue: market metrics update:", err)
			}
			err = simpleUpdateSecurities(conn)
			if err != nil {
				fmt.Println("schedule issue: dw000", err)
			}
			err = updateSecurityDetails(conn, false)
			if err != nil {
				fmt.Println("schedule issue: security details update:", err)
			}
			err = updateSectors(conn)
			if err != nil {
				fmt.Println("schedule issue: sector update close:", err)
			}
		}

	}
}
