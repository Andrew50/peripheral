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
	ws   *websocket.Conn
	mu   sync.Mutex
	send chan []byte
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
		client := &Client{
			ws:   ws,
			send: make(chan []byte, 256),
		}
		go client.writePump()

		client.readPump(conn)
	}
}
func (c *Client) subscribe(conn *Conn,channelName string) {
	channelsMutex.Lock()
	defer channelsMutex.Unlock()

	subscribers, exists := channelSubscribers[channelName]
	if !exists {
		subscribers = make(map[*Client]bool)
		channelSubscribers[channelName] = subscribers
	}
	subscribers[c] = true

	if !exists {
		pubsub := conn.Cache.Subscribe(context.Background(), channelName)
		redisSubscriptions[channelName] = pubsub

		go handleRedisChannel(pubsub, channelName)
	}
}
func (c *Client) unsubscribe(channelName string) {
	channelsMutex.Lock()
	defer channelsMutex.Unlock()

	if subscribers, exists := channelSubscribers[channelName]; exists {
		delete(subscribers, c)
		if len(subscribers) == 0 {
			delete(channelSubscribers, channelName)
			if pubsub, ok := redisSubscriptions[channelName]; ok {
				pubsub.Close()
				delete(redisSubscriptions, channelName)
			}
		}
	}
}
func handleRedisChannel(pubsub *redis.PubSub, channelName string) {
	for msg := range pubsub.Channel() {
		channelsMutex.RLock()
		subscribers := channelSubscribers[channelName]
		channelsMutex.RUnlock()

		for client := range subscribers {
			select {
			case client.send <- []byte(msg.Payload):
			default:
				go client.close()
			}
		}
	}
}
func (c *Client) readPump(conn *Conn) {
	defer func() {
		c.close()
		c.ws.Close()
	}()
	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Println("4kltyvk, WebSocket read error:", err)
			}
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
			c.subscribe(conn,clientMsg.ChannelName)
		case "unsubscribe":
			c.unsubscribe(clientMsg.ChannelName)
		default:
			fmt.Println("Unknown Action:", clientMsg.Action)
		}

	}
}
func (c *Client) writePump() {
	defer c.ws.Close()
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				return
			}
			c.mu.Lock()
			err := c.ws.WriteMessage(websocket.TextMessage, message)
			c.mu.Unlock()
			if err != nil {
				log.Println("WebSocket write error:", err)
				return
			}
		}
	}

}
func (c *Client) close() {
	channelsMutex.Lock()
	defer channelsMutex.Unlock()

	for channelName, subscribers := range channelSubscribers {
		if _, ok := subscribers[c]; ok {
			delete(subscribers, c)
			if len(subscribers) == 0 {
				if pubsub, exists := redisSubscriptions[channelName]; exists {
					pubsub.Close()
					delete(redisSubscriptions, channelName)
				}
			}
			delete(channelSubscribers, channelName)
		}
	}
	close(c.send)

}
