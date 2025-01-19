package alerts

import (
    "context"
    "fmt"
    "log"
    "time"
    "sync"
    "backend/utils"
)

type Alert struct {
    AlertId    int
    UserId     int
    AlertType  string
    SetupId    *int
    Price      *float64
    Direction  *bool
    SecurityId *int
    Ticker      *string
    Message *string
}

var (
    frequency = time.Second * 3
    alerts    sync.Map
    ctx       context.Context
    cancel    context.CancelFunc
)

func AddAlert(alert Alert) {
    alerts.Store(alert.AlertId, alert)
}

func RemoveAlert(alertId int) {
    alerts.Delete(alertId)
}

func StartAlertLoop(conn *utils.Conn)  error {
	err := InitTelegramBot()
    if err != nil {
        return err
    }
    fmt.Println("initing aggs")
    if err := InitAlertsAndAggs(conn); err != nil {
        fmt.Println("god0ws")
        return err
    }
    fmt.Println("finished initing aggs")

    /*if err := loadActiveAlerts(ctx, conn); err != nil {
        return err
    }*/
    ctx, cancel = context.WithCancel(context.Background())
    go alertLoop(ctx, conn)
    return nil
}

func StopAlertLoop() {
    if cancel != nil {
        cancel()
    }
}

func alertLoop(ctx context.Context, conn *utils.Conn) {
    ticker := time.NewTicker(frequency)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            processAlerts(conn)
        }
    }
}

func processAlerts( conn *utils.Conn) {
    var wg sync.WaitGroup
    alerts.Range(func(key, value interface{}) bool {
        alert := value.(Alert)
        wg.Add(1)
        go func(a Alert) {
            defer wg.Done()
            var err error
            switch a.AlertType {
            case "price":
                err = processPriceAlert(conn, a)
            case "setup":
                err = processSetupAlert(conn, a)
            case "algo":
                err = processAlgoAlert(conn, a)
            default:
                log.Printf("Unknown alert type: %s", a.AlertType)
                return
            }
            if err != nil {
                log.Printf("Error processing alert %d: %v", a.AlertId, err)
                return
            }
        }(alert)
        return true
    })
    wg.Wait()
}
