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
			case models.EquityAgg:
				data := fmt.Sprintf(`{"ticker": "%s", "open": %f, "close": %f}`, msg.Symbol, msg.Open, msg.Close)
				err = conn.Cache.Publish(context.Background(), fmt.Sprintf("aggs-%s", msg.Symbol), data).Err()
				if err != nil {
					log.Println("Error publishing to Redis:", err)
				}
			case models.EquityTrade:
				data := fmt.Sprintf(`{"ticker": "%s", "price": %v, "size": %v, "timestamp": %v}`, msg.Symbol, msg.Price, msg.Size, msg.Timestamp)
				channelName := fmt.Sprintf("trades-fast-%s", msg.Symbol)
				err = conn.Cache.Publish(context.Background(), channelName, data).Err()
				if err != nil {
					fmt.Println("error publishing to redis:", err)
				}
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
