package socket

import (
	"backend/internal/data"
	"backend/internal/data/edgar"
	"backend/internal/services/marketdata"
	"encoding/json"
	"fmt"
)

// subscribeSECFilings handles subscription to the SEC filings feed
func (c *Client) subscribeSECFilings(conn *data.Conn) {
	channelName := "sec-filings"

	// Add client to the channel subscribers
	channelsMutex.Lock()
	if _, exists := channelSubscribers[channelName]; !exists {
		channelSubscribers[channelName] = make(map[*Client]bool)
	}
	channelSubscribers[channelName][c] = true
	incListeners(channelName)
	c.addSubscribedChannel(channelName)
	channelsMutex.Unlock()

	// Get the latest filings from the cache
	if conn != nil {
		// Get the latest filings from the cache
		latestFilings := marketdata.GetLatestEdgarFilings()
		if len(latestFilings) <= 0 {
			fmt.Println("No SEC filings available to send initially")
			return
		}
		// Limit to 50 filings if there are more
		if len(latestFilings) > 50 {
			latestFilings = latestFilings[:50]
		}
		// Create a message with channel information
		message := map[string]interface{}{
			"channel": channelName,
			"data":    latestFilings,
		}

		// Send the initial data
		jsonData, err := json.Marshal(message)
		if err == nil {
			c.send <- jsonData
		} else {
			fmt.Println("Error marshaling SEC filings:", err)
		}
	}

}

// unsubscribeSECFilings handles unsubscription from the SEC filings feed
func (c *Client) unsubscribeSECFilings() {
	channelName := "sec-filings"

	channelsMutex.Lock()
	defer channelsMutex.Unlock()

	if subscribers, exists := channelSubscribers[channelName]; exists {
		if _, ok := subscribers[c]; ok {
			delete(subscribers, c)
			decListeners(channelName)
			c.removeSubscribedChannel(channelName)
		}
	}
}

// SECFilingMessage represents a single SEC filing message to be sent over WebSocket
type SECFilingMessage struct {
	Type      string `json:"type"`      // Filing type (e.g., "10-K", "8-K")
	Date      string `json:"date"`      // Filing date as string
	URL       string `json:"url"`       // URL to the filing
	Timestamp int64  `json:"timestamp"` // UTC timestamp in milliseconds
	Ticker    string `json:"ticker"`    // The ticker symbol
	Channel   string `json:"channel"`   // Channel name (always "sec-filings")
}

// BroadcastGlobalSECFiling sends a new global SEC filing to all clients subscribed to the sec-filings channel
func BroadcastGlobalSECFiling(filing edgar.GlobalEDGARFiling) {
	if !hasListeners("sec-filings") {
		return
	}
	filingMessage := SECFilingMessage{
		Type:      filing.Type,
		Date:      filing.Date,
		URL:       filing.URL,
		Timestamp: filing.Timestamp,
		Ticker:    filing.Ticker,
		Channel:   "sec-filings",
	}

	// Create a wrapper with data property to match the expected format
	wrapper := map[string]interface{}{
		"channel": "sec-filings",
		"data":    filingMessage,
	}

	jsonData, err := json.Marshal(wrapper)
	if err != nil {
		fmt.Println("Error marshaling global SEC filing:", err)
		return
	}

	broadcastToChannel("sec-filings", string(jsonData))
}
