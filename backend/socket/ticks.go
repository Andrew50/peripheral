package socket

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/polygon-io/client-go/rest/models"
)

type TickData interface {
	GetTimestamp() int64
	GetPrice() float64
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

func (t TradeData) GetPrice() float64 {
	return t.Price
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

func (q QuoteData) GetPrice() float64 {
	return q.BidPrice
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

	// Get the previous day's timestamp to fetch the close data
	prevDayTime := inputTime.AddDate(0, 0, -1)
	startOfDay := time.Date(prevDayTime.Year(), prevDayTime.Month(), prevDayTime.Day(), 0, 0, 0, 0, easternLocation)
	endOfDay := time.Date(prevDayTime.Year(), prevDayTime.Month(), prevDayTime.Day(), 23, 59, 59, 999999999, easternLocation)

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
		return closeDataList, nil
	}

	return nil, fmt.Errorf("no close data found for the specified date range")
}

func getInitialStreamValue(conn *utils.Conn, channelName string, timestamp int64) ([]byte, error) {
	parts := strings.Split(channelName, "-")
	if len(parts) != 2 && len(parts) != 3 { //3 length means extended vs regular hours
		return nil, fmt.Errorf("invalid channel name: %s", channelName)
	}

	securityId, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("d0if02f %v", err)
	}

	streamType := parts[1]
	var extendedHours bool
	if len(parts) == 3 {
		extendedHours = parts[2] == "extended"
	}

	// Determine the query time
	var queryTime time.Time
	if timestamp == 0 {
		queryTime = time.Now()
	} else {
		queryTime = time.Unix(timestamp/1000, (timestamp%1000)*1e6).UTC()
	}

	var ticker string
	ticker, err = utils.GetTicker(conn, securityId, queryTime)
	if err != nil {
		return nil, fmt.Errorf("error getting ticker: %v", err)
	}

	if streamType == "quote" {
		// Define the variables needed for QuoteData
		var bidPrice, askPrice float64
		var bidSize, askSize int32
		var quoteTimestamp int64

		if timestamp == 0 {
			// Get the latest quote
			quote, err := utils.GetLastQuote(conn.Polygon, ticker)
			if err != nil {
				return nil, fmt.Errorf("foi20nf2 %v", err)
			}
			// Assign values from GetLastQuote
			bidPrice = quote.BidPrice
			askPrice = quote.AskPrice
			bidSize = int32(quote.BidSize)
			askSize = int32(quote.AskSize)
			quoteTimestamp = time.Time(quote.SipTimestamp).UnixNano() / int64(time.Millisecond)
		} else {
			// Get the quote at the specified timestamp
			quote, err := utils.GetQuoteAtTimestamp(conn.Polygon, securityId, queryTime)
			if err != nil {
				return nil, fmt.Errorf("failed to get quote at timestamp: %v", err)
			}
			// Assign values from GetQuoteAtTimestamp
			bidPrice = quote.BidPrice
			askPrice = quote.AskPrice
			bidSize = int32(quote.BidSize)
			askSize = int32(quote.AskSize)
			quoteTimestamp = time.Time(quote.SipTimestamp).UnixNano() / int64(time.Millisecond)
		}

		// Create the QuoteData struct using the variables
		data := QuoteData{
			BidPrice:  bidPrice,
			AskPrice:  askPrice,
			BidSize:   bidSize,
			AskSize:   askSize,
			Timestamp: quoteTimestamp,
			Channel:   channelName,
		}

		// Marshal to JSON
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("error marshaling quote data: %v", err)
		}
		return jsonData, nil

	} else if streamType == "slow" {
		// Define the variables needed for TradeData
		var price float64
		var size int64
		var tradeTimestamp int64
		var conditions []int32

		if timestamp == 0 {
			// Get the latest trade
			trade, err := utils.GetLastTrade(conn.Polygon, ticker)
			if err != nil {
				return nil, fmt.Errorf("failed to get last trade: %v", err)
			}

			tradeTime := time.Time(trade.Timestamp)
			if !extendedHours && !utils.IsTimestampRegularHours(tradeTime) { //if regular channel and last trade was during extended
				fmt.Println("regular channel and extended hours trade")
				// If not extended hours and last trade was during extended hours,
				// get the most recent close instead
				closePrice, err := utils.GetMostRecentRegularClose(conn.Polygon, ticker)
				if err != nil {
					return nil, fmt.Errorf("error getting close price: %v", err)
				}
				price = closePrice
				size = 0
				tradeTimestamp = tradeTime.UnixNano() / int64(time.Millisecond)
				conditions = []int32{}
			} else if extendedHours && utils.IsTimestampRegularHours(tradeTime) { //if extended channel and last trade was during regular hours
				fmt.Println("extended hours and regular hours trade")
				// Get the most recent open price for extended hours
				openPrice, err := utils.GetMostRecentExtendedHoursClose(conn.Polygon, ticker)
				if err != nil {
					return nil, fmt.Errorf("error getting open price: %v", err)
				}
				price = openPrice
				size = 0
				tradeTimestamp = tradeTime.UnixNano() / int64(time.Millisecond)
				conditions = []int32{}
			} else {
				fmt.Println("god")

				// Use the last trade data
				price = trade.Price
				size = int64(trade.Size)
				tradeTimestamp = tradeTime.UnixNano() / int64(time.Millisecond)
				conditions = trade.Conditions
			}
			fmt.Println("slow", extendedHours, ticker, price)
		} else { //need to use same if logic as if timestamp == 0 TODO !!!!!
			// Get the trade at the specified timestamp
			trade, err := utils.GetTradeAtTimestamp(conn.Polygon, securityId, queryTime)
			if err != nil {
				return nil, fmt.Errorf("failed to get trade at timestamp: %v", err)
			}
			// Assign values from GetTradeAtTimestamp
			price = trade.Price
			size = int64(trade.Size)
			tradeTimestamp = time.Time(trade.SipTimestamp).UnixNano() / int64(time.Millisecond)
			conditions = trade.Conditions
		}

		// Create the TradeData struct using the variables
		data := TradeData{
			Price:      price,
			Size:       size,
			Timestamp:  tradeTimestamp,
			Conditions: conditions,
			Channel:    channelName,
		}

		// Marshal to JSON
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("error marshaling trade data: %v", err)
		}
		return jsonData, nil

	} else if streamType == "fast" {
		return nil, nil
	} else if streamType == "close" {
		if !extendedHours {
			// Get previous day's close as before
			close, err := getPrevCloseData(conn, securityId, queryTime.UnixNano()/int64(time.Millisecond))
			if err != nil {
				return nil, fmt.Errorf("error getting prev close: %v", err)
			}
			data := struct {
				Price   float64 `json:"price"`
				Channel string  `json:"channel"`
			}{
				Price:   close[0].GetPrice(),
				Channel: channelName,
			}
			fmt.Println("close", extendedHours, ticker, close[0].GetPrice())
			jsonData, err := json.Marshal(data)
			if err != nil {
				return nil, fmt.Errorf("error marshaling prev close data: %v", err)
			}
			return jsonData, nil
		} else {
			// Get current day's most recent regular hours close
			closePrice, err := utils.GetMostRecentRegularClose(conn.Polygon, ticker)
			fmt.Println("closePrice", ticker, closePrice)
			if err != nil {
				return nil, fmt.Errorf("failed to get current regular hours close: %v", err)
			}
			data := struct {
				Price   float64 `json:"price"`
				Channel string  `json:"channel"`
			}{
				Price:   closePrice,
				Channel: channelName,
			}
			fmt.Println("close2", extendedHours, ticker, closePrice)
			jsonData, err := json.Marshal(data)
			if err != nil {
				return nil, fmt.Errorf("error marshaling current close data: %v", err)
			}
			return jsonData, nil
		}
	} else if streamType == "all" {
		// Return an empty response for "all" stream type
		return nil, nil
	} else {
		return nil, fmt.Errorf("unknown stream type: %s", streamType)
	}
	return nil, nil
}
