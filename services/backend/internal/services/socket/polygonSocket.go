package socket

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"log"

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

// Latest price cache for alerts
var (
	latestPrices      = make(map[int]float64) // securityID -> latest price
	latestPricesMutex sync.RWMutex
)

// Condition code filtering constants
var (
	// Price-only skips - keep the shares (condition codes whose price should be ignored but volume may be kept)
	tradeConditionsToSkipOhlc = map[int32]struct{}{
		2: {}, 7: {}, 12: {}, 13: {}, 20: {}, 21: {}, 37: {}, 52: {}, 53: {},
	}

	// Volume-only skips (condition codes whose volume must be ignored)
	tradeConditionsToSkipVolume = map[int32]struct{}{
		15: {}, 16: {}, 38: {},
	}

	// Hard rejects - ignore price AND volume (trades that are useless for both price and volume)
	tradeConditionsToExcludeCompletely = map[int32]struct{}{
		15: {}, 16: {}, // official open/close stub prints
		//13: {}, // optional: very late ext-hours prints
	}
)

// Helper function to check if trade should be excluded completely
func shouldExcludeTrade(conditions []int32) bool {
	for _, condition := range conditions {
		if _, found := tradeConditionsToExcludeCompletely[condition]; found {
			return true
		}
	}
	return false
}

// Helper function to check if trade should skip OHLC updates
func shouldSkipOhlc(conditions []int32) bool {
	for _, condition := range conditions {
		if _, found := tradeConditionsToSkipOhlc[condition]; found {
			return true
		}
	}
	return false
}

// Helper function to check if trade should skip volume updates
func shouldSkipVolume(conditions []int32) bool {
	for _, condition := range conditions {
		if _, found := tradeConditionsToSkipVolume[condition]; found {
			return true
		}
	}
	return false
}

// GetLatestPrice returns the latest price for a given security ID
func GetLatestPrice(securityID int) (float64, bool) {
	latestPricesMutex.RLock()
	defer latestPricesMutex.RUnlock()
	price, exists := latestPrices[securityID]
	return price, exists
}

// updateLatestPrice updates the latest price for a security ID
func updateLatestPrice(securityID int, price float64) {
	latestPricesMutex.Lock()
	defer latestPricesMutex.Unlock()
	latestPrices[securityID] = price
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
					ohlcvBuffer.addBar(msg.EndTimestamp, symbol, msg)
				}

				/* alerts.appendAggregate(securityId,msg.Open,msg.High,msg.Low,msg.Close,msg.Volume)*/
			case models.EquityTrade:
				// First check if trade should be completely excluded (ignore both price and volume)
				if shouldExcludeTrade(msg.Conditions) {
					continue
				}

				// Check if we should skip price updates but keep volume
				skipPriceUpdate := shouldSkipOhlc(msg.Conditions)
				skipVolumeUpdate := shouldSkipVolume(msg.Conditions)

				channelNameType := getChannelNameType(msg.Timestamp)
				fastChannelName := fmt.Sprintf("%d-fast-%s", securityID, channelNameType)
				allChannelName := fmt.Sprintf("%d-all", securityID)
				slowChannelName := fmt.Sprintf("%d-slow-%s", securityID, channelNameType)

				// Create trade data with conditional price and size
				// If skipping volume updates, set size to 0
				tradeSize := msg.Size
				if skipVolumeUpdate {
					tradeSize = 0
				}

				// If skipping price updates, set price to -1 to signal frontend to ignore price
				price := msg.Price
				if skipPriceUpdate {
					price = -1
				}

				data := TradeData{
					//					Ticker:     msg.Symbol,
					Price:      price,
					Size:       tradeSize,
					Timestamp:  msg.Timestamp,
					Conditions: msg.Conditions,
					ExchangeID: int(msg.Exchange),
					Channel:    fastChannelName,
				}

				// Only update latest price cache if we're not skipping price updates
				if !skipPriceUpdate {
					updateLatestPrice(securityID, msg.Price)
				}

				// COMMENTED OUT: appendTick call disabled - alerts will be processed directly from ticks
				/*
					//if alerts.IsAggsInitialized() {
					if useAlerts {
						if err := appendTick(conn, securityID, data.Timestamp, data.Price, data.Size); err != nil {
							// Only log non-initialization errors to reduce noise
							if !strings.Contains(err.Error(), "aggregates not yet initialized") {
								fmt.Printf("Error appending tick: %v\n", err)
							}
						}
					}
				*/
				// Process alerts directly from tick data
				/*if useAlerts {
					// Update tick prices and process alerts
					alerts.ProcessTickUpdate(conn, securityID, data.Price)
				}*/
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
				// Only send to slow stream if price is not -1 (not volume-only trade)
				if data.Price >= 0 && (!exists || now.After(nextDispatch)) {
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
func StartPolygonWS(conn *data.Conn, _useAlerts bool, enableRealtime bool) error {
	useAlerts = _useAlerts
	if err := initTickerToSecurityIDMap(conn); err != nil {
		return fmt.Errorf("failed to initialize ticker to security ID map: %v", err)
	}

	// Initialize OHLCV buffer with realtime flag
	InitOHLCVBuffer(conn, enableRealtime)

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
