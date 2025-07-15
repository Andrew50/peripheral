package socket

import (
	"backend/internal/data"
	"backend/internal/data/polygon"
	"backend/internal/data/postgres"
	"backend/internal/data/utils"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/polygon-io/client-go/rest/models"
)

// TickData represents a structure for handling TickData data.
type TickData interface {
	GetTimestamp() int64
	GetPrice() float64
	GetChannel() string
	SetChannel(channel string)
}

// TradeData represents a structure for handling TradeData data.
type TradeData struct {
	Price             float64 `json:"price"`
	Size              int64   `json:"size"`
	Timestamp         int64   `json:"timestamp"`
	ExchangeID        int     `json:"exchange"`
	Conditions        []int32 `json:"conditions"`
	Channel           string  `json:"channel"`
	ShouldUpdatePrice bool    `json:"shouldUpdatePrice"`
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

// QuoteData represents a structure for handling QuoteData data.
type QuoteData struct {
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

// Combine all ticks (TradeData or QuoteData) in a small window to produce one aggregated tick
func aggregateTicks(ticks []TickData, baseDataType string) TickData {
	if len(ticks) == 0 {
		return nil
	}
	switch baseDataType {
	case "quote":
		// Just return the last quote in the list
		return ticks[len(ticks)-1]

	case "trade":
		// Summation logic for trade sizes
		totalSize := int64(0)
		var lastTrade TradeData
		conditionsMap := make(map[int32]bool)
		var lastValidPrice float64
		hasValidPrice := false

		for _, tick := range ticks {
			if trade, ok := tick.(*TradeData); ok {
				totalSize += trade.Size
				lastTrade = *trade
				for _, condition := range trade.Conditions {
					conditionsMap[condition] = true
				}
				// Track the last valid price (not -1)
				if trade.Price >= 0 {
					lastValidPrice = trade.Price
					hasValidPrice = true
				}
			}
		}

		uniqueConditions := make([]int32, 0, len(conditionsMap))
		for condition := range conditionsMap {
			uniqueConditions = append(uniqueConditions, condition)
		}

		// Use the last valid price if available, otherwise use the last trade's price
		finalPrice := lastTrade.Price
		if hasValidPrice {
			finalPrice = lastValidPrice
		}

		aggregatedTrade := TradeData{
			Price:      finalPrice,
			Size:       totalSize,
			Timestamp:  lastTrade.Timestamp,
			ExchangeID: lastTrade.ExchangeID,
			Conditions: uniqueConditions,
			Channel:    lastTrade.Channel,
		}
		return &aggregatedTrade

	default:
		// Unrecognized type
		return nil
	}
}

func getTradeData(conn *data.Conn, securityID int, timestamp int64, lengthOfTime int64, _ bool) ([]TickData, error) {
	easternLocation, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("issue loading eastern location: %v", err)
	}
	inputTime := time.Unix(timestamp/1000, (timestamp%1000)*1e6).UTC()
	query := `SELECT ticker, minDate, maxDate FROM securities WHERE securityid=$1 
		AND (minDate <= $2 AND (maxDate IS NULL or maxDate >= $2)) ORDER BY minDate ASC`
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	rows, err := conn.DB.Query(ctx, query, securityID, inputTime.In(easternLocation).Format(time.DateTime))
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("query timed out: %w", err)
		}
		return nil, fmt.Errorf("error executing query: %w", err)
	}
	defer rows.Close()

	var tradeDataList []TickData
	windowStartTime := timestamp
	windowEndTime := timestamp + lengthOfTime

	for rows.Next() {
		var ticker string
		var minDateFromSQL *time.Time
		var maxDateFromSQL *time.Time
		if err := rows.Scan(&ticker, &minDateFromSQL, &maxDateFromSQL); err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}

		windowStartTimeNanos, err := utils.NanosFromUTCTime(
			time.Unix(windowStartTime/1000, (windowStartTime%1000)*1e6).UTC(),
		)
		if err != nil {
			return nil, fmt.Errorf("error converting time: %v", err)
		}

		iter, err := polygon.GetTrade(conn.Polygon, ticker, windowStartTimeNanos, "asc", models.GTE, 30000)
		if err != nil {
			return nil, fmt.Errorf("error getting trade data: %v", err)
		}

		for iter.Next() {
			tradeTsMillis := int64(time.Time(iter.Item().ParticipantTimestamp).Unix()) * 1000
			if tradeTsMillis > windowEndTime {
				return tradeDataList, nil
			}
			if !utils.IsTimestampRegularHours(time.Time(iter.Item().ParticipantTimestamp)) {
				continue
			}
			tradeDataList = append(tradeDataList, &TradeData{
				Price:      iter.Item().Price,
				Size:       int64(iter.Item().Size),
				Timestamp:  time.Time(iter.Item().ParticipantTimestamp).UnixNano() / 1e6,
				ExchangeID: iter.Item().Exchange,
				Conditions: iter.Item().Conditions,
				Channel:    "",
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

func getQuoteData(conn *data.Conn, securityID int, timestamp int64, lengthOfTime int64, _ bool) ([]TickData, error) {
	// Very similar logic to getTradeData, skipping some debug prints
	easternLocation, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("issue loading eastern location: %v", err)
	}
	inputTime := time.Unix(timestamp/1000, (timestamp%1000)*1e6).UTC()
	query := `SELECT ticker, minDate, maxDate FROM securities WHERE securityid=$1 
		AND (minDate <= $2 AND (maxDate IS NULL or maxDate >= $2)) ORDER BY minDate ASC`
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	rows, err := conn.DB.Query(ctx, query, securityID, inputTime.In(easternLocation).Format(time.DateTime))
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("query timed out: %w", err)
		}
		return nil, fmt.Errorf("error executing query: %w", err)
	}
	defer rows.Close()

	var quoteDataList []TickData
	windowStartTime := timestamp
	windowEndTime := timestamp + lengthOfTime

	for rows.Next() {
		var ticker string
		var minDateFromSQL *time.Time
		var maxDateFromSQL *time.Time
		if err := rows.Scan(&ticker, &minDateFromSQL, &maxDateFromSQL); err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}
		windowStartTimeNanos, err := utils.NanosFromUTCTime(
			time.Unix(windowStartTime/1000, (windowStartTime%1000)*1e6).UTC(),
		)
		if err != nil {
			return nil, fmt.Errorf("error converting time: %v", err)
		}
		iter := polygon.GetQuote(conn.Polygon, ticker, windowStartTimeNanos, "asc", models.GTE, 30000)
		for iter.Next() {
			quoteTsMillis := int64(time.Time(iter.Item().ParticipantTimestamp).Unix()) * 1000
			if quoteTsMillis > windowEndTime {
				return quoteDataList, nil
			}
			// We do not exclude extended hours quotes
			quoteDataList = append(quoteDataList, &QuoteData{
				BidPrice:  iter.Item().BidPrice,
				AskPrice:  iter.Item().AskPrice,
				BidSize:   int32(iter.Item().BidSize),
				AskSize:   int32(iter.Item().AskSize),
				Timestamp: time.Time(iter.Item().ParticipantTimestamp).UnixNano() / 1e6,
				Channel:   "",
			})
		}
		if len(quoteDataList) == 0 {
			return nil, fmt.Errorf("no quotes found, or none match extended hours logic")
		}
		windowStartTime = quoteDataList[len(quoteDataList)-1].GetTimestamp()
	}
	if len(quoteDataList) != 0 {
		return quoteDataList, nil
	}
	return nil, fmt.Errorf("no data found for the specified range")
}

