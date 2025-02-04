package alerts

import (
	"backend/utils"
	"fmt"
)

func processPriceAlert(conn *utils.Conn, alert Alert) error {
	alertAggDataMutex.RLock()         // Acquire read lock
	defer alertAggDataMutex.RUnlock() // Release read lock
	ds := alertAggData[*alert.SecurityId]
	if ds == nil {
		return fmt.Errorf("1-90vj- price alert")
	}
	ds.SecondDataExtended.mutex.RLock()
	defer ds.SecondDataExtended.mutex.RUnlock()
	directionPtr := alert.Direction
	if directionPtr != nil {
		price := ds.SecondDataExtended.Aggs[Length-1][1]
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
