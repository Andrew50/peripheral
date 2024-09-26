package tasks

import (
    "backend/utils"
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/polygon-io/client-go/rest/iter"
    "github.com/polygon-io/client-go/rest/models"
)

type GetChartDataArgs struct {
    SecurityId    int    `json:"securityId"`
    Timeframe     string `json:"timeframe"`
    Timestamp     int64  `json:"timestamp"`
    Direction     string `json:"direction"`
    Bars          int    `json:"bars"`
    ExtendedHours bool   `json:"extendedHours"`
    IsReplay      bool   `json:"isreplay"`
}

type GetChartDataResults struct {
    Timestamp float64 `json:"time"`
    Open      float64 `json:"open"`
    High      float64 `json:"high"`
    Low       float64 `json:"low"`
    Close     float64 `json:"close"`
    Volume    float64 `json:"volume"`
}
func MaxDivisorOf30(n int) int {
    for k := n; k >= 1; k-- {
        if 30%k == 0 && n%k == 0 {
            return k
        }
    }
    return 1 // 1 divides all integers, so we return 1 if no other common divisor is found.
}
type GetChartIncompleteAggregateCandleArgs struct {
    SecurityId int `json:"securityId"`
    Timeframe string `json:"timeframe"`
    Timestamp int64 `json:timestamp` // any time within the candle 
    IsReplay bool `json:"isreplay`

}
func GetChartIncompleteAggregateCandle(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
    var args GetChartIncompleteAggregateCandleArgs 
    if err := json.Unmarshal(rawArgs, &args); err !=  nil {
        return nil, fmt.Errorf("invalid args: %v", err)
    }
    multiplier, timespan, _, _, err := utils.GetTimeFrame(args.Timeframe)
    if err != nil {
        return nil, fmt.Errorf("invalid timeframe: %v", err)
    }
    startOfTimeframe := utils.getStartOfTimeWindow(, args)

}
func GetChartData(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
    var args GetChartDataArgs
    if err := json.Unmarshal(rawArgs, &args); err != nil {
        return nil, fmt.Errorf("invalid args: %v", err)
    }

    multiplier, timespan, _, _, err := utils.GetTimeFrame(args.Timeframe)
    if err != nil {
        return nil, fmt.Errorf("invalid timeframe: %v", err)
    }

    var queryTimespan string
    var queryMultiplier int
    var queryBars int
    haveToAggregate := false
    if (timespan == "second" || timespan == "minute") && (30%multiplier != 0) {
        queryTimespan = timespan
        queryMultiplier = MaxDivisorOf30(multiplier)
        queryMultiplier = 1
        queryBars = args.Bars * multiplier / queryMultiplier
        haveToAggregate = true
    } else if timespan == "hour" && !args.ExtendedHours {
        queryTimespan = "minute"
        queryMultiplier = 30
        queryBars = multiplier * 2 * args.Bars
        timespan = "minute"
        multiplier *= 60
        haveToAggregate = true
    }else{
        queryTimespan = timespan
        queryMultiplier = multiplier
        queryBars = args.Bars
    }

    // For daily and higher timeframes, always include extended hours
    if timespan != "minute" && timespan != "second" && timespan != "hour" {
        args.ExtendedHours = true
    }

    easternLocation, err := time.LoadLocation("America/New_York")
    if err != nil {
        return nil, fmt.Errorf("issue loading eastern location: %v", err)
    }

    var inputTimestamp time.Time
    if (args.Timestamp == 0){

        inputTimestamp = time.Now().In(easternLocation)
    }else{
        inputTimestamp = time.Unix(args.Timestamp/1000, (args.Timestamp%1000)*1e6).UTC()
    }
    fmt.Println(inputTimestamp)

    var query string
    var queryParams []interface{}
    var polyResultOrder string

    if args.Timestamp == 0 {
        query = `SELECT ticker, minDate, maxDate 
                 FROM securities 
                 WHERE securityid = $1
                 ORDER BY minDate DESC NULLS FIRST`
        queryParams = []interface{}{args.SecurityId}
        polyResultOrder = "desc"
    } else if args.Direction == "backward" {
        query = `SELECT ticker, minDate, maxDate
                 FROM securities 
                 WHERE securityid = $1 AND (maxDate > $2 OR maxDate IS NULL)
                 ORDER BY minDate DESC NULLS FIRST LIMIT 1`
        queryParams = []interface{}{args.SecurityId, inputTimestamp}
        polyResultOrder = "desc"
    } else if args.Direction == "forward" {
        query = `SELECT ticker, minDate, maxDate
                 FROM securities 
                 WHERE securityid = $1 AND (minDate < $2 OR minDate IS NULL)
                 ORDER BY minDate ASC NULLS LAST`
        queryParams = []interface{}{args.SecurityId, inputTimestamp}
        polyResultOrder = "asc"
    } else {
        return nil, fmt.Errorf("Incorrect direction passed")
    }

    ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
    defer cancel()
    rows, err := conn.DB.Query(ctx, query, queryParams...)
    if err != nil {
        if ctx.Err() == context.DeadlineExceeded {
            return nil, fmt.Errorf("query timed out: %w", err)
        }
        return nil, fmt.Errorf("error querying data: %w", err)
    }
    defer rows.Close()

    var barDataList []GetChartDataResults
    numBarsRemaining := args.Bars

    for rows.Next() {
        var ticker string
        var minDateFromSQL *time.Time
        var maxDateFromSQL *time.Time
        err := rows.Scan(&ticker, &minDateFromSQL, &maxDateFromSQL)
        if err != nil {
            return nil, fmt.Errorf("error scanning data: %w", err)
        }

        // Handle NULL dates from the database
		if maxDateFromSQL == nil {
			now := time.Now()
			maxDateFromSQL = &now
		}
        var minDateSQL, maxDateSQL time.Time
        if minDateFromSQL != nil {
            minDateSQL = minDateFromSQL.In(easternLocation)
        } else {
            // Default to a very early date if minDate is NULL
            minDateSQL = time.Date(1970, 1, 1, 0, 0, 0, 0, easternLocation)
        }

        if maxDateFromSQL != nil {
            maxDateSQL = (maxDateFromSQL).In(easternLocation)
        } else {
            // Default to current time if maxDate is NULL
            maxDateSQL = time.Now().In(easternLocation)
        }

        var queryStartTime, queryEndTime time.Time

        if args.Direction == "backward" {
            queryEndTime = inputTimestamp
            if maxDateSQL.Before(queryEndTime) {
                queryEndTime = maxDateSQL
            }
            queryStartTime = minDateSQL
            if queryStartTime.After(queryEndTime) {
                return nil, fmt.Errorf("i10i0v")
            }
        } else if args.Direction == "forward" {
            queryStartTime = inputTimestamp
            if minDateSQL.After(queryStartTime) {
                queryStartTime = minDateSQL
            }
            queryEndTime = maxDateSQL
            if queryEndTime.Before(queryStartTime) {
                continue
            }
        }

        date1, date2, err := getRequestDates(queryStartTime,queryEndTime,args.Direction,timespan,multiplier,queryBars)
        if err != nil {
            return nil, fmt.Errorf("dkn0 %v",err)
        }

        if haveToAggregate {
            iter, err := utils.GetAggsData(conn.Polygon, ticker, queryMultiplier, queryTimespan, date1, date2, 50000, "asc", !args.IsReplay)
            if err != nil {
                return nil, fmt.Errorf("error fetching data from Polygon: %v", err)
            }
            aggregatedData,err := buildHigherTimeframeFromLower(iter, multiplier, timespan, args.ExtendedHours, easternLocation, &numBarsRemaining,args.Direction)
            if err != nil {
                return nil, err
            }
            barDataList = append(barDataList, aggregatedData...)
            if numBarsRemaining <= 0 {
                break
            }
        } else {
            iter, err := utils.GetAggsData(conn.Polygon, ticker, queryMultiplier, queryTimespan, date1, date2, 50000, polyResultOrder, !args.IsReplay)
            if err != nil {
                return nil, fmt.Errorf("error fetching data from Polygon: %v", err)
            }
            for iter.Next() {
                item := iter.Item()
                if iter.Err() != nil {
                    return nil, fmt.Errorf("dkn0w")
                }
                timestamp := time.Time(item.Timestamp).In(easternLocation)
                marketOpenTime := time.Date(timestamp.Year(), timestamp.Month(), timestamp.Day(), 9, 30, 0, 0, easternLocation)
                marketCloseTime := time.Date(timestamp.Year(), timestamp.Month(), timestamp.Day(), 16, 0, 0, 0, easternLocation)
                if args.ExtendedHours || (!timestamp.Before(marketOpenTime) && timestamp.Before(marketCloseTime)) {
                    barData := GetChartDataResults{
                        Timestamp: float64(timestamp.Unix()),
                        Open:      item.Open,
                        High:      item.High,
                        Low:       item.Low,
                        Close:     item.Close,
                        Volume:    item.Volume,
                    }
                    barDataList = append(barDataList, barData)
                    numBarsRemaining--
                    if numBarsRemaining <= 0 {
                        break
                    }
                }
            }
            if numBarsRemaining <= 0 {
                break
            }
        }
    }
    if len(barDataList) != 0 {
        if haveToAggregate || args.Direction == "forward" {
            return barDataList, nil
        } else {
            reverse(barDataList)
            starTim := int64(barDataList[0].Timestamp)
            endTim := int64(barDataList[len(barDataList)-1].Timestamp)
            strt := time.Unix(starTim, (starTim)*1e6).UTC()
            end := time.Unix(endTim, (endTim)*1e6).UTC()
            fmt.Println("got", strt,end)
            return barDataList, nil
        }
    }
    return nil, fmt.Errorf("no data found")
}

