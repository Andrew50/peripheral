// realtime.go
package socket

import (
	"backend/utils"
	"fmt"
	"os"

	"github.com/gorilla/websocket"
)

/*
	subscribes the client webscoket (client struct really) to the redis pub/sub

for the requested channel name. if the redis pub sub doenst exist yet then it creates it, and then runs the
goroutine that handles the redis pubsubs. if it already exists the pushing of updates to the subscribers (Client structs /
client webscokets) is already happening. either way it
adds the client to the list of client subsrcibed to that channel (subscribers variable)
*/
func (c *Client) subscribeRealtime(conn *utils.Conn, channelName string) {
	channelsMutex.Lock()
	os.Stdout.Sync()
	//splits := strings.Split(channelNameWithTimeType, "-")
	//channelName := fmt.Sprintf("%s-%s", splits[0], splits[1])

	subscribers, exists := channelSubscribers[channelName]
	if !exists {
		subscribers = make(map[*Client]bool)
		channelSubscribers[channelName] = subscribers
	}
	subscribers[c] = true

	channelsMutex.Unlock()
	go func() {
		initialValue, err := getInitialStreamValue(conn, channelName, 0)
		if err != nil {
			fmt.Println("Error fetching initial value from API:", err)
			return
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		err = c.ws.WriteMessage(websocket.TextMessage, []byte(initialValue))
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

/*
ones of the is ran as a goroutine for each redis pubsub channel (ticker + channel type)
when a message comes it iteraties through the list of clients structs subscribed to the channel
and sends a message to the chan of each which will then be handled by the gourtoune running the writePump
function for that client
*/
func broadcastToChannel(channelName string, message string) {
	channelsMutex.RLock()
	defer channelsMutex.RUnlock()

	// Get the list of subscribers for the channel
	subscribers := channelSubscribers[channelName]

	// Broadcast the message to each subscriber
	for client := range subscribers {
		select {
		case client.send <- []byte(message):
		default:
			go client.close()
		}
	}
}
