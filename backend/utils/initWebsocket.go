package utils

import (
	"encoding/json"
	"fmt"
	"strings"
    "github.com/polygon-io/client-go/rest/models"
)

type TradeData struct {
	Ticker     string  `json:"ticker"`
	Price      float64 `json:"price"`
	Size       int64   `json:"size"`
	Timestamp  int64   `json:"timestamp"`
	Conditions []int32 `json:"conditions"`
	Channel    string  `json:"channel"`
}

type QuoteData struct {
	Ticker    string  `json:"ticker"`
	BidPrice  float64 `json:"bidPrice"`
	AskPrice  float64 `json:"askPrice"`
	BidSize   float64   `json:"bidSize"`
	AskSize   float64   `json:"askSize"`
	Timestamp models.Nanos   `json:"timestamp"`
	Channel   string  `json:"channel"`
}

func getInitialStreamValue(channelName string, conn *Conn) (string, error) {
	// Split the channelName to determine the type (e.g., "AAPL-quote" or "AAPL-trade")
	parts := strings.Split(channelName, "-")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid channel name: %s", channelName)
	}

	ticker := parts[0]
	streamType := parts[1]

    fmt.Println(streamType)
	if streamType == "quote"{
		quote, err := GetLastQuote(conn.Polygon, ticker)
		if err != nil {
			return "", fmt.Errorf("failed to get last quote: %v", err)
		}
		data := QuoteData{
			Ticker:    ticker,
			BidPrice:  quote.BidPrice,
			AskPrice:  quote.AskPrice,
			BidSize:   quote.BidSize,
			AskSize:   quote.AskSize,
			Timestamp: quote.SipTimestamp,
			Channel:   channelName,
		}
		jsonData, err := json.Marshal(data)
		if err != nil {
			return "", fmt.Errorf("error marshaling quote data: %v", err)
		}
		return string(jsonData), nil

    }else if streamType == "slow" || streamType == "fast"{
		price, err := GetLastTrade(conn.Polygon, ticker)
		if err != nil {
			return "", fmt.Errorf("failed to get last trade: %v", err)
		}
		data := TradeData{
			Ticker:     ticker,
			Price:      price,
			Size:       100,
			Timestamp:  0, 
			Conditions: []int32{},
			Channel:    channelName,
		}
		jsonData, err := json.Marshal(data)
		if err != nil {
			return "", fmt.Errorf("error marshaling trade data: %v", err)
		}
		return string(jsonData), nil


    }else{
		return "", fmt.Errorf("unknown stream type: %s", streamType)
	}
}

