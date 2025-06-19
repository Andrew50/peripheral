package socket

import (
	"backend/internal/data"
)

// Subscribes the client WebSocket to the requested channel in "realtime" mode
func (c *Client) subscribeRealtime(conn *data.Conn, channelName string) {
	if _, exists := c.subscribedChannels[channelName]; exists {
		return
	}
	channelsMutex.Lock()
	// Note: os.Stdout.Sync() removed as it doesn't work properly in Docker containers
	// and is not necessary for WebSocket subscriptions
	subscribers, exists := channelSubscribers[channelName]
	if !exists {
		subscribers = make(map[*Client]bool)
		channelSubscribers[channelName] = subscribers
	}
	subscribers[c] = true
	channelsMutex.Unlock()
	c.addSubscribedChannel(channelName)
	incListeners(channelName)
	go func() {
		initialValue, fetchErr := getInitialStreamValue(conn, channelName, 0)
		//fmt.Println("\n\ninitialValue", initialValue, string(initialValue))
		if fetchErr != nil {
			////fmt.Println("Error fetching initial value from API:", fetchErr)
			return
		}

		// Send to the client via the send channel (thread-safe)
		select {
		case c.send <- initialValue:
			// Successfully sent
		default:
			// Channel is full or closed, skip this message
		}
	}()
}

func (c *Client) unsubscribeRealtime(channelName string) {
	channelsMutex.Lock()
	defer channelsMutex.Unlock()

	if subscribers, exists := channelSubscribers[channelName]; exists {
		delete(subscribers, c)
		if len(subscribers) == 0 {
			delete(channelSubscribers, channelName)
		}
	}
	decListeners(channelName)
	c.removeSubscribedChannel(channelName)
}

// Broadcast a message to all clients subscribed to the given channelName
func broadcastToChannel(channelName string, message string) {

	if !hasListeners(channelName) {
		return
	}
	channelsMutex.RLock()
	defer channelsMutex.RUnlock()

	subscribers := channelSubscribers[channelName]
	for client := range subscribers {
		select {
		case client.send <- []byte(message):
		default:
			go client.close()
		}
	}
}
