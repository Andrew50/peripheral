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

const slowRedisTimeout = 1 * time.Second // Adjust the timeout as needed

var tickerToSecurityId map[string]int

func StreamPolygonDataToRedis(conn *utils.Conn, polygonWS *polygonws.Client) {
	err := polygonWS.Subscribe(polygonws.StocksQuotes)
	if err != nil {
		log.Println("niv0: ", err)
		return
	} else {
		fmt.Printf("\n successfully connected to Polygon Quotes\n ")
	}
	err = polygonWS.Subscribe(polygonws.StocksTrades)
	if err != nil {
		log.Println("Error subscribing to Polygon WebSocket: ", err)
		return
	} else {
		fmt.Printf("\n successfully connected to Polygon Trades \n ")
	}
	err = polygonWS.Subscribe(polygonws.StocksMinAggs)
	if err != nil {
		log.Println("Error subscribing to Polygon WebSocket: ", err)
		return
	} else {
		fmt.Printf("\n successfully connected to Polygon \n")
	}
	for {
		select {
		case err := <-polygonWS.Error():
			fmt.Printf("PolygonWS Error: %v", err)
		case out := <-polygonWS.Output():
			var symbol string
			switch msg := out.(type) {
			case models.EquityAgg:
				symbol = msg.Symbol
				if symbol == "NVDA" {
					fmt.Println(msg.EndTimestamp)
					fmt.Println(msg.StartTimestamp)
				}
			case models.EquityTrade:
				symbol = msg.Symbol
			case models.EquityQuote:
				symbol = msg.Symbol
			default:
				//jlog.Println("Unknown message type received")
				continue
			}
			securityId, exists := tickerToSecurityId[symbol]
			if !exists {
				//log.Printf("Symbol %s not found in tickerToSecurityId map\n", symbol)
				continue
			}
			switch msg := out.(type) {
			case models.EquityTrade:
				channelName := fmt.Sprintf("%d-fast", securityId)
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

				conn.Cache.Publish(context.Background(), channelName, string(jsonData))
				channelName = fmt.Sprintf("%d-all", securityId)
				data.Channel = channelName
				jsonData, err = json.Marshal(data)
				if err != nil {
					fmt.Println("Error marshling JSON:", err)
				}
				conn.Cache.Publish(context.Background(), channelName, string(jsonData))
				now := time.Now()
				nextDispatchTimes.RLock()
				nextDispatch, exists := nextDispatchTimes.times[msg.Symbol]
				nextDispatchTimes.RUnlock()
				if !exists || now.After(nextDispatch) {
					slowChannelName := fmt.Sprintf("%d-slow", securityId)
					data.Channel = slowChannelName
					jsonData, err = json.Marshal(data)
					conn.Cache.Publish(context.Background(), slowChannelName, string(jsonData))
					nextDispatchTimes.Lock()
					nextDispatchTimes.times[msg.Symbol] = now.Add(slowRedisTimeout)
					nextDispatchTimes.Unlock()
				}
			case models.EquityQuote:
				channelName := fmt.Sprintf("%d-quote", securityId)
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
				conn.Cache.Publish(context.Background(), channelName, jsonData)
			}

		}
	}
}
func PolygonDataToRedis(conn *utils.Conn) {
	jsonData := `{"message": "Hello, WebSocket!", "value": 123}`
	err := conn.Cache.Publish(context.Background(), "websocket-test", jsonData).Err()
	if err != nil {
		log.Println("Error publishing to Redis:", err)
	}
}
func StartPolygonWS(conn *utils.Conn) error {
	if err := initTickerToSecurityIdMap(conn); err != nil {
		return fmt.Errorf("failed to initialize ticker to security ID map: %v", err)
	}
	polygonWSConn, err := polygonws.New(polygonws.Config{
		APIKey: "ogaqqkwU1pCi_x5fl97pGAyWtdhVLJYm",
		Feed:   polygonws.RealTime,
		Market: polygonws.Stocks,
	})
	if err != nil {
		fmt.Printf("Error init polygonWs connection")
	}
	if err := polygonWSConn.Connect(); err != nil {
		fmt.Printf("Error connecting to polygonWS")
	}
	go StreamPolygonDataToRedis(conn, polygonWSConn)
	return nil
}

func initTickerToSecurityIdMap(conn *utils.Conn) error {
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