func getPrevCloseData(conn *data.Conn, securityID int, timestamp int64) ([]TickData, error) {
	easternLocation, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("issue loading eastern location: %v", err)
	}
	inputTime := time.Unix(timestamp/1000, (timestamp%1000)*1e6).In(easternLocation)
	ticker, err := postgres.GetTicker(conn, securityID, inputTime)
	if err != nil {
		return nil, fmt.Errorf("error getting ticker: %v", err)
	}

	// We do a 5-day lookback, get a minute bar, find lastActivityTime, etc.
	startOfDay := time.Date(inputTime.Year(), inputTime.Month(), inputTime.Day(), 0, 0, 0, 0, easternLocation)
	startMillis := models.Millis(startOfDay.AddDate(0, 0, -5))
	endMillis := models.Millis(inputTime)

	iter, err := polygon.GetAggsData(conn.Polygon, ticker, 1, "minute", startMillis, endMillis, 1, "desc", true)
	if err != nil {
		return nil, fmt.Errorf("error fetching recent minute data: %v", err)
	}
	var lastActivityTime time.Time
	for iter.Next() {
		agg := iter.Item()
		lastActivityTime = time.Time(agg.Timestamp)
		break
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("error iterating minute data: %v", err)
	}
	if lastActivityTime.IsZero() {
		return nil, fmt.Errorf("no recent market activity found for %s", ticker)
	}

	lastActivityET := lastActivityTime.In(easternLocation)
	mostRecentSession := time.Date(lastActivityET.Year(), lastActivityET.Month(), lastActivityET.Day(), 16, 0, 0, 0, easternLocation)
	prevSessionDay := mostRecentSession.AddDate(0, 0, -1)
	for prevSessionDay.Weekday() == time.Saturday || prevSessionDay.Weekday() == time.Sunday {
		prevSessionDay = prevSessionDay.AddDate(0, 0, -1)
	}
	prevSessionStart := time.Date(prevSessionDay.Year(), prevSessionDay.Month(), prevSessionDay.Day(), 0, 0, 0, 0, easternLocation)
	prevSessionEnd := time.Date(prevSessionDay.Year(), prevSessionDay.Month(), prevSessionDay.Day(), 23, 59, 59, 999999999, easternLocation)

	iter, err = polygon.GetAggsData(conn.Polygon, ticker, 1, "day",
		models.Millis(prevSessionStart),
		models.Millis(prevSessionEnd), 1, "desc", true)
	if err != nil {
		return nil, fmt.Errorf("error fetching previous session data: %v", err)
	}

	var closeDataList []TickData
	for iter.Next() {
		agg := iter.Item()
		closeData := TradeData{
			Price:      agg.Close,
			Size:       0,
			Timestamp:  time.Time(agg.Timestamp).UnixNano() / 1e6,
			ExchangeID: 0,
			Conditions: []int32{},
			Channel:    "",
		}
		closeDataList = append(closeDataList, &closeData)
	}
	if len(closeDataList) > 0 {
		return closeDataList, nil
	}
	return nil, fmt.Errorf("no close data found for the previous session")
}

