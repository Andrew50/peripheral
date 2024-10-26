package alerts

import (
    "sync"
    "context"
    "backend/utils"
)

func loadActiveAlerts(ctx context.Context, conn *utils.Conn) error {
    query := `
        SELECT alertId, userId, alertType, setupId, price, securityID
        FROM alerts
        WHERE active = true
    `
    rows, err := conn.DB.Query(ctx, query)
    if err != nil {
        return err
    }
    defer rows.Close()
    alerts = sync.Map{}
    for rows.Next() {
        var alert Alert
        err := rows.Scan(
            &alert.AlertId,
            &alert.UserId,
            &alert.AlertType,
            &alert.SetupId,
            &alert.Price,
            &alert.SecurityID,
        )
        if err != nil {
            return err
        }
        alerts.Store(alert.AlertId, alert)
    }
    if err = rows.Err(); err != nil {
        return err
    }
    return nil
}


func loadAggregates( conn *utils.Conn) error {
    return nil 
}
