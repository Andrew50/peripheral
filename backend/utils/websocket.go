package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
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

/*
return a fuction to be run when /ws endpoint is hit.
this function (when hit) will make the connection a websocket and then
makes a new client. it then starts the goroutine that will handle chan updates caused by the redis pubsub
asynynchrnously and then syncronolously (not really because the server is already running this whole thing as a goroutine)
checks for websocket messages from the frontend.
*/
func WsFrontendHandler(conn *Conn) http.HandlerFunc {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
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

/*
handles updates to the channel of the client. these updates are sent by a goruotine that listens to
pub sub from redis and then iterates through all the subscribeers (possibly one of them being this client).
this function simply sends the message to the frontend. it has to lock to prevent concurrent writes to the socket
*/
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

/*
	"blocking" function that listens to the client webscoket (not polygon) for subscrib

and unsubscribe messages. breaks the loop when the socket is closed
*/
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
		//fmt.Printf("message receieved %s\n", clientMsg)
		os.Stdout.Sync()
		switch clientMsg.Action {
		case "subscribe":
			c.subscribe(conn, clientMsg.ChannelName)
		case "unsubscribe":
			c.unsubscribe(clientMsg.ChannelName)
		default:
			fmt.Println("Unknown Action:", clientMsg.Action)
		}

	}
}

/*
	subscribes the client webscoket (client struct really) to the redis pub/sub

for the requested channel name. if the redis pub sub doenst exist yet then it creates it, and then runs the
goroutine that handles the redis pubsubs. if it already exists the pushing of updates to the subscribers (Client structs /
client webscokets) is already happening. either way it
adds the client to the list of client subsrcibed to that channel (subscribers variable)
*/
func (c *Client) subscribe(conn *Conn, channelName string) {
	channelsMutex.Lock()
	os.Stdout.Sync()

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
	channelsMutex.Unlock()
	go func() {
		initialValue, err := getInitialStreamValue(channelName, conn)
		if err != nil {
			fmt.Println("Error fetching initial value from API:", err)
			return
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		err = c.ws.WriteMessage(websocket.TextMessage, []byte(initialValue))
		if err != nil {
			log.Println("WebSocket write error while sending initial value:", err)
		}
	}()
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

/*
ones of the is ran as a goroutine for each redis pubsub channel (ticker + channel type)
when a message comes it iteraties through the list of clients structs subscribed to the channel
and sends a message to the chan of each which will then be handled by the gourtoune running the writePump
function for that client
*/
func handleRedisChannel(pubsub *redis.PubSub, channelName string) {
    var lastMessage string
	for msg := range pubsub.Channel() {
        if msg.Payload == lastMessage {
			continue // filter out duplicate prints
		}
        lastMessage = msg.Payload
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

/*
	when a client unsubscribes from a channel (ticker + channel type) then it removes the client from the

list of clients subscribed to that channel. if there are no more subscribes then it also removes the subscription
to the red pub sub
*/
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
