package alerts

import (
    "backend/utils"
    "fmt"
    "context"
    "sync"
)
func InitAlertsAndAggs(conn *utils.Conn) error {
    ctx := context.Background()
    
    // Initialize alerts first
    if err := loadActiveAlerts(ctx, conn); err != nil {
        return fmt.Errorf("loading active alerts: %w", err)
    }
    query := `
        SELECT securityId 
        FROM securities 
        WHERE maxDate is NULL = true`
    
    rows, err := conn.DB.Query(ctx, query)
    if err != nil {
        return fmt.Errorf("querying securities: %w", err)
    }
    defer rows.Close()

    // Initialize data map
    data = make(map[int]*SecurityData)
    
    // Process each security
    var loadErrors []error
    for rows.Next() {
        var securityId int
        if err := rows.Scan(&securityId); err != nil {
            loadErrors = append(loadErrors, fmt.Errorf("scanning security ID: %w", err))
            continue
        }

        // Initialize security data with all timeframes
        sd := initSecurityData(conn, securityId)
        if sd == nil {
            loadErrors = append(loadErrors, fmt.Errorf("failed to initialize security data for ID %d", securityId))
            continue
        }

        // Validate the initialized data
        if err := validateSecurityData(sd, securityId); err != nil {
            loadErrors = append(loadErrors, fmt.Errorf("validation failed for security %d: %w", securityId, err))
            continue
        }

        // Store in global map
        data[securityId] = sd
    }

    if err = rows.Err(); err != nil {
        return fmt.Errorf("iterating security rows: %w", err)
    }

    // If there were any errors during loading, combine them into a single error
    if len(loadErrors) > 0 {
        var errMsg string
        for i, err := range loadErrors {
            if i > 0 {
                errMsg += "; "
            }
            errMsg += err.Error()
        }
        return fmt.Errorf("errors loading aggregates: %s", errMsg)
    }

    // Validate alert securities exist in data map
    var alertErrors []error
    alerts.Range(func(key, value interface{}) bool {
        alert := value.(Alert)
        if alert.SecurityId != nil {
            if _, exists := data[*alert.SecurityId]; !exists {
                alertErrors = append(alertErrors, 
                    fmt.Errorf("alert ID %d references non-existent security ID %d", 
                        alert.AlertId, *alert.SecurityId))
            }
        }
        return true
    })

    // Report any alert validation errors
    if len(alertErrors) > 0 {
        var errMsg string
        for i, err := range alertErrors {
            if i > 0 {
                errMsg += "; "
            }
            errMsg += err.Error()
        }
        return fmt.Errorf("errors validating alerts: %s", errMsg)
    }

    return nil
}

// Helper function to load active alerts
func loadActiveAlerts(ctx context.Context, conn *utils.Conn) error {
    query := `
        SELECT alertId, userId, alertType, setupId, price, direction, securityId
        FROM alerts
        WHERE active = true
    `
    rows, err := conn.DB.Query(ctx, query)
    if err != nil {
        return fmt.Errorf("querying active alerts: %w", err)
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
            &alert.Direction,
            &alert.SecurityId,
        )
        if err != nil {
            return fmt.Errorf("scanning alert row: %w", err)
        }
        alerts.Store(alert.AlertId, alert)
    }

    if err = rows.Err(); err != nil {
        return fmt.Errorf("iterating alert rows: %w", err)
    }

    return nil
}

// Helper function to validate initialized security data
func validateSecurityData(sd *SecurityData, securityId int) error {
    if sd == nil {
        return fmt.Errorf("security data is nil")
    }

    // Validate SecondDataExtended
    if err := validateTimeframeData(&sd.SecondDataExtended, "second", true); err != nil {
        return fmt.Errorf("second data validation failed: %w", err)
    }

    // Validate MinuteDataExtended
    if err := validateTimeframeData(&sd.MinuteDataExtended, "minute", true); err != nil {
        return fmt.Errorf("minute data validation failed: %w", err)
    }

    // Validate HourData
    if err := validateTimeframeData(&sd.HourData, "hour", false); err != nil {
        return fmt.Errorf("hour data validation failed: %w", err)
    }

    // Validate DayData
    if err := validateTimeframeData(&sd.DayData, "day", false); err != nil {
        return fmt.Errorf("day data validation failed: %w", err)
    }

    return nil
}

// Helper function to validate timeframe data
func validateTimeframeData(td *TimeframeData, timeframeName string, extendedHours bool) error {
    if td == nil {
        return fmt.Errorf("%s timeframe data is nil", timeframeName)
    }

    if td.Aggs == nil {
        return fmt.Errorf("%s aggregates array is nil", timeframeName)
    }

    if len(td.Aggs) != Length {
        return fmt.Errorf("%s aggregates length mismatch: got %d, want %d", 
            timeframeName, len(td.Aggs), Length)
    }

    if td.rolloverTimestamp == -1 {
        return fmt.Errorf("%s rollover timestamp not initialized", timeframeName)
    }

    return nil
}
