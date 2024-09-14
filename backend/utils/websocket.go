package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
)

var (
	channelsMutex           sync.RWMutex
	channelSubscriberCounts = make(map[string]int)
	channelSubscribers      = make(map[string]map[*Client]bool)
	redisSubscriptions      = make(map[string]*redis.PubSub)
)

type Client struct {
	ws       *websocket.Conn
	mu       sync.Mutex
	channels map[string]*redis.PubSub
}

func WsFrontendHandler(conn *Conn) http.HandlerFunc {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			// Adjust this according to your CORS policy
			return true
		},
	}
	return func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			fmt.Println("failed to upgrade to websocket: ", err)
			return
		}
		defer ws.Close()
		client := &Client{
			ws:       ws,
			channels: make(map[string]*redis.PubSub),
		}
		redisMessages := make(chan *redis.Message)

		go func() {
			for {
				_, message, err := ws.ReadMessage()
				if err != nil {
					log.Println("WebSocket read error:", err)
					break
				}
				var clientMsg struct {
					Action      string `json:"action"`
					ChannelName string `json:"channelName"`
				}
				if err := json.Unmarshal(message, &clientMsg); err != nil {
					fmt.Println("Invalid message format", err)
					continue
				}
				switch clientMsg.Action {
				case "subscribe":
					client.subscribe(conn.Cache, clientMsg.ChannelName, redisMessages)
				case "unsubscribe":
					client.unsubscribe(clientMsg.ChannelName)
				default:
					fmt.Println("4lgkdvv, Unknown action received from WebSocket client:", clientMsg.Action)

				}
			}
			client.close()
		}()
		for msg := range redisMessages {
			err := ws.WriteMessage(websocket.TextMessage, []byte(msg.Payload))
			if err != nil {
				fmt.Println("l4ifjv, WebSocket write error:", err)
				break
			}
		}
	}
}
func (c *Client) subscribe(redisClient *redis.Client, channelName string, redisMessages chan<- *redis.Message) {
	c.mu.Lock()
	if _, exists := c.channels[channelName]; exists {
		c.mu.Unlock()
		return
	}

	pubsub := redisClient.Subscribe(context.Background(), channelName)
	c.channels[channelName] = pubsub
	c.mu.Unlock()

	channelsMutex.Lock()
	channelSubscriberCounts[channelName]++
	channelsMutex.Unlock()

	go func() {
		for msg := range pubsub.Channel() {
			redisMessages <- msg
		}
	}()

}
func (c *Client) unsubscribe(channelName string) {
	c.mu.Lock()
	pubsub, exists := c.channels[channelName]
	if !exists {
		c.mu.Unlock()
		return
	}
	pubsub.Close()
	delete(c.channels, channelName)
	c.mu.Unlock()

	channelsMutex.Lock()
	channelSubscriberCounts[channelName]--
	if channelSubscriberCounts[channelName] <= 0 {
		delete(channelSubscriberCounts, channelName)
	}

}
func (c *Client) close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for channelName, pubsub := range c.channels {
		pubsub.Close()

		channelsMutex.Lock()
		channelSubscriberCounts[channelName]--
		if channelSubscriberCounts[channelName] <= 0 {
			delete(channelSubscriberCounts, channelName)
		}
		channelsMutex.Unlock()
	}
	c.channels = nil
	c.ws.Close()
}
func GetChannelsWithSubscribers() []string {
	channelsMutex.RLock()
	defer channelsMutex.RUnlock()

	var channels []string

	for channelName := range channelSubscriberCounts {
		channels = append(channels, channelName)
	}

	return channels

}
