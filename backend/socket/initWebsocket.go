package socket

import (
	"encoding/json"
    "time"
	"fmt"
	"strings"
    "backend/utils"
    "strconv"
)



func getInitialStreamValue(conn *utils.Conn, channelName string, timestamp int64) ([]byte, error) {
	parts := strings.Split(channelName, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid channel name: %s", channelName)
	}

	securityId, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("d0if02f %v\n", err)
	}

	streamType := parts[1]

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

	} else if streamType == "slow" || streamType == "fast" {
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
			// Assign values from GetLastTrade
			price = trade.Price
			size = int64(trade.Size)
			tradeTimestamp = time.Time(trade.Timestamp).UnixNano() / int64(time.Millisecond)
			conditions = trade.Conditions
		} else {
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

    } else if streamType == "close" {

	} else if streamType == "all" {
		// Return an empty response for "all" stream type
		return nil, nil
	} else {
		return nil, fmt.Errorf("unknown stream type: %s", streamType)
	}
    return nil, nil
}