func getRequestDates(
    lowerDate time.Time,
    upperDate time.Time,
    direction string,
    timespan string,
    multiplier int,
    bars int,
) (models.Millis, models.Millis, error) {
    overestimate := 2.0
    badReturn, err := utils.MillisFromUTCTime(time.Now())
    if direction != "backward" && direction != "forward" {
        return badReturn, badReturn, fmt.Errorf("invalid direction; must be 'back' or 'forward'")
    }
    barDuration := timespanToDuration(timespan) * time.Duration(multiplier)
    totalDuration := barDuration * time.Duration(int(float64(bars)*overestimate))
    tradingMinutesPerDay := 960.0 // 16 hours * 60 minutes
    tradingDurationPerDay := time.Duration(int(tradingMinutesPerDay)) * time.Minute
    totalTradingDays := totalDuration / tradingDurationPerDay
    totalTradingDays += 3 //overestimate cause weekends
    var queryStartTime, queryEndTime time.Time
    if direction == "backward" {
        queryStartTime = upperDate.AddDate(0, 0, -int(totalTradingDays))
        if queryStartTime.Before(lowerDate) {
            queryStartTime = lowerDate
        }
        queryEndTime = upperDate
        queryStartTime = time.Date(queryStartTime.Year(), queryStartTime.Month(), queryStartTime.Day(), 0, 0, 0, 0, queryStartTime.Location())
    } else {
        queryEndTime = lowerDate.AddDate(0, 0, int(totalTradingDays))
        if queryEndTime.After(upperDate) {
            queryEndTime = upperDate
        }
        queryStartTime = lowerDate
    }
    fmt.Println(queryStartTime,queryEndTime)

    startMillis, err := utils.MillisFromUTCTime(queryStartTime)
    if err != nil {
        return badReturn,badReturn,err
    }
    endMillis, err := utils.MillisFromUTCTime(queryEndTime)
    if err != nil {
        return badReturn,badReturn, err
    }
    return startMillis, endMillis, nil
}

