package jobs

import (
    "time"
    "backend/utils"
    "fmt"
)

var eOpenRun = false
var eCloseRun = false

func StartScheduler(conn *utils.Conn) chan struct{} {
    ticker := time.NewTicker(1 * time.Minute)
    quit := make(chan struct{})
    location, err := time.LoadLocation("EST")
    if err != nil {
        panic(fmt.Errorf("219jv %v",err))
    }
    go func() {
        for {
            select {
            case <-ticker.C:
                now := time.Now().In(location)
                eventLoop(now,conn)
            case <-quit:
                ticker.Stop()
                return
            }
        }
    }()
    return quit
}

func eventLoop(now time.Time,conn *utils.Conn) {
    year, month, day := now.Date()
    eOpen := time.Date(year,month,day,4,0,0,0,now.Location())
    eClose := time.Date(year,month,day,16,0,0,0,now.Location())
    //open := time.Date(year, month, day, 9, 30, 0, 0, now.Location())
    //close_ := time.Date(year, month, day, 16, 0, 0, 0, now.Location())
    if !eOpenRun && now.After(eOpen) && now.Before(eClose) {
        fmt.Println("running open update")
        startPolygonWS(conn)
        eOpenRun = true
        eCloseRun = false
    }

    if !eCloseRun && now.After(eClose) {
        fmt.Println("running close update")
        updateSecurities(conn,false)
        eOpenRun = false
        eCloseRun = true
    }

}
