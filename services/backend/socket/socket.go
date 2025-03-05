// socket.go
package socket

import (
	"backend/utils"
	"container/list"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

//

var (
	channelsMutex           sync.RWMutex
	channelSubscriberCounts = make(map[string]int)
	channelSubscribers      = make(map[string]map[*Client]bool)
	UserToClient            = make(map[int]*Client)
	UserToClientMutex       sync.RWMutex
	//redisSubscriptions      = make(map[string]*redis.PubSub)
	lastTimestampUpdate time.Time
	timestampMutex      sync.RWMutex
)

// ReplayData represents a structure for handling ReplayData data.
type ReplayData struct {
	channelTypes []string
	data         *list.List
	refilling    bool
	baseDataType string
	securityId   int
}

// Client represents a structure for handling Client data.
type Client struct {
	ws                    *websocket.Conn
	mu                    sync.Mutex
	send                  chan []byte
	done                  chan struct{}
	replayActive          bool
	replayPaused          bool
	replaySpeed           float64
	replayExtendedHours   bool
	loopRunning           bool
	buffer                int64
	simulatedTime         int64
	replayData            map[string]*ReplayData
	conn                  *utils.Conn
	simulatedTimeStart    int64
	accumulatedActiveTime time.Duration
	lastTickTime          time.Time
	accumulatedPauseTime  time.Duration
}

/*
return a fuction to be run when /ws endpoint is hit.
this function (when hit) will make the connection a websocket and then
makes a new client. it then starts the goroutine that will handle chan updates caused by the redis pubsub
asynynchrnously and then syncronolously (not really because the server is already running this whole thing as a goroutine)
checks for websocket messages from the frontend.
*/
/*
handles updates to the channel of the client. these updates are sent by a goruotine that listens to
pub sub from redis and then iterates through all the subscribeers (possibly one of them being this client).
this function simply sends the message to the frontend. it has to lock to prevent concurrent writes to the socket
*/

func getChannelNameType(timestamp int64) string {

	if utils.IsTimestampRegularHours(time.Unix(timestamp/1000, 0)) { //might not need / 1000
		return "regular"
	} else {
		return "extended"
	}
}

// AlertMessage represents a structure for handling AlertMessage data.
type AlertMessage struct {
	AlertID    int    `json:"alertId"`
	Timestamp  int64  `json:"timestamp"`
	SecurityID int    `json:"securityId"`
	Message    string `json:"message"`
	Channel    string `json:"channel"`
	Ticker     string `json:"ticker"`
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

// BroadcastSECFiling sends a new SEC filing to all clients subscribed to the sec-filings channel
func BroadcastSECFiling(filing utils.EDGARFiling, ticker string) {
	filingMessage := SECFilingMessage{
		Type:      filing.Type,
		Date:      filing.Date.Format("2006-01-02"),
		URL:       filing.URL,
		Timestamp: filing.Timestamp,
		Ticker:    ticker,
		Channel:   "sec-filings",
	}

	jsonData, err := json.Marshal(filingMessage)
	if err != nil {
		fmt.Println("Error marshaling SEC filing:", err)
		return
	}

	broadcastToChannel("sec-filings", string(jsonData))
}

// BroadcastGlobalSECFiling sends a new global SEC filing to all clients subscribed to the sec-filings channel
func BroadcastGlobalSECFiling(filing utils.GlobalEDGARFiling) {
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

// SendAlertToUser performs operations related to SendAlertToUser functionality.
func SendAlertToUser(userID int, alert AlertMessage) {
	jsonData, err := json.Marshal(alert)
	if err == nil {
		UserToClientMutex.RLock()
		client, ok := UserToClient[userID]
		UserToClientMutex.RUnlock()
		if !ok {
			fmt.Println("client not found")
			return
		}
		client.send <- jsonData
	} else {
		fmt.Println("Error marshaling alert:", err)
	}
}
func (c *Client) writePump() {
	defer c.ws.Close()
	for message := range c.send {
		if err := c.ws.WriteMessage(websocket.TextMessage, message); err != nil {
			return
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
			Action        string   `json:"action"`
			ChannelName   string   `json:"channelName"`
			Timestamp     *int64   `json:"timestamp,omitempty"`
			Speed         *float64 `json:"speed,omitempty"`
			ExtendedHours *bool    `json:"extendedHours,omitempty"`
		}
		if err := json.Unmarshal(message, &clientMsg); err != nil {
			fmt.Println("Invalid message format", err)
			continue
		}
		os.Stdout.Sync()
		//fmt.Printf("clientMsg.Action: %v %v\n", clientMsg.Action, clientMsg.ChannelName)
		switch clientMsg.Action {
		case "subscribe-sec-filings":
			// Special handler for SEC filings subscription
			c.subscribeSECFilings(conn)
		case "unsubscribe-sec-filings":
			// Special handler for SEC filings unsubscription
			c.unsubscribeSECFilings()
		case "subscribe":
			if c.replayActive {
				c.subscribeReplay(clientMsg.ChannelName)
			} else {
				c.subscribeRealtime(conn, clientMsg.ChannelName)
			}
		case "unsubscribe":
			if c.replayActive {
				c.unsubscribeReplay(clientMsg.ChannelName)
			} else {
				c.unsubscribeRealtime(clientMsg.ChannelName)
			}
		case "replay":
			fmt.Println("replay request")
			if !c.replayActive {
				if clientMsg.Timestamp == nil {
					fmt.Println("ERR-------------------------nil timestamp")
				} else {
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
			fmt.Println("realtime request")
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
	fmt.Printf("replay started at %v\n", c.simulatedTime)
	c.mu.Lock()
	c.replayActive = true
	c.replayPaused = false
	c.simulatedTimeStart = c.simulatedTime
	c.accumulatedActiveTime = 0
	c.lastTickTime = time.Now()
	c.mu.Unlock()
	for channelName := range channelSubscribers {
		fmt.Println(channelName)
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
			c.subscribeRealtime(c.conn, channelName+"-"+channelType)
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
	close(c.done)
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
			/*
				if len(subscribers) == 0 {
					if pubsub, exists := redisSubscriptions[channelName]; exists {
						pubsub.Close()
						delete(redisSubscriptions, channelName)
					}
				}*/

			// Clean up the channelSubscribers map
			delete(channelSubscribers, channelName)
		}
	}

	// Remove the client from the UserToClient map
	UserToClientMutex.Lock()
	for userID, client := range UserToClient {
		if client == c {
			delete(UserToClient, userID)
			break
		}
	}
	UserToClientMutex.Unlock()

	// Close the send channel to stop the writePump
	fmt.Println("closing channel")
	close(c.send)

	// Close the WebSocket connection
	if err := c.ws.Close(); err != nil {
		fmt.Println("Error closing WebSocket connection:", err)
	}
}

// HandleWebSocket performs operations related to HandleWebSocket functionality.
func HandleWebSocket(conn *utils.Conn, ws *websocket.Conn, userID int) {
	client := &Client{
		ws:                  ws,
		send:                make(chan []byte, 3000),
		done:                make(chan struct{}),
		replayActive:        false,
		replayPaused:        false,
		replaySpeed:         1.0,
		replayExtendedHours: false,
		simulatedTime:       0,
		replayData:          make(map[string]*ReplayData),
		conn:                conn,
		buffer:              10000,
		loopRunning:         false,
	}

	// Store the client in the userToClient map
	UserToClientMutex.Lock()
	UserToClient[userID] = client
	UserToClientMutex.Unlock()

	// Start the writePump and readPump goroutines
	go client.writePump()
	client.readPump(conn)
}

func broadcastTimestamp() {
	timestampMutex.Lock()
	now := time.Now()
	if now.Sub(lastTimestampUpdate) >= TimestampUpdateInterval {
		timestamp := now.UnixNano() / int64(time.Millisecond)
		timestampUpdate := map[string]interface{}{
			"channel":   "timestamp",
			"timestamp": timestamp,
		}
		jsonData, err := json.Marshal(timestampUpdate)
		if err == nil {
			// Broadcast to all connected clients
			for client := range UserToClient {
				if c := UserToClient[client]; c != nil {
					select {
					case c.send <- jsonData:
					default:
						// Channel full or closed
					}
				}
			}
		}
		lastTimestampUpdate = now
	}
	timestampMutex.Unlock()
}

// subscribeSECFilings handles subscription to the SEC filings feed
func (c *Client) subscribeSECFilings(conn *utils.Conn) {
	channelName := "sec-filings"
	fmt.Printf("\nGot subscription to SEC filings feed\n")

	// Add client to the channel subscribers
	channelsMutex.Lock()
	if _, exists := channelSubscribers[channelName]; !exists {
		channelSubscribers[channelName] = make(map[*Client]bool)
	}
	channelSubscribers[channelName][c] = true
	channelSubscriberCounts[channelName]++
	channelsMutex.Unlock()

	// Get the latest filings from the cache
	if conn != nil {
		// Get the latest filings from the cache
		latestFilings := utils.GetLatestEdgarFilings()

		// Limit to 50 filings if there are more
		if len(latestFilings) > 50 {
			latestFilings = latestFilings[:50]
		}

		if len(latestFilings) > 0 {
			fmt.Printf("Found %d SEC filings to send initially\n", len(latestFilings))

			// Debug: Print the first filing's timestamp
			if len(latestFilings) > 0 {
				fmt.Printf("First filing timestamp: %d\n", latestFilings[0].Timestamp)
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
				fmt.Printf("Sent %d initial SEC filings to client\n", len(latestFilings))
			} else {
				fmt.Println("Error marshaling SEC filings:", err)
			}
		} else {
			fmt.Println("No SEC filings available to send initially")
		}
	}

	fmt.Printf("Client subscribed to SEC filings feed, %d subscribers\n",
		channelSubscriberCounts[channelName])
}

// unsubscribeSECFilings handles unsubscription from the SEC filings feed
func (c *Client) unsubscribeSECFilings() {
	channelName := "sec-filings"

	channelsMutex.Lock()
	defer channelsMutex.Unlock()

	if subscribers, exists := channelSubscribers[channelName]; exists {
		if _, ok := subscribers[c]; ok {
			delete(subscribers, c)
			channelSubscriberCounts[channelName]--
			fmt.Printf("Client unsubscribed from SEC filings feed, %d subscribers remaining\n",
				channelSubscriberCounts[channelName])
		}
	}
}

// /socket.go
