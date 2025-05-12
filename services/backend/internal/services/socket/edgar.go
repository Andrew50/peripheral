package socket

import (
	"backend/internal/data"
	"backend/internal/services/marketData"
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
		latestFilings := marketData.GetLatestEdgarFilings()
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
