package alerts

import (
	"backend/internal/data"
	"backend/internal/services/socket"
	"fmt"
)

func getCurrentPriceSnapshot() map[int]float64 {
	snapshot := make(map[int]float64)

	socket.AggDataMutex.RLock()
	defer socket.AggDataMutex.RUnlock()

	for securityID, ds := range socket.AggData {
		if ds != nil && ds.SecondDataExtended != nil {
			ds.SecondDataExtended.Mutex.RLock()
			if len(ds.SecondDataExtended.Aggs) >= socket.AggsLength {
				snapshot[securityID] = ds.SecondDataExtended.Aggs[socket.AggsLength-1][1]
			}
			ds.SecondDataExtended.Mutex.RUnlock()
		}
	}

	return snapshot
}

// depricated?!?!?!?!?!?!?!?!???
func processPriceAlert(conn *data.Conn, alert Alert) error {
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
				if err := dispatchAlert(alert); err != nil {
					return fmt.Errorf("failed to dispatch alert: %v", err)
				}
			}
		} else {
			if price <= *alert.Price {
				if err := dispatchAlert(alert); err != nil {
					return fmt.Errorf("failed to dispatch alert: %v", err)
				}
			}
		}
	} else {
		return fmt.Errorf("no direction pointer")
	}
	return nil
}
