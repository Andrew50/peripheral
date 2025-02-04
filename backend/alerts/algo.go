package alerts

import (
	"backend/utils"
	"fmt"
	"log"
	"time"
)

func processAlgoAlert(conn *utils.Conn, alert Alert) error {
	return nil
}
func processTapeBursts(conn *utils.Conn, alert Alert) {
	socket.
		alertAggDataMutex.RLock()
	defer alertAggDataMutex.RUnlock()
	for securityID, sd := range alertAggData {
		if len(sd.VolBurstData.VolumeThreshold) == 0 {
			continue
		}
		ticker, err := utils.GetTicker(conn, securityID, time.Now())
		fmt.Printf("Running volburst on %s\n", ticker)
		if isTapeBurst(sd) {
			if err != nil {
				log.Printf("Error getting ticker for security %d: %v", securityID, err)
				continue
			}
			dispatchAlert(conn, alert)
		}
	}

}
