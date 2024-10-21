package socket

import (
	"backend/utils"
	"context"
	"fmt"
	"time"

	"github.com/polygon-io/client-go/rest/models"
)

type TickData interface {
	GetTimestamp() int64
	GetChannel() string
	SetChannel(channel string)
}
type TradeData struct {
	//	Ticker     string  `json:"ticker"`
	Price      float64 `json:"price"`
	Size       int64   `json:"size"`
	Timestamp  int64   `json:"timestamp"`
	ExchangeId int32   `json:"exchange"`
	Conditions []int32 `json:"conditions"`
	Channel    string  `json:"channel"`
}

func (t TradeData) GetTimestamp() int64 {
	return t.Timestamp
}

func (t TradeData) GetChannel() string {
	return t.Channel
}
func (t *TradeData) SetChannel(channel string) {
	t.Channel = channel
}

type QuoteData struct {
	//Ticker    string  `json:"ticker"`
	BidPrice  float64 `json:"bidPrice"`
	AskPrice  float64 `json:"askPrice"`
	BidSize   int32   `json:"bidSize"`
	AskSize   int32   `json:"askSize"`
	Timestamp int64   `json:"timestamp"`
	Channel   string  `json:"channel"`
}

func (q QuoteData) GetTimestamp() int64 {
	return q.Timestamp
}
func (q QuoteData) GetChannel() string {
	return q.Channel
}
func (q *QuoteData) SetChannel(channel string) {
	q.Channel = channel
}

func aggregateTicks(ticks []TickData, baseDataType string) TickData {
	if len(ticks) == 0 {
		return nil
	}

	switch baseDataType {
	case "quote":
		// Aggregate the last QuoteData in the list
		return ticks[len(ticks)-1]

	case "trade":
		// Aggregate the trade data by summing the sizes and filtering conditions
		totalSize := int64(0)
		var lastTrade TradeData
		conditionsMap := make(map[int32]bool)

		for _, tick := range ticks {
			if trade, ok := tick.(*TradeData); ok {
				totalSize += trade.Size
				lastTrade = *trade
				// Add unique conditions to the map
				for _, condition := range trade.Conditions {
					conditionsMap[condition] = true
				}
			}
		}

		// Extract unique conditions into a slice
		uniqueConditions := make([]int32, 0, len(conditionsMap))
		for condition := range conditionsMap {
			uniqueConditions = append(uniqueConditions, condition)
		}

		// Create a new aggregated TradeData
		aggregatedTrade := TradeData{
			Price:      lastTrade.Price, // Using the last price in the list
			Size:       totalSize,
			Timestamp:  lastTrade.Timestamp,
			ExchangeId: lastTrade.ExchangeId,
			Conditions: uniqueConditions,
			Channel:    lastTrade.Channel,
		}

		// Marshal the aggregated trade data to JSON
		return &aggregatedTrade

	default:
		fmt.Println("Unknown baseDataType:", baseDataType)
		return nil
	}
}

func getTradeData(conn *utils.Conn, securityId int, timestamp int64, lengthOfTime int64, extendedHours bool) ([]TickData, error) {
	easternLocation, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("issue loading eastern location: %v", err)
	}

	inputTime := time.Unix(timestamp/1000, (timestamp%1000)*1e6).UTC()
	query := `SELECT ticker, minDate, maxDate FROM securities WHERE securityid=$1 AND (minDate <= $2 AND (maxDate IS NULL or maxDate >= $2)) ORDER BY minDate ASC`
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	rows, err := conn.DB.Query(ctx, query, securityId, inputTime.In(easternLocation).Format(time.DateTime))
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("query timed out: %w", err)
		}
		return nil, fmt.Errorf("error executing query: %w", err)
	}
	defer rows.Close()

	var tradeDataList []TickData
	windowStartTime := timestamp              // milliseconds
	windowEndTime := timestamp + lengthOfTime // milliseconds

	for rows.Next() {
		var ticker string
		var minDateFromSQL *time.Time
		var maxDateFromSQL *time.Time
		err := rows.Scan(&ticker, &minDateFromSQL, &maxDateFromSQL)
		if err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}

		windowStartTimeNanos, err := utils.NanosFromUTCTime(time.Unix(windowStartTime/1000, (windowStartTime%1000)*1e6).UTC())
		if err != nil {
			return nil, fmt.Errorf("error converting time: %v", err)
		}

		iter, err := utils.GetTrade(conn.Polygon, ticker, windowStartTimeNanos, "asc", models.GTE, 30000)
		if err != nil {
			return nil, fmt.Errorf("error getting trade data: %v", err)
		}

		for iter.Next() {
			if int64(time.Time(iter.Item().ParticipantTimestamp).Unix())*1000 > windowEndTime {
				return tradeDataList, nil
			}

			if !extendedHours {
				timestamp := time.Time(iter.Item().ParticipantTimestamp).In(easternLocation)
				hour := timestamp.Hour()
				minute := timestamp.Minute()
				if hour < 9 || (hour == 9 && minute < 30) || hour >= 16 {
					continue
				}
			}

			tradeDataList = append(tradeDataList, &TradeData{
				Price:      iter.Item().Price,
				Size:       int64(iter.Item().Size),
				Timestamp:  time.Time(iter.Item().ParticipantTimestamp).UnixNano() / int64(time.Millisecond),
				ExchangeId: int32(iter.Item().Exchange),
				Conditions: iter.Item().Conditions,
				Channel:    "", // Set the Channel to an empty string
			})
		}

		if len(tradeDataList) > 0 {
			windowStartTime = tradeDataList[len(tradeDataList)-1].GetTimestamp()
		} else {
			return tradeDataList, nil
		}
	}

	if len(tradeDataList) != 0 {
		return tradeDataList, nil
	}

	return nil, fmt.Errorf("no data found for the specified range")
}

