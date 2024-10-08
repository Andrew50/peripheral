package socket



import (
	"encoding/json"
    "time"
    "os"
    "backend/utils"
	"fmt"
	"net/http"
	"sync"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
    "container/list"
)


//

var (
	channelsMutex           sync.RWMutex
	channelSubscriberCounts = make(map[string]int)
	channelSubscribers      = make(map[string]map[*Client]bool)
	redisSubscriptions      = make(map[string]*redis.PubSub)
)

type ReplayData struct {
    channelTypes []string
    data *list.List
    refilling bool
    baseDataType string
    securityId int
}
type Client struct {
	ws   *websocket.Conn
	mu   sync.Mutex
	send chan []byte
    replayActive bool
    replayPaused bool
    replaySpeed float64
    replayExtendedHours bool
    loopRunning bool
    buffer int64
    simulatedTime int64
    replayData map[string]*ReplayData
    conn *utils.Conn
}

/*
return a fuction to be run when /ws endpoint is hit.
this function (when hit) will make the connection a websocket and then
makes a new client. it then starts the goroutine that will handle chan updates caused by the redis pubsub
asynynchrnously and then syncronolously (not really because the server is already running this whole thing as a goroutine)
checks for websocket messages from the frontend.
*/
func WsHandler(conn *utils.Conn) http.HandlerFunc {
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
			send: make(chan []byte, 1000),
            replayActive: false,
            replayPaused: false,
            replaySpeed: 1.0,
            replayExtendedHours: false,
            simulatedTime: 0,
            replayData: make(map[string]*ReplayData),
            conn: conn,
            loopRunning: false,
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
    lastTime := time.Now() // Store the time of the last message

    for {
        select {
        case message, ok := <-c.send:
            if !ok {
                fmt.Println("Channel closed, exiting writePump")
                return
            }

            // Calculate the time since the last message
            if false{
                now := time.Now()
                interval := now.Sub(lastTime).Milliseconds()
                lastTime = now

                // Print the interval between messages
                fmt.Printf("Message interval: %d ms\n", interval)

                // Lock and write the message
            }
            c.mu.Lock()
            err := c.ws.WriteMessage(websocket.TextMessage, message)
            c.mu.Unlock()
            if err != nil {
                fmt.Println("WebSocket write error:", err)
                return
            }
        }
    }
}
/*
	"blocking" function that listens to the client webscoket (not polygon) for subscrib

and unsubscribe messages. breaks the loop when the socket is closed
*/
func (c *Client) readPump(conn *utils.Conn) {
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
            Timestamp   *int64  `json:"timestamp,omitempty"`
            Speed   *float64  `json:"speed,omitempty"`
            ExtendedHours *bool `json:"extendedHours,omitempty"`
		}
		if err := json.Unmarshal(message, &clientMsg); err != nil {
			fmt.Println("Invalid message format", err)
			continue
		}
        os.Stdout.Sync()
        //fmt.Println(clientMsg)
		switch clientMsg.Action {
        case "subscribe":
            if c.replayActive {
                c.subscribeReplay(clientMsg.ChannelName)
            }else{
                c.subscribeRealtime(conn, clientMsg.ChannelName)
            }
        case "unsubscribe":
            if c.replayActive {
                c.unsubscribeReplay(clientMsg.ChannelName)
            }else{
                c.unsubscribeRealtime(clientMsg.ChannelName)
            }
        case "replay":
            fmt.Println("replay request")
            if !c.replayActive {
                if clientMsg.Timestamp == nil {
                    fmt.Println("ERR-------------------------nil timestamp")
                }else{
                    c.simulatedTime = *(clientMsg.Timestamp)
                    c.realtimeToReplay()
                }
            }
        case "pause":
            c.pauseReplay()
        case "play":
            c.playReplay()
        case "speed":
            c.setReplaySpeed(*(clientMsg.Speed))
        case "realtime":
            if c.replayActive {
                c.replayToRealtime()
            }
        case "nextOpen":
            if c.replayActive {
                c.jumpToNextMarketOpen()
            }
        case "setExtended":
            if c.replayActive {
                c.replayExtendedHours = *(clientMsg.ExtendedHours)
            }
        default:
            fmt.Println("Unknown Action:", clientMsg.Action)
        }
	}
}


func (c *Client) realtimeToReplay() {
    c.mu.Lock()
    c.replayActive = true
    c.mu.Unlock()
    for channelName := range channelSubscribers {
        if _, isSubscribed := channelSubscribers[channelName][c]; isSubscribed {
            c.unsubscribeRealtime(channelName)
            c.subscribeReplay(channelName)
        }
    }
}
func (c *Client) replayToRealtime() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.stopReplay()
    for channelName, replayData := range c.replayData {
        c.unsubscribeReplay(channelName)
        for _, channelType := range replayData.channelTypes {
            c.subscribeRealtime(c.conn, channelName + "-" + channelType)
        }
    }
}

/*
	when a client unsubscribes from a channel (ticker + channel type) then it removes the client from the

list of clients subscribed to that channel. if there are no more subscribes then it also removes the subscription
to the red pub sub
*/
func (c *Client) close() {
	// Lock to ensure thread safety
	c.mu.Lock()
	defer c.mu.Unlock()

	// Stop replay if it's active
	if c.replayActive {
		c.stopReplay()
	}

	// Clear all replayData
	c.replayData = make(map[string]*ReplayData)
	c.replayActive = false
	c.replayPaused = false
	c.simulatedTime = 0

	// Remove the client from all channel subscribers and close Redis subscriptions if needed
	channelsMutex.Lock()
	defer channelsMutex.Unlock()
	for channelName, subscribers := range channelSubscribers {
		if _, ok := subscribers[c]; ok {
			// Remove the client from the list of subscribers
			delete(subscribers, c)

			// If there are no more subscribers, close the Redis Pub/Sub and clean up
			if len(subscribers) == 0 {
				if pubsub, exists := redisSubscriptions[channelName]; exists {
					pubsub.Close()
					delete(redisSubscriptions, channelName)
				}
			}

			// Clean up the channelSubscribers map
			delete(channelSubscribers, channelName)
		}
	}

	// Close the send channel to stop the writePump
    fmt.Println("closing channel")
	close(c.send)

	// Close the WebSocket connection
	if err := c.ws.Close(); err != nil {
		fmt.Println("Error closing WebSocket connection:", err)
	}
}
