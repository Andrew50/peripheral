package alerts

import (
    "sync"
    "errors"
    "backend/utils"
    "time"
    "fmt"
)

const (
    Length = 100
    OHLCV = 5
    Second = 1
    Minute = 60
    Hour = 3600
    Day = 86400
)

type TimeframeData struct {
    Aggs [][]float64
    size int
    currentPeriod int64
    extendedHours bool
    mutex sync.RWMutex
}

type SecurityData struct {
    SecondDataExtended TimeframeData
    MinuteDataExtended TimeframeData
    HourData TimeframeData
    DayData TimeframeData
    Dolvol float64
    Mcap float64
    Adr float64
}

var (
    data = make(map[int]*SecurityData)
)

func initTimeframeData(conn *utils.Conn, securityId int, timeframe int, isExtendedHours bool) TimeframeData {
    aggs := make([][]float64, Length)
    for i := range aggs {
        aggs[i] = make([]float64, OHLCV)
    }
    td := TimeframeData{
        Aggs:          aggs,
        size:          0,
        currentPeriod: -1,
        extendedHours: isExtendedHours,
    }
    toTime := time.Now()
    var tfStr string
    var multiplier int
    switch timeframe {
    case Second:
        tfStr = "second"
        multiplier = 1
    case Minute:
        tfStr = "minute"
        multiplier = 1
    case Hour:
        tfStr = "hour"
        multiplier = 1
    case Day:
        tfStr = "day"
        multiplier = 1
    default:
        fmt.Printf("Invalid timeframe: %d\n", timeframe)
        return td
    }
    fromMillis, toMillis, err := utils.GetRequestStartEndTime(time.Unix(0,0),toTime,"backward",tfStr,multiplier,Length)
    ticker, err := utils.GetTicker(conn,securityId,toTime)
    if err != nil {
        fmt.Printf("error getting hist data")
        return td
    }
    iter, err := utils.GetAggsData(conn.Polygon, ticker, multiplier, tfStr, fromMillis, toMillis, Length, "desc", true)
    if err != nil {
        fmt.Printf("Error getting historical data: %v\n", err)
        return td
    }

    // Process historical data
    var idx int
    for iter.Next() {
        agg := iter.Item()
        
        // Skip if we're not including extended hours data
        timestamp := time.Time(agg.Timestamp)
        if !isExtendedHours && !utils.IsTimestampRegularHours(timestamp) {
            continue
        }

        if idx >= Length {
            break
        }

        td.Aggs[idx] = []float64{
            agg.Open,
            agg.High,
            agg.Low,
            agg.Close,
            float64(agg.Volume),
        }
        
        // Update period tracking
        if td.currentPeriod == -1 {
            td.currentPeriod = getPeriodStart(timestamp.Unix(), timeframe)
        }
        
        idx++
    }

    if err := iter.Err(); err != nil {
        fmt.Printf("Error iterating historical data: %v\n", err)
    }

    td.size = idx
    return td
}

func initSecurityData(conn *utils.Conn, securityId int) *SecurityData {
    return &SecurityData{
        SecondDataExtended: initTimeframeData(conn, securityId, Second, true),
        MinuteDataExtended: initTimeframeData(conn, securityId, Minute, true),
        HourData:          initTimeframeData(conn, securityId, Hour, false),
        DayData:           initTimeframeData( conn, securityId, Day, false),
/*        Mcap: getMcap(conn,securityId),
        Dolvol: getDolvol(conn,securityId),
        Adr: getAdr(conn,securityId),*/
    }
}
func updateTimeframe(td *TimeframeData, timestamp int64, price float64, volume float64, timeframe int) {
    periodStart := getPeriodStart(timestamp, timeframe)
    if td.currentPeriod == -1 {
        td.currentPeriod = periodStart
        td.Aggs[0] = []float64{price, price, price, price, volume}
        td.size = 1
        return
    }

    if periodStart > td.currentPeriod {
        if td.size > 0 {
            copy(td.Aggs[1:], td.Aggs[0:min(td.size, Length-1)])
        }
        td.Aggs[0] = []float64{price, price, price, price, volume}
        td.currentPeriod = periodStart
        if td.size < Length {
            td.size++
        }
    } else {
        td.Aggs[0][1] = max(td.Aggs[0][1], price) // High
        td.Aggs[0][2] = min64(td.Aggs[0][2], price) // Low
        td.Aggs[0][3] = price                      // Close
        td.Aggs[0][4] += volume                    // Volume
    }
}
func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}

func max(a, b float64) float64 {
    if a > b {
        return a
    }
    return b
}

func min64(a, b float64) float64 {
    if a < b {
        return a
    }
    return b
}

func AppendTick(conn *utils.Conn,securityId int, timestamp int64, price float64, volume float64) error {
    sd, exists := data[securityId]
    if !exists {
        sd = initSecurityData(conn,securityId)
        data[securityId] = sd
    }
    if !isExtendedHours {
        sd.HourData.mutex.Lock()
        updateTimeframe(&sd.HourData, timestamp, price, volume, Hour)
        sd.HourData.mutex.Unlock()
        sd.DayData.mutex.Lock()
        updateTimeframe(&sd.DayData, timestamp, price, volume, Day)
        sd.DayData.mutex.Unlock()
    }
    sd.SecondDataExtended.mutex.Lock()
    updateTimeframe(&sd.SecondDataExtended, timestamp, price, volume, Second)
    sd.SecondDataExtended.mutex.Unlock()
    sd.MinuteDataExtended.mutex.Lock()
    updateTimeframe(&sd.MinuteDataExtended, timestamp, price, volume, Minute)
    sd.MinuteDataExtended.mutex.Unlock()
    return nil
}

func getPeriodStart(timestamp int64, tf int) int64 {
    return timestamp - (timestamp % int64(tf))
}

func GetTimeframeData(securityId int, timeframe int, extendedHours bool) ([][]float64, error) {
    sd, exists := data[securityId]
    if !exists {
        return nil, errors.New("security not found")
    }
    var td *TimeframeData
    switch timeframe {
    case Second:
        if extendedHours {
            td = &sd.SecondDataExtended
        }
    case Minute:
        if extendedHours {
            td = &sd.MinuteDataExtended
        }
    case Hour:
        td = &sd.HourData
    case Day:
        td = &sd.DayData
    default:
        return nil, errors.New("invalid timeframe")
    }
    if td == nil {
        return nil, errors.New("timeframe data not available")
    }
    td.mutex.RLock()
    defer td.mutex.RUnlock()
    return td.Aggs, nil //careful, this doesnt return a copy
}
/*
func appendAggregate(securityId int,timeframe string, o float64, h float64, l float64, c float64) error {
    sd, exists := data[securityId]
    if !exists {
        sd = initSecurityData()
        data[securityId] = sd
    }
    sd.mutex.Lock()
    defer sd.mutex.Unlock()

    if sd.size > 0 {
        copy(sd.Aggs[1:],sd.Aggs[0:min(sd.size,Length-1)])
    }
    sd.Aggs[0] = []float64{o,h,l,c}
    if sd.size < Length {
        sd.size ++
    }
    return nil
}*/
