package socket

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	//"log"
	"sync"
	"time"

	polygonws "github.com/polygon-io/client-go/websocket"
	"github.com/polygon-io/client-go/websocket/models"
)

var nextDispatchTimes = struct {
	sync.RWMutex
	times map[string]time.Time
}{times: make(map[string]time.Time)}

var useAlerts bool

const slowRedisTimeout = 1 * time.Second // Adjust the timeout as needed

var tickerToSecurityID map[string]int
var tickerToSecurityIDLock sync.RWMutex

var polygonWSConn *polygonws.Client

const TimestampUpdateInterval = 2 * time.Second

var (
	lastTickTimestamp   int64
	tickTimestampMutex  sync.RWMutex
	lastTimestampUpdate time.Time
	timestampMutex      sync.RWMutex
)

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

// StreamPolygonDataToRedis performs operations related to StreamPolygonDataToRedis functionality.
func StreamPolygonDataToRedis(conn *data.Conn, polygonWS *polygonws.Client) {
	err := polygonWS.Subscribe(polygonws.StocksQuotes)
	if err != nil {
		//log.Println("niv0: ", err)
		return
	}
	err = polygonWS.Subscribe(polygonws.StocksTrades)
	if err != nil {
		//log.Println("Error subscribing to Polygon WebSocket: ", err)
		return
	}
	err = polygonWS.Subscribe(polygonws.StocksMinAggs)
	if err != nil {
		//log.Println("Error subscribing to Polygon WebSocket: ", err)
		return
	}
	err = polygonWS.Subscribe(polygonws.StocksSecAggs)
	if err != nil {
		log.Println("Error subscribing to Polygon WebSocket: ", err)
		return
	}

	InitOHLCVBuffer(conn)

	// Add timestamp ticker
	timestampTicker := time.NewTicker(TimestampUpdateInterval)
	defer timestampTicker.Stop()

	for {
		select {
		case <-timestampTicker.C:
			broadcastTimestamp()
		case out := <-polygonWS.Output():
			var symbol string
			var timestamp int64

			switch msg := out.(type) {
			case models.EquityAgg:
				symbol = msg.Symbol
				timestamp = msg.EndTimestamp
			case models.EquityTrade:
				symbol = msg.Symbol
				timestamp = msg.Timestamp
			case models.EquityQuote:
				symbol = msg.Symbol
				timestamp = msg.Timestamp
			default:
				//j//log.Println("Unknown message type received")
				continue
			}

			// Update the last tick timestamp
			tickTimestampMutex.Lock()
			if timestamp > lastTickTimestamp {
				lastTickTimestamp = timestamp
			}
			tickTimestampMutex.Unlock()

			tickerToSecurityIDLock.RLock()
			securityID, exists := tickerToSecurityID[symbol]
			tickerToSecurityIDLock.RUnlock()
			if !exists {
				//log.Printf("Symbol %s not found in tickerToSecurityID map\n", symbol)
				continue
			}
			switch msg := out.(type) {
			case models.EquityAgg:
				if msg.EndTimestamp-msg.StartTimestamp == 1000 {
					ohlcvBuffer.addBar(msg.EndTimestamp, securityID, msg)
				}

				/* alerts.appendAggregate(securityId,msg.Open,msg.High,msg.Low,msg.Close,msg.Volume)*/
			case models.EquityTrade:
				channelNameType := getChannelNameType(msg.Timestamp)
				fastChannelName := fmt.Sprintf("%d-fast-%s", securityID, channelNameType)
				allChannelName := fmt.Sprintf("%d-all", securityID)
				slowChannelName := fmt.Sprintf("%d-slow-%s", securityID, channelNameType)

				data := TradeData{
					//					Ticker:     msg.Symbol,
					Price:      msg.Price,
					Size:       msg.Size,
					Timestamp:  msg.Timestamp,
					Conditions: msg.Conditions,
					ExchangeID: int(msg.Exchange),
					Channel:    fastChannelName,
				}
				//if alerts.IsAggsInitialized() {
				if useAlerts {
					if err := appendTick(conn, securityID, data.Timestamp, data.Price, data.Size); err != nil {
						// Only log non-initialization errors to reduce noise
						if !strings.Contains(err.Error(), "aggregates not yet initialized") {
							fmt.Printf("Error appending tick: %v\n", err)
						}
					}
				}
				if !hasListeners(fastChannelName) && !hasListeners(allChannelName) && !hasListeners(slowChannelName) {
					break
				}
				jsonData, err := json.Marshal(data)
				if err != nil {
					fmt.Println("Error marshling JSON:", err)
				}
				broadcastToChannel(fastChannelName, string(jsonData))
				data.Channel = allChannelName
				jsonData, err = json.Marshal(data)
				if err != nil {
					fmt.Println("Error marshling JSON:", err)
				} else {
					//conn.Cache.Publish(context.Background(), channelName, string(jsonData))
					broadcastToChannel(allChannelName, string(jsonData))
				}
				now := time.Now()
				nextDispatchTimes.RLock()
				nextDispatch, exists := nextDispatchTimes.times[msg.Symbol]
				nextDispatchTimes.RUnlock()
				// Only append tick if aggregates are initialized
				//////fmt.Println("debug: alerts.IsAggsInitialized()", alerts.IsAggsInitialized())

				//}
				if !exists || now.After(nextDispatch) {
					data.Channel = slowChannelName
					jsonData, _ = json.Marshal(data) // Handle potential error, though unlikely
					//conn.Cache.Publish(context.Background(), slowChannelName, string(jsonData))
					broadcastToChannel(slowChannelName, string(jsonData))
					nextDispatchTimes.Lock()
					nextDispatchTimes.times[msg.Symbol] = now.Add(slowRedisTimeout)
					nextDispatchTimes.Unlock()
				}
			case models.EquityQuote:
				channelName := fmt.Sprintf("%d-quote", securityID)
				if !hasListeners(channelName) {
					break
				}
				data := QuoteData{
					Timestamp: msg.Timestamp,
					BidPrice:  msg.BidPrice,
					AskPrice:  msg.AskPrice,
					BidSize:   msg.BidSize,
					AskSize:   msg.AskSize,
					Channel:   channelName,
				}
				jsonData, err := json.Marshal(data)
				if err != nil {
					//fmt.Printf("io1nv %v\n", err)
					continue
				}
				broadcastToChannel(channelName, string(jsonData))
			}

		}
	}
}

