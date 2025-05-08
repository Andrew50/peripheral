package socket

import (
	"backend/internal/data"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
)

// Subscribes the client WebSocket to the requested channel in "realtime" mode
func (c *Client) subscribeRealtime(conn *data.Conn, channelName string) {
	channelsMutex.Lock()
	os.Stdout.Sync()

	subscribers, exists := channelSubscribers[channelName]
	if !exists {
		subscribers = make(map[*Client]bool)
		channelSubscribers[channelName] = subscribers
	}
	subscribers[c] = true
	channelsMutex.Unlock()

	go func() {
		// 1) Check Redis for cached initial value
		cacheKey := "channelCache:" + channelName
		ctx := context.Background()
		cachedValue, err := conn.Cache.Get(ctx, cacheKey).Result()
		if err == nil && cachedValue != "" {
			// Cache hit -> send to client
			c.mu.Lock()
			_ = c.ws.WriteMessage(websocket.TextMessage, []byte(cachedValue))
			c.mu.Unlock()
			return
		} else if err != nil && err != redis.Nil {
			// Only log real errors. redis.Nil just means "not found."
			fmt.Println("Error reading Redis cache:", err)
		}

		// 2) Cache miss -> fetch from Polygon / DB
		initialValue, fetchErr := getInitialStreamValue(conn, channelName, 0)
		if fetchErr != nil {
			fmt.Println("Error fetching initial value from API:", fetchErr)
			return
		}
		// 3) Store in Redis so next subscription can get it quickly
		setErr := conn.Cache.Set(ctx, cacheKey, string(initialValue), 5*time.Minute).Err()
		if setErr != nil {
			fmt.Println("Error writing Redis cache:", setErr)
		}

		// 4) Send to the client
		c.mu.Lock()
		defer c.mu.Unlock()
		err = c.ws.WriteMessage(websocket.TextMessage, initialValue)
		if err != nil {
			fmt.Println("WebSocket write error while sending initial value:", err)
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
}

// Broadcast a message to all clients subscribed to the given channelName
func broadcastToChannel(channelName string, message string) {
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
