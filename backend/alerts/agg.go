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
    rolloverTimestamp int64
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
        rolloverTimestamp: -1,
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
    var lastTimestamp int64
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
        
        idx++
        lastTimestamp = time.Time(agg.Timestamp).Unix()
    }
    if err := iter.Err(); err != nil {
        fmt.Printf("Error iterating historical data: %v\n", err)
    }
    //if td.rolloverTimestamp == -1 {
    td.rolloverTimestamp = lastTimestamp + int64(timeframe) //if theres no data then this wont work but is extreme edge case
    //}
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
    //periodStart := getPeriodStart(timestamp, timeframe)
    /*if td.currentPeriod == -1 {
        td.currentPeriod = periodStart
        td.Aggs[0] = []float64{price, price, price, price, volume}
        td.size = 1
        return
    }*/

    //if periodStart > td.currentPeriod {
    if timestamp >= td.rolloverTimestamp { // if out of order ticks
        if td.size > 0 {
            copy(td.Aggs[1:], td.Aggs[0:min(td.size, Length-1)])
        }
        td.Aggs[0] = []float64{price, price, price, price, volume}
        //td.currentPeriod = periodStart
        td.rolloverTimestamp = nextPeriodStart(timestamp,timeframe)
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

func LoadAggregates(conn *utils.Conn){
}

func AppendTick(conn *utils.Conn,securityId int, timestamp int64, price float64, intVolume int64) error {
//    fmt.Println("added tick",securityId,intVolume)
    volume := float64(intVolume)
    sd, exists := data[securityId]
    if !exists {
        return fmt.Errorf("fid0w0f")
        //sd = initSecurityData(conn,securityId)
        //data[securityId] = sd
    }
    if utils.IsTimestampRegularHours(time.Unix(timestamp,timestamp*int64(time.Millisecond))) {
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

/*func getPeriodStart(timestamp int64, tf int) int64 {
    return timestamp - (timestamp % int64(tf))
}*/
func nextPeriodStart(timestamp int64, tf int) int64 {
    return timestamp - (timestamp % int64(tf)) + int64(tf)
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
    result := make([][]float64, len(td.Aggs))
    for i := range td.Aggs {
        result[i] = make([]float64, OHLCV)
        copy(result[i], td.Aggs[i])
    }
    return result, nil
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
