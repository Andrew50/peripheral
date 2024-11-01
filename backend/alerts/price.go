package alerts

import (
    "backend/utils"
    "fmt"
)

func processPriceAlert(conn *utils.Conn, alert Alert) error {
    ds := data[*alert.SecurityId]
    if ds == nil {
        return fmt.Errorf("1-90vj- price alert")
    }
    fmt.Println("god")
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

