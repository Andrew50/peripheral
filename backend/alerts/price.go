package alerts

import (
    "backend/utils"
)

func processPriceAlert(conn *utils.Conn, alert Alert) error {
    ds := data[*alert.SecurityId]
    ds.DayData.mutex.RLock()
    defer ds.DayData.mutex.RUnlock()
    if *alert.Direction {
        if ds.DayData.Aggs[Length - 1][1] > *alert.Price {
            dispatchAlert(conn,alert)

        }
    }else{
        if ds.DayData.Aggs[Length - 1][2] < *alert.Price {
            dispatchAlert(conn,alert)
        }
    
    }
    return nil
}

