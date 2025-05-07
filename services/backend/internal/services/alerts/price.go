package alerts

import (
	"backend/internal/services/marketData"
	"backend/internal/data"
	"fmt"
)

func processPriceAlert(conn *data.Conn, alert Alert) error {
	marketData.AggDataMutex.RLock()         // Acquire read lock
	defer marketData.AggDataMutex.RUnlock() // Release read lock
	ds := marketData.AggData[*alert.SecurityID]
	if ds == nil {
		return fmt.Errorf("market data not found for security ID %d", *alert.SecurityID)
	}
	ds.SecondDataExtended.Mutex.RLock()
	defer ds.SecondDataExtended.Mutex.RUnlock()
	directionPtr := alert.Direction
	if directionPtr != nil {
		price := ds.SecondDataExtended.Aggs[marketData.AggsLength-1][1]
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
