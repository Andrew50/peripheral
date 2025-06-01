package alerts

import (
	"backend/internal/data"
	"backend/internal/services/socket"
	"fmt"
)

func createPriceSnapshot() map[int]float64 {
	snapshot := make(map[int]float64)

	socket.AggDataMutex.RLock()
	defer socket.AggDataMutex.RUnlock()

	for securityID, ds := range socket.AggData {
		if ds != nil {
			ds.SecondDataExtended.Mutex.RLock()

			if len(ds.SecondDataExtended.Aggs) > 0 {
				price := ds.SecondDataExtended.Aggs[socket.AggsLength-1][1]
				snapshot[securityID] = price
			}
			ds.SecondDataExtended.Mutex.RUnlock()
		}
	}

	return snapshot
}

func processPriceAlert(conn *data.Conn, alert Alert, priceSnapshot map[int]float64) error {
	if alert.SecurityID == nil {
		return fmt.Errorf("alert has no security ID")
	}

	currentPrice, exists := priceSnapshot[*alert.SecurityID]
	if !exists {
		return fmt.Errorf("market data not found for security ID %d", *alert.SecurityID)
	}

	directionPtr := alert.Direction
	if directionPtr == nil {
		return fmt.Errorf("no direction pointer")
	}

	shouldTrigger := false
	if *directionPtr {
		shouldTrigger = currentPrice >= *alert.Price
	} else {
		shouldTrigger = currentPrice <= *alert.Price
	}

	if shouldTrigger {
		if err := dispatchAlert(conn, alert); err != nil {
			return fmt.Errorf("failed to dispatch alert: %v", err)
		}
	}

	return nil
}
