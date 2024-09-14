package utils

import (
	"context"
	"fmt"
	"log"

	polygonws "github.com/polygon-io/client-go/websocket"
	"github.com/polygon-io/client-go/websocket/models"
)

func StreamPolygonDataToRedis(conn *Conn) {
	err := conn.PolygonWS.Subscribe(polygonws.StocksTrades)
	if err != nil {
		log.Println("Error subscribing to Polygon WebSocket: ", err)
		return
	} else {
		fmt.Printf("successfully connected to Polygon")
	}
	err = conn.PolygonWS.Subscribe(polygonws.StocksMinAggs)
	if err != nil {
		log.Println("Error subscribing to Polygon WebSocket: ", err)
		return
	} else {
		fmt.Printf("successfully connected to Polygon")
	}
	for {
		select {
		case err := <-conn.PolygonWS.Error():
			fmt.Printf("PolygonWS Error: %v", err)
		case out := <-conn.PolygonWS.Output():
			switch msg := out.(type) {
			case models.EquityTrade:
				data := fmt.Sprintf(`{"ticker": "%s", "price": %v, "size": %v, "timestamp": %v}`, msg.Symbol, msg.Price, msg.Size, msg.Timestamp)
				conn.Cache.Publish(context.Background(), "trades-agg", data)
				channelName := fmt.Sprintf("trades-fast-%s", msg.Symbol)
				conn.Cache.Publish(context.Background(), channelName, data)
			case models.EquityQuote:
				data := fmt.Sprintf(`{"ticker": "%s", "bidprice": %v, "bidsize": %v, "bidex": %v, "askprice": %v, "asksize": %v, "askex": %v, "timestamp":%v}`,
					msg.Symbol, msg.BidPrice, msg.BidSize, msg.BidExchangeID, msg.AskPrice, msg.AskSize, msg.AskExchangeID, msg.Timestamp)
				channelName := fmt.Sprintf("quotes-%s", msg.Symbol)
				conn.Cache.Publish(context.Background(), channelName, data)
			}

		}
	}
}
func PolygonDataToRedis(conn *Conn) {
	jsonData := `{"message": "Hello, WebSocket!", "value": 123}`
	err := conn.Cache.Publish(context.Background(), "websocket-test", jsonData).Err()
	fmt.Println("Done.")
	if err != nil {
		log.Println("Error publishing to Redis:", err)
	}
}