func timespanToDuration(timespan string) time.Duration {
    switch timespan {
    case "second":
        return time.Second
    case "minute":
        return time.Minute
    case "hour":
        return time.Hour
    case "day":
        return time.Hour * 24
    case "week":
        return time.Hour * 24 * 7
    case "month":
        return time.Hour * 24 * 30
    case "year":
        return time.Hour * 24 * 365
    default:
        return time.Minute
    }
}

func reverse(data []GetChartDataResults) {
    left, right := 0, len(data)-1
    for left < right {
        data[left], data[right] = data[right], data[left]
        left++
        right--
    }
}

func buildHigherTimeframeFromLower(iter *iter.Iter[models.Agg], multiplier int, timespan string, extendedHours bool, easternLocation *time.Location, numBarsRemaining *int,direction string) ([]GetChartDataResults,error) {
    var barDataList []GetChartDataResults
    var currentBar GetChartDataResults
    var barStartTime time.Time

    b := 0

    for iter.Next() {
        item := iter.Item()
        err := iter.Err()
        if err != nil {
            return nil, fmt.Errorf("din0wi %v",err)
        }
        timestamp := time.Time(item.Timestamp).In(easternLocation)
        marketOpenTime := time.Date(timestamp.Year(), timestamp.Month(), timestamp.Day(), 9, 30, 0, 0, easternLocation)
        marketCloseTime := time.Date(timestamp.Year(), timestamp.Month(), timestamp.Day(), 16, 0, 0, 0, easternLocation)
        if extendedHours || (!timestamp.Before(marketOpenTime) && timestamp.Before(marketCloseTime)) {
            diff := timestamp.Sub(barStartTime)
            if barStartTime.IsZero() || diff >= time.Duration(multiplier)*timespanToDuration(timespan) {
                if !barStartTime.IsZero() {
                    barDataList = append(barDataList, currentBar)
                    if direction == "forwards" {
                        *numBarsRemaining--
                        if *numBarsRemaining <= 0 {
                            break
                        }
                    }
                }
                currentBar = GetChartDataResults{
                    Timestamp: float64(timestamp.Unix()),
                    Open:      item.Open,
                    High:      item.High,
                    Low:       item.Low,
                    Close:     item.Close,
                    Volume:    item.Volume,
                }
                barStartTime = timestamp
            } else {
                currentBar.High = max(currentBar.High, item.High)
                currentBar.Low = min(currentBar.Low, item.Low)
                currentBar.Close = item.Close
                currentBar.Volume += item.Volume
            }
        }
        b ++
    }
    if direction == "forwards" {
        if !barStartTime.IsZero() && *numBarsRemaining > 0 {
            barDataList = append(barDataList, currentBar)
            *numBarsRemaining--
        }
    }else{
        barsToKeep := len(barDataList) - *numBarsRemaining
        if barsToKeep < 0 {
            barsToKeep = 0
            *numBarsRemaining -= len(barDataList)
        }else{
            *numBarsRemaining = 0
        }
        barDataList = barDataList[barsToKeep:]
    }

    return barDataList, nil
}

func max(a, b float64) float64 {
    if a > b {
        return a
    }
    return b
}

func min(a, b float64) float64 {
    if a < b {
        return a
    }
    return b
}

