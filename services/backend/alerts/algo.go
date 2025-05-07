package alerts

import (
	"backend/utils"
)

func processAlgoAlert(conn *utils.Conn, alert Alert) error {
	if alert.AlgoID != nil && *alert.AlgoID == 0 {
		processTapeBursts(conn, alert)
	}
	return nil
}
