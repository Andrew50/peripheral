// polygonSocket.go
package socket

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"log"
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

var tickerToSecurityId map[string]int
var tickerToSecurityIdLock sync.RWMutex

// Add this package-level variable

var polygonWSConn *polygonws.Client

const TimestampUpdateInterval = 2 * time.Second

var (
	lastTickTimestamp  int64
	tickTimestampMutex sync.RWMutex
)

func StreamPolygonDataToRedis(conn *utils.Conn, polygonWS *polygonws.Client) {
	err := polygonWS.Subscribe(polygonws.StocksQuotes)
	if err != nil {
		log.Println("niv0: ", err)
		return
	} else {
		fmt.Println("✅ Connected to Polygon Quotes stream")
	}
	err = polygonWS.Subscribe(polygonws.StocksTrades)
	if err != nil {
		log.Println("Error subscribing to Polygon WebSocket: ", err)
		return
	} else {
		fmt.Println("✅ Connected to Polygon Trades stream")
	}
	err = polygonWS.Subscribe(polygonws.StocksMinAggs)
	if err != nil {
		log.Println("Error subscribing to Polygon WebSocket: ", err)
		return
	} else {
		fmt.Println("✅ Connected to Polygon Minute Aggregates stream")
	}

	fmt.Println("🚀 All Polygon streams initialized and ready to process data")

	// Add timestamp ticker
	timestampTicker := time.NewTicker(TimestampUpdateInterval)
	defer timestampTicker.Stop()

	for {
		select {
		case <-timestampTicker.C:
			broadcastTimestamp()
		case err := <-polygonWS.Error():
			fmt.Printf("PolygonWS Error: %v", err)
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
				//jlog.Println("Unknown message type received")
				continue
			}

			// Update the last tick timestamp
			tickTimestampMutex.Lock()
			if timestamp > lastTickTimestamp {
				lastTickTimestamp = timestamp
			}
			tickTimestampMutex.Unlock()

			tickerToSecurityIdLock.RLock()
			securityId, exists := tickerToSecurityId[symbol]
			tickerToSecurityIdLock.RUnlock()
			if !exists {
				//log.Printf("Symbol %s not found in tickerToSecurityId map\n", symbol)
				continue
			}
			switch msg := out.(type) {
			/*            case models.EquityAgg:
			              alerts.appendAggregate(securityId,msg.Open,msg.High,msg.Low,msg.Close,msg.Volume)*/
			case models.EquityTrade:
				channelNameType := getChannelNameType(msg.Timestamp)
				channelName := fmt.Sprintf("%d-fast-%s", securityId, channelNameType)
				data := TradeData{
					//					Ticker:     msg.Symbol,
					Price:      msg.Price,
					Size:       msg.Size,
					Timestamp:  msg.Timestamp,
					Conditions: msg.Conditions,
					ExchangeId: msg.Exchange,
					Channel:    channelName,
				}
				jsonData, err := json.Marshal(data)
				if err != nil {
					fmt.Println("Error marshling JSON:", err)
				}
				//	conn.Cache.Publish(context.Background(), "trades-agg", string(jsonData))

				//conn.Cache.Publish(context.Background(), channelName, string(jsonData))
				broadcastToChannel(channelName, string(jsonData))
				channelName = fmt.Sprintf("%d-all", securityId)
				data.Channel = channelName
				jsonData, err = json.Marshal(data)
				if err != nil {
					fmt.Println("Error marshling JSON:", err)
				}
				//conn.Cache.Publish(context.Background(), channelName, string(jsonData))
				broadcastToChannel(channelName, string(jsonData))
				now := time.Now()
				nextDispatchTimes.RLock()
				nextDispatch, exists := nextDispatchTimes.times[msg.Symbol]
				nextDispatchTimes.RUnlock()
				// Only append tick if aggregates are initialized
				//fmt.Println("debug: alerts.IsAggsInitialized()", alerts.IsAggsInitialized())

				//if alerts.IsAggsInitialized() {
				if useAlerts {
					appendTick(conn, securityId, data.Timestamp, data.Price, data.Size)
				}
				//}
				if !exists || now.After(nextDispatch) {
					slowChannelName := fmt.Sprintf("%d-slow-%s", securityId, channelNameType)
					data.Channel = slowChannelName
					jsonData, err = json.Marshal(data)
					if err != nil {
						fmt.Println("E2fi200e2e0rror marshling JSON:", err)
					}
					//conn.Cache.Publish(context.Background(), slowChannelName, string(jsonData))
					broadcastToChannel(slowChannelName, string(jsonData))
					nextDispatchTimes.Lock()
					nextDispatchTimes.times[msg.Symbol] = now.Add(slowRedisTimeout)
					nextDispatchTimes.Unlock()
				}
			case models.EquityQuote:
				channelNameType := getChannelNameType(msg.Timestamp)

				channelName := fmt.Sprintf("%d-%s-quote", securityId, channelNameType)
				data := QuoteData{

					//					Ticker:    msg.Symbol,
					Timestamp: msg.Timestamp,
					BidPrice:  msg.BidPrice,
					AskPrice:  msg.AskPrice,
					BidSize:   msg.BidSize,
					AskSize:   msg.AskSize,
					Channel:   channelName,
				}
				jsonData, err := json.Marshal(data)
				if err != nil {
					fmt.Printf("io1nv %v\n", err)
					continue
				}
				//conn.Cache.Publish(context.Background(), channelName, jsonData)
				broadcastToChannel(channelName, string(jsonData))
			}

		}
	}
}

/*
	func PolygonDataToRedis(conn *utils.Conn) {
		jsonData := `{"message": "Hello, WebSocket!", "value": 123}`
		err := conn.Cache.Publish(context.Background(), "websocket-test", jsonData).Err()
		if err != nil {
			log.Println("Error publishing to Redis:", err)
		}
	}
*/
func StartPolygonWS(conn *utils.Conn, _useAlerts bool) error {
	useAlerts = _useAlerts
	if err := initTickerToSecurityIdMap(conn); err != nil {
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

func StopPolygonWS() error {
	if polygonWSConn == nil {
		return fmt.Errorf("polygon websocket connection is not initialized")
	}

	polygonWSConn.Close()
	return nil
}

func initTickerToSecurityIdMap(conn *utils.Conn) error {
	tickerToSecurityIdLock.Lock()
	defer tickerToSecurityIdLock.Unlock()
	tickerToSecurityId = make(map[string]int)
	rows, err := conn.DB.Query(context.Background(), "SELECT ticker, securityId FROM securities where maxDate is NULL")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var ticker string
		var securityId int
		if err := rows.Scan(&ticker, &securityId); err != nil {
			return err
		}
		tickerToSecurityId[ticker] = securityId
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return nil
}
