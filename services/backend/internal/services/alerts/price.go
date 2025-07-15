package alerts

import (
	"backend/internal/data"
	"backend/internal/services/socket"
	"fmt"
)

func processPriceAlert(conn *data.Conn, alert Alert) error {
	directionPtr := alert.Direction
	if directionPtr != nil {
		// Get the latest price from the websocket price cache
		price, exists := socket.GetLatestPrice(*alert.SecurityID)
		if !exists {
			return fmt.Errorf("no price data available for security ID %d", *alert.SecurityID)
		}

		// Skip alert processing if price is -1 (indicates skip OHLC condition)
		if price < 0 {
			return nil
		}

		if *directionPtr {
			if price >= *alert.Price {
				if err := dispatchAlert(conn, alert); err != nil {
					return fmt.Errorf("failed to dispatch alert: %v", err)
				}
			}
		} else {
			if price <= *alert.Price {
				if err := dispatchAlert(conn, alert); err != nil {
					return fmt.Errorf("failed to dispatch alert: %v", err)
				}
			}
		}
	} else {
		return fmt.Errorf("no direction pointer")
	}
	return nil
}
