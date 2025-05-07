package socket

import (
	"backend/internal/data"
    "backend/internal/data/utils"
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
	conn                  *data.Conn
	simulatedTimeStart    int64
	accumulatedActiveTime time.Duration
	lastTickTime          time.Time
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

// FunctionStatusUpdate represents a status update message sent to the client
// during long-running backend operations (e.g., function tool execution).
// It contains a user-friendly message describing the current step.
type FunctionStatusUpdate struct {
	Type        string `json:"type"` // Will be "function_status"
	UserMessage string `json:"userMessage"`
}

// SendFunctionStatus sends a status update about a running function to a specific user.
func SendFunctionStatus(userID int, userMessage string) {
	// Use a default message if the specific one is empty
	messageToSend := userMessage
	if messageToSend == "" {
		// Use a generic message instead of revealing the function name
		messageToSend = "Processing..."
	}

	statusUpdate := FunctionStatusUpdate{
		Type:        "function_status",
		UserMessage: messageToSend,
	}

	jsonData, err := json.Marshal(statusUpdate)
	if err != nil {
		fmt.Printf("Error marshaling function status update: %v\n", err)
		return
	}

	UserToClientMutex.RLock()
	client, ok := UserToClient[userID]
	UserToClientMutex.RUnlock()

	if !ok {
		fmt.Printf("SendFunctionStatus: client not found for userID: %d\n", userID)
		return
	}

	// Send the update non-blockingly
	select {
	case client.send <- jsonData:
		fmt.Printf("Sent status message to user %d: '%s'\n", userID, messageToSend)
	default:
		// This might happen if the client's send buffer is full or the connection is closing.
		// It's usually okay to just drop the status update in this case.
		fmt.Printf("SendFunctionStatus: send channel blocked or closed for userID: %d. Dropping status update.\n", userID)
	}
}

func (c *Client) writePump() {
	// ticker := time.NewTicker(pingPeriod) // Keep connection alive if needed
	defer func() {
		// ticker.Stop() // Stop the ticker if used
		c.ws.Close() // Ensure connection is closed ONLY here on exit
		fmt.Println("writePump exiting, connection closed")
	}()
	for {
		select {
		case message, ok := <-c.send:
			// c.ws.SetWriteDeadline(time.Now().Add(writeWait)) // Set deadline if needed
			if !ok {
				// The send channel was closed. Tell the client.
				fmt.Println("send channel closed, sending close message")
				c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return // Exit writePump
			}

			if err := c.ws.WriteMessage(websocket.TextMessage, message); err != nil {
				fmt.Println("writePump error:", err)
				return // Exit writePump on write error
			}
		/* // Example ping logic if needed
		case <-ticker.C:
			// c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				fmt.Println("writePump ping error:", err)
				return // Exit writePump on ping error
			}
		*/
		case <-c.done: // Add a way to explicitly stop writePump if needed elsewhere
			fmt.Println("writePump received done signal")
			return
		}
	}
}

/*
	"blocking" function that listens to the client webscoket (not polygon) for subscrib

and unsubscribe messages. breaks the loop when the socket is closed
*/
func (c *Client) readPump(conn *data.Conn) {
	defer func() {
		c.close() // Clean up client resources (unsubscribe, remove from maps etc.)
	}()
	// c.ws.SetReadLimit(maxMessageSize) // Set read limit if needed
	// c.ws.SetReadDeadline(time.Now().Add(pongWait)) // Set initial read deadline
	// c.ws.SetPongHandler(func(string) error { c.ws.SetReadDeadline(time.Now().Add(pongWait)); return nil }) // Pong handler to reset deadline

	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Println("4kltyvk, WebSocket read error:", err)
			} else {
				fmt.Println("WebSocket read error (expected close?):", err)
			}
			break // Exit readPump loop on any error
		}

		// Reset read deadline on successful read if using deadlines
		// c.ws.SetReadDeadline(time.Now().Add(pongWait))

		// Process message
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

	// Signal writePump to stop *if* it hasn't already exited due to error/channel close
	// Use select to avoid blocking if done channel is already closed or nil
	select {
	case <-c.done:
		// Already closing or closed
	default:
		// Not closing yet, signal it
		close(c.done)
		fmt.Println("Closed done channel in close()")
	}

	// Stop replay if it's active (moved unlock after potential stopReplay which might lock/unlock)
	replayWasActive := c.replayActive
	c.mu.Unlock() // Unlock before potentially long-running cleanup

	if replayWasActive {
		c.stopReplay() // Needs to happen after unlocking mu if stopReplay uses it
	}

	// Re-lock for map/channel cleanup? Let's assume stopReplay and unsubscribe handle their own locking
	c.mu.Lock()
	// Clear all replayData
	c.replayData = make(map[string]*ReplayData)
	c.replayActive = false
	c.replayPaused = false
	c.simulatedTime = 0
	c.mu.Unlock()

	// Remove the client from all channel subscribers and close Redis subscriptions if needed
	// Assuming unsubscribeRealtime/unsubscribeReplay called by readPump/stopReplay handle this.
	// If not, the logic needs to be here or called explicitly.
	// Let's simplify and assume the specific unsubscribe calls handle channelSubscribers map.

	// Remove the client from the UserToClient map
	UserToClientMutex.Lock()
	for userID, client := range UserToClient {
		if client == c {
			delete(UserToClient, userID)
			break
		}
	}
	UserToClientMutex.Unlock()

}

// HandleWebSocket performs operations related to HandleWebSocket functionality.
func handleWebSocket(conn *data.Conn, ws *websocket.Conn, userID int) {
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


// subscribeSECFilings handles subscription to the SEC filings feed
func (c *Client) subscribeSECFilings(conn *data.Conn) {
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
