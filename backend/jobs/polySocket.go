package jobs

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"log"

	polygonws "github.com/polygon-io/client-go/websocket"
	"github.com/polygon-io/client-go/websocket/models"
)

type TradeData struct {
    Ticker     string  `json:"ticker"`
    Price      float64 `json:"price"`
    Size       int64   `json:"size"`
    Timestamp  int64   `json:"timestamp"`
    Conditions []int32 `json:"conditions"`
    Channel    string  `json:"channel"`
}
type QuoteData struct {
    Ticker  string `json:"ticker"`
    BidPrice float64 `json:"bidPrice"`
    AskPrice float64  `json:"askPrice"`
    BidSize int32 `json:"bidSize"`
    AskSize int32 `json:"askSize"`
    Timestamp int64 `json:"timestamp"`
    Channel string `json:"channel"`
}
func StreamPolygonDataToRedis(conn *utils.Conn, polygonWS *polygonws.Client) {
	err := polygonWS.Subscribe(polygonws.StocksQuotes)
	if err != nil {
		log.Println("niv0: ", err)
		return
	} else { fmt.Printf("\n successfully connected to Polygon Quotes\n ") }
	err = polygonWS.Subscribe(polygonws.StocksTrades)
	if err != nil {
		log.Println("Error subscribing to Polygon WebSocket: ", err)
		return
	} else { fmt.Printf("\n successfully connected to Polygon Trades \n ") }
	err = polygonWS.Subscribe(polygonws.StocksMinAggs)
	if err != nil {
		log.Println("Error subscribing to Polygon WebSocket: ", err)
		return
	} else { fmt.Printf("\n successfully connected to Polygon \n") }
	for {
		select {
		case err := <-polygonWS.Error():
			fmt.Printf("PolygonWS Error: %v", err)
		case out := <-polygonWS.Output():
			switch msg := out.(type) {
			case models.EquityTrade:
				channelName := fmt.Sprintf("%s-fast", msg.Symbol)
				data := TradeData{
					Ticker:     msg.Symbol,
					Price:      msg.Price,
					Size:       msg.Size,
					Timestamp:  msg.Timestamp,
					Conditions: msg.Conditions,
					Channel:    channelName,
				}
				jsonData, err := json.Marshal(data)
				if err != nil {
					fmt.Println("Error marshling JSON:", err)
				}
				conn.Cache.Publish(context.Background(), "trades-agg", string(jsonData))
				conn.Cache.Publish(context.Background(), channelName, string(jsonData))
			case models.EquityQuote:
				channelName := fmt.Sprintf("%s-quote", msg.Symbol)
                data := QuoteData{
                    Ticker:msg.Symbol, 
                    Timestamp:msg.Timestamp,
                    BidPrice: msg.BidPrice,
                    AskPrice: msg.AskPrice,
                    BidSize: msg.BidSize,
                    AskSize:msg.AskSize,
                    Channel: channelName,
                }
		//		data := fmt.Sprintf(`{"ticker": "%s", "bidprice": %v, "bidsize": %v, "bidex": %v, "askprice": %v, "asksize": %v, "askex": %v, "timestamp":%v}`,
		//			msg.Symbol, msg.BidPrice, msg.BidSize, msg.BidExchangeID, msg.AskPrice, msg.AskSize, msg.AskExchangeID, msg.Timestamp)
       //         fmt.Println(data)
                jsonData, err := json.Marshal(data)
                if err != nil {
                    fmt.Printf("io1nv %v\n",err)
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
	fmt.Println("Done.")
	if err != nil {
		log.Println("Error publishing to Redis:", err)
	}
}
func startPolygonWS(conn *utils.Conn) error {
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