func getQuoteData(conn *utils.Conn, securityId int, timestamp int64, lengthOfTime int64, extendedHours bool) ([]TickData, error) {
	easternLocation, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("issue loading eastern location: %v", err)
	}

	inputTime := time.Unix(timestamp/1000, (timestamp%1000)*1e6).UTC()
	query := `SELECT ticker, minDate, maxDate FROM securities WHERE securityid=$1 AND (minDate <= $2 AND (maxDate IS NULL or maxDate >= $2)) ORDER BY minDate ASC`
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	rows, err := conn.DB.Query(ctx, query, securityId, inputTime.In(easternLocation).Format(time.DateTime))
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("query timed out: %w", err)
		}
		return nil, fmt.Errorf("error executing query: %w", err)
	}
	defer rows.Close()

	var quoteDataList []TickData
	windowStartTime := timestamp              // milliseconds
	windowEndTime := timestamp + lengthOfTime // milliseconds

	for rows.Next() {
		var ticker string
		var minDateFromSQL *time.Time
		var maxDateFromSQL *time.Time
		err := rows.Scan(&ticker, &minDateFromSQL, &maxDateFromSQL)
		if err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}
		tim := time.Unix(windowStartTime/1000, (windowStartTime%1000)*1e6).UTC()
		fmt.Println("test %v", tim)
		windowStartTimeNanos, err := utils.NanosFromUTCTime(tim)
		if err != nil {
			return nil, fmt.Errorf("error converting time: %v", err)
		}
		iter := utils.GetQuote(conn.Polygon, ticker, windowStartTimeNanos, "asc", models.GTE, 30000)
		for iter.Next() {
			if int64(time.Time(iter.Item().ParticipantTimestamp).Unix())*1000 > windowEndTime {
				return quoteDataList, nil
			}

			//quotes should still be added even if not extedned
			/*if !extendedHours {
			    timestamp := time.Time(iter.Item().ParticipantTimestamp).In(easternLocation)
			    hour := timestamp.Hour()
			    minute := timestamp.Minute()
			    if hour < 9 || (hour == 9 && minute < 30) || hour >= 16 {
			        continue
			    }
			}*/

			quoteDataList = append(quoteDataList, &QuoteData{
				BidPrice:  iter.Item().BidPrice,
				AskPrice:  iter.Item().AskPrice,
				BidSize:   int32(iter.Item().BidSize),
				AskSize:   int32(iter.Item().AskSize),
				Timestamp: time.Time(iter.Item().ParticipantTimestamp).UnixNano() / int64(time.Millisecond),
				Channel:   "", // Set the Channel to an empty string
			})
		}
		if len(quoteDataList) == 0 {
			return nil, fmt.Errorf("difw0")
		}

		windowStartTime = quoteDataList[len(quoteDataList)-1].GetTimestamp()
	}

	if len(quoteDataList) != 0 {
		return quoteDataList, nil
	}

	return nil, fmt.Errorf("no data found for the specified range")
}

func getPrevCloseData(conn *utils.Conn, securityId int, timestamp int64) ([]TickData, error) {
	easternLocation, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("issue loading eastern location: %v", err)
	}

	// Use the input timestamp to find the ticker
	inputTime := time.Unix(timestamp/1000, (timestamp%1000)*1e6).In(easternLocation)
	ticker, err := utils.GetTicker(conn, securityId, inputTime)
	if err != nil {
		return nil, fmt.Errorf("error getting ticker: %v", err)
	}

	// Get the next day's timestamp to fetch the close data
	nextDayTime := inputTime.AddDate(0, 0, 1)
	startOfDay := time.Date(nextDayTime.Year(), nextDayTime.Month(), nextDayTime.Day(), 0, 0, 0, 0, easternLocation)
	endOfDay := time.Date(nextDayTime.Year(), nextDayTime.Month(), nextDayTime.Day(), 23, 59, 59, 999999999, easternLocation)

	// Convert the start and end times to models.Millis
	startOfDayMillis := models.Millis(startOfDay)
	endOfDayMillis := models.Millis(endOfDay)

	// Fetch more bars (set limit to a higher number, e.g., 10)
	iter, err := utils.GetAggsData(conn.Polygon, ticker, 1, "day", startOfDayMillis, endOfDayMillis, 10, "desc", true)
	if err != nil {
		return nil, fmt.Errorf("error fetching aggregate data: %v", err)
	}

	var closeDataList []TickData
	for iter.Next() {
		agg := iter.Item()
		closeData := TradeData{
			Price:      agg.Close,
			Size:       0,                                                             // Size is not applicable for close data
			Timestamp:  time.Time(agg.Timestamp).UnixNano() / int64(time.Millisecond), // Use the actual timestamp of each bar
			ExchangeId: 0,                                                             // ExchangeId is not applicable for close data
			Conditions: []int32{},
			Channel:    "",
		}
		closeDataList = append(closeDataList, &closeData)

	}

	if len(closeDataList) > 0 {
		fmt.Println("close data", closeDataList[0].GetTimestamp())
		return closeDataList, nil
	}

	return nil, fmt.Errorf("no close data found for the specified date range")
}
