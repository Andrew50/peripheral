package alerts

import (
	"backend/socket"
	"backend/utils"
	"fmt"
)

func processPriceAlert(conn *utils.Conn, alert Alert) error {
	socket.AggDataMutex.RLock()         // Acquire read lock
	defer socket.AggDataMutex.RUnlock() // Release read lock
	ds := socket.AggData[*alert.SecurityID]
	if ds == nil {
		return fmt.Errorf("market data not found for security ID %d", *alert.SecurityID)
	}
	ds.SecondDataExtended.Mutex.RLock()
	defer ds.SecondDataExtended.Mutex.RUnlock()
	directionPtr := alert.Direction
	if directionPtr != nil {
		price := ds.SecondDataExtended.Aggs[socket.AggsLength-1][1]
		if *directionPtr {
			if price >= *alert.Price {
				dispatchAlert(conn, alert)
			}
		} else {
			if price <= *alert.Price {
				dispatchAlert(conn, alert)
			}
		}
	} else {
		fmt.Println("no direction pointer")
	}
	return nil
}