/*
	func PolygonDataToRedis(conn *data.Conn) {
		jsonData := `{"message": "Hello, WebSocket!", "value": 123}`
		err := conn.Cache.Publish(context.Background(), "websocket-test", jsonData).Err()
		if err != nil {
			//log.Println("Error publishing to Redis:", err)
		}
	}
*/

// StartPolygonWS performs operations related to StartPolygonWS functionality.
func StartPolygonWS(conn *data.Conn, _useAlerts bool) error {
	useAlerts = _useAlerts
	if err := initTickerToSecurityIDMap(conn); err != nil {
		return fmt.Errorf("failed to initialize ticker to security ID map: %v", err)
	}

	var err error
	polygonWSConn, err = polygonws.New(polygonws.Config{
		APIKey: "ogaqqkwU1pCi_x5fl97pGAyWtdhVLJYm",
		Feed:   polygonws.RealTime,
		Market: polygonws.Stocks,
	})
	if err != nil {
		return fmt.Errorf("error initializing polygonWS connection: %v", err)
	}

	if err := polygonWSConn.Connect(); err != nil {
		return fmt.Errorf("error connecting to polygonWS: %v", err)
	}

	go StreamPolygonDataToRedis(conn, polygonWSConn)
	return nil
}

// StopPolygonWS performs operations related to StopPolygonWS functionality.
func StopPolygonWS() error {
	if polygonWSConn == nil {
		fmt.Println("polygon websocket connection is not initialized")
		return fmt.Errorf("polygon websocket connection is not initialized")
	}

	if ohlcvBuffer != nil {
		ohlcvBuffer.Stop()
	}

	polygonWSConn.Close()
	return nil
}

// initTickerToSecurityIDMap initializes the map of ticker symbols to security IDs
func initTickerToSecurityIDMap(conn *data.Conn) error {
	tickerToSecurityIDLock.Lock()
	defer tickerToSecurityIDLock.Unlock()
	tickerToSecurityID = make(map[string]int)
	rows, err := conn.DB.Query(context.Background(), "SELECT ticker, securityId FROM securities where maxDate is NULL")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var ticker string
		var securityID int
		if err := rows.Scan(&ticker, &securityID); err != nil {
			return err
		}
		tickerToSecurityID[ticker] = securityID
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return nil
}
