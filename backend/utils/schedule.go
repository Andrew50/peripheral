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
	for {
		select {
		case err := <-conn.PolygonWS.Error():
			fmt.Printf("PolygonWS Error: %v", err)
		case out := <-conn.PolygonWS.Output():
			switch msg := out.(type) {
			case models.EquityAgg:
				data := fmt.Sprintf(`{"ticker": "%s", "open": %f, "close": %f}`, msg.Symbol, msg.Open, msg.Close)
				err = conn.Cache.Publish(context.Background(), "polygon-aggregates", data).Err()
				if err != nil {
					log.Println("Error publishing to Redis:", err)
				} else {
					log.Printf("Published to Redis: %s\n", data)
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
