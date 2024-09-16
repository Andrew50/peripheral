package jobs

import (
	"backend/utils"
	"context"
	"fmt"
	"log"

	polygonws "github.com/polygon-io/client-go/websocket"
	"github.com/polygon-io/client-go/websocket/models"
)

func StreamPolygonDataToRedis(conn *utils.Conn, polygonWS *polygonws.Client) {
	err := polygonWS.Subscribe(polygonws.StocksTrades)
	if err != nil {
		log.Println("Error subscribing to Polygon WebSocket: ", err)
		return
	} else {
		fmt.Printf("successfully connected to Polygon")
	}
	err = polygonWS.Subscribe(polygonws.StocksMinAggs)
	if err != nil {
		log.Println("Error subscribing to Polygon WebSocket: ", err)
		return
	} else {
		fmt.Printf("successfully connected to Polygon")
	}
	for {
		select {
		case err := <-polygonWS.Error():
			fmt.Printf("PolygonWS Error: %v", err)
		case out := <-polygonWS.Output():
			switch msg := out.(type) {
			case models.EquityTrade:
				channelName := fmt.Sprintf("%s-fast", msg.Symbol)
				data := fmt.Sprintf(`{"ticker": "%s", "price": %v, "size": %v, "timestamp": %v, "channel": "%v"}`, msg.Symbol, msg.Price, msg.Size, msg.Timestamp, channelName)
				conn.Cache.Publish(context.Background(), "trades-agg", data)
				conn.Cache.Publish(context.Background(), channelName, data)
			case models.EquityQuote:
				data := fmt.Sprintf(`{"ticker": "%s", "bidprice": %v, "bidsize": %v, "bidex": %v, "askprice": %v, "asksize": %v, "askex": %v, "timestamp":%v}`,
					msg.Symbol, msg.BidPrice, msg.BidSize, msg.BidExchangeID, msg.AskPrice, msg.AskSize, msg.AskExchangeID, msg.Timestamp)
				channelName := fmt.Sprintf("%s-quote", msg.Symbol)
				conn.Cache.Publish(context.Background(), channelName, data)
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
