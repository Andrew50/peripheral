package alerts

import (
	"backend/socket"
	"backend/utils"
	"fmt"
	"log"
	"time"
)

func processAlgoAlert(conn *utils.Conn, alert Alert) error {
	if alert.AlgoID != nil && *alert.AlgoID == 0 {
		processTapeBursts(conn, alert)
	} else {
		//fmt.Println("Algo alert not found")
	}
	return nil
}
func processTapeBursts(conn *utils.Conn, alert Alert) {
	socket.AggDataMutex.RLock()
	defer socket.AggDataMutex.RUnlock()
	for securityID, sd := range socket.AggData {
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
			fmt.Printf("Tape burst detected on %s %v\n", ticker, alert)
			//dispatchAlert(conn, alert)
		}
	}

}