// Return the "initial" price/quote/close. The result is JSON, typically used once per subscription.
func getInitialStreamValue(conn *data.Conn, channelName string, timestamp int64) ([]byte, error) {
	parts := strings.Split(channelName, "-")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid channel name format: %s", channelName)
	}
	securityID, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid securityId in channelName: %v", err)
	}
	streamType := parts[1]
	var extendedHours bool
	if len(parts) == 3 {
		extendedHours = (parts[2] == "extended")
	}

	var queryTime time.Time
	if timestamp == 0 {
		queryTime = time.Now()
	} else {
		queryTime = time.Unix(timestamp/1000, (timestamp%1000)*1e6).UTC()
	}
	ticker, err := postgres.GetTicker(conn, securityID, queryTime)
	if err != nil {
		return nil, fmt.Errorf("error getting ticker: %v", err)
	}

	// Based on streamType, fetch the data
	switch streamType {
	case "quote":
		// ...
		bidPrice, askPrice := 0.0, 0.0
		var bidSize, askSize int32
		var quoteTimestamp int64
		if timestamp == 0 {
			// Latest quote
			quote, err := polygon.GetLastQuote(conn.Polygon, ticker)
			if err != nil {
				return nil, fmt.Errorf("could not get last quote: %v", err)
			}
			bidPrice = quote.BidPrice
			askPrice = quote.AskPrice
			bidSize = int32(quote.BidSize)
			askSize = int32(quote.AskSize)
			quoteTimestamp = time.Time(quote.SipTimestamp).UnixNano() / 1e6
		} else {
			// Historical quote
			quote, err := polygon.GetQuoteAtTimestamp(conn, securityID, queryTime)
			if err != nil {
				return nil, fmt.Errorf("could not get quote at timestamp: %v", err)
			}
			bidPrice = quote.BidPrice
			askPrice = quote.AskPrice
			bidSize = int32(quote.BidSize)
			askSize = int32(quote.AskSize)
			quoteTimestamp = time.Time(quote.SipTimestamp).UnixNano() / 1e6
		}
		data := QuoteData{
			BidPrice:  bidPrice,
			AskPrice:  askPrice,
			BidSize:   bidSize,
			AskSize:   askSize,
			Timestamp: quoteTimestamp,
			Channel:   channelName,
		}
		return json.Marshal(data)

	case "slow":
		// For "slow", we basically want the last known trade, possibly adjusting if it was extended/regular
		// ...
		var price float64
		var size int64
		var tradeTimestamp int64
		var conditions []int32

		var trade models.Trade
		if timestamp == 0 {
			latestTrade, err := polygon.GetLastTrade(conn.Polygon, ticker, true)
			if err != nil {
				return nil, fmt.Errorf("failed to get last trade: %v", err)
			}
			trade = models.Trade{
				Price:        latestTrade.Price,
				Size:         latestTrade.Size,
				SipTimestamp: latestTrade.Timestamp,
				Conditions:   latestTrade.Conditions,
				Exchange:     latestTrade.Exchange,
			}
		} else {
			fetchedTrade, err := polygon.GetTradeAtTimestamp(conn, securityID, queryTime, false)
			if err != nil {
				return nil, fmt.Errorf("failed to get trade at timestamp: %v", err)
			}
			trade = fetchedTrade
		}
		tradeTime := time.Time(trade.SipTimestamp)
		size = int64(trade.Size)
		if !extendedHours && !utils.IsTimestampRegularHours(tradeTime) {
			closePrice, err := polygon.GetMostRecentRegularClose(conn.Polygon, ticker, tradeTime)
			if err != nil {
				return nil, fmt.Errorf("error getting close price: %v", err)
			}
			price = closePrice
			conditions = []int32{}
			tradeTimestamp = tradeTime.UnixNano() / 1e6
		} else if extendedHours && utils.IsTimestampRegularHours(tradeTime) {
			// When in extended hours mode but during regular trading hours
			// We should use the daily open price as the reference point
			// This represents how much the price has changed since the open (including pre-market activity)
			dailyOpen, err := polygon.GetDailyOpen(conn.Polygon, ticker, tradeTime)
			if err != nil {
				return nil, fmt.Errorf("error getting daily open: %v", err)
			}
			price = dailyOpen
			conditions = []int32{}
			tradeTimestamp = tradeTime.UnixNano() / 1e6
		} else {
			price = trade.Price
			conditions = trade.Conditions
			tradeTimestamp = tradeTime.UnixNano() / 1e6
		}
		data := TradeData{
			Price:      price,
			Size:       size,
			Timestamp:  tradeTimestamp,
			Conditions: conditions,
			ExchangeID: trade.Exchange,
			Channel:    channelName,
		}
		return json.Marshal(data)

	case "fast":
		// Typically we do not fetch "fast" historical on subscribe
		return nil, nil

	case "close":
		// ...
		if !extendedHours {
			prevCloseSlice, err := getPrevCloseData(conn, securityID, queryTime.UnixNano()/1e6)
			if err != nil {
				return nil, err
			}
			out := struct {
				Price   float64 `json:"price"`
				Channel string  `json:"channel"`
			}{
				Price:   prevCloseSlice[0].GetPrice(),
				Channel: channelName,
			}
			return json.Marshal(out)
		}
		// Extended hours close - for extended hours calculations
		// During regular hours: use the previous day's close
		// During extended hours: use the daily open price
		var referencePrice float64
		if utils.IsTimestampRegularHours(queryTime) {
			// During regular hours, use previous day's close for extended hours calculation
			prevCloseSlice, err := getPrevCloseData(conn, securityID, queryTime.UnixNano()/1e6)
			if err != nil {
				return nil, err
			}
			referencePrice = prevCloseSlice[0].GetPrice()
		} else { // IF EXTENDED HOURS
			// Check if we're in after-hours (post 4:00 PM) or pre-market
			easternLocation, err := time.LoadLocation("America/New_York")
			if err != nil {
				return nil, fmt.Errorf("issue loading eastern location: %v", err)
			}

			queryTimeET := queryTime.In(easternLocation)
			currentHour := queryTimeET.Hour()

			// After market hours (after 16:00 / 4:00 PM)
			if currentHour >= 16 {
				// Get the current day's regular market close price (4:00 PM close)
				today := time.Date(queryTimeET.Year(), queryTimeET.Month(), queryTimeET.Day(), 0, 0, 0, 0, easternLocation)
				marketCloseTime := time.Date(today.Year(), today.Month(), today.Day(), 16, 0, 0, 0, easternLocation)

				// Get the closing price at 4:00 PM
				closePrice, err := polygon.GetMostRecentRegularClose(conn.Polygon, ticker, marketCloseTime)
				if err != nil {
					return nil, fmt.Errorf("error getting today's close price: %v", err)
				}
				referencePrice = closePrice
			} else {
				// Pre-market, use previous day's close (same as regular hours logic)
				// This ensures consistent extended hours % calculation from previous session close
				prevCloseSlice, err := getPrevCloseData(conn, securityID, queryTime.UnixNano()/1e6)
				if err != nil {
					return nil, fmt.Errorf("error getting previous close: %v", err)
				}
				referencePrice = prevCloseSlice[0].GetPrice()
			}
		}

		out := struct {
			Price   float64 `json:"price"`
			Channel string  `json:"channel"`
		}{
			Price:   referencePrice,
			Channel: channelName,
		}
		return json.Marshal(out)
	case "all":
		// Return empty or no data for "all" on initial
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown stream type: %s", streamType)
	}
}
