package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v4/pgxpool"
	polygon "github.com/polygon-io/client-go/rest"
	polygonws "github.com/polygon-io/client-go/websocket"

	"github.com/gorilla/websocket"
)

type Conn struct {
	//Cache *redis.Client
	DB        *pgxpool.Pool
	Polygon   *polygon.Client
	Cache     *redis.Client
	PolygonWS *polygonws.Client
}

func InitConn(inContainer bool) (*Conn, func()) {
	//TODO change this sahit to use env vars as well
	var dbUrl string
	var cacheUrl string
	if inContainer {
		dbUrl = "postgres://postgres:pass@db:5432"
		cacheUrl = "redis:6379"
	} else {
		dbUrl = "postgres://postgres:pass@localhost:5432"
		cacheUrl = "localhost:6379"
	}
	var dbConn *pgxpool.Pool
	var err error
	for true {
		dbConn, err = pgxpool.Connect(context.Background(), dbUrl)
		if err != nil {
			//if strings.Contains(err.Error(), "the database system is starting up") {
			log.Println("waiting for db %v", err)
			time.Sleep(5 * time.Second)
		} else {
			break
		}
	}
	var cache *redis.Client
	for {
		cache = redis.NewClient(&redis.Options{Addr: cacheUrl})
		err = cache.Ping(context.Background()).Err()
		if err != nil {
			//if strings.Contains(err.Error(), "the database system is starting up") {
			log.Println("waiting for cache")
			time.Sleep(5 * time.Second)
		} else {
			break
		}
	}
	polygonConn := polygon.New("ogaqqkwU1pCi_x5fl97pGAyWtdhVLJYm")
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
	conn := &Conn{DB: dbConn, Cache: cache, Polygon: polygonConn, PolygonWS: polygonWSConn}

	go StreamPolygonDataToRedis(conn)
	cleanup := func() {
		conn.DB.Close()
		conn.Cache.Close()
		conn.PolygonWS.Close()
	}
	return conn, cleanup
}

type Client struct {
	ws       *websocket.Conn
	mu       sync.Mutex
	channels map[string]*redis.PubSub
}

func WsFrontendHandler(conn *Conn) http.HandlerFunc {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			// Adjust this according to your CORS policy
			return true
		},
	}
	return func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			fmt.Println("failed to upgrade to websocket: ", err)
			return
		}
		defer ws.Close()
		client := &Client{
			ws:       ws,
			channels: make(map[string]*redis.PubSub),
		}
		redisMessages := make(chan *redis.Message)

		go func() {
			for {
				_, message, err := ws.ReadMessage()
				if err != nil {
					log.Println("WebSocket read error:", err)
					break
				}
				var clientMsg struct {
					Action      string `json:"action"`
					ChannelName string `json:"channelName"`
				}
				if err := json.Unmarshal(message, &clientMsg); err != nil {
					fmt.Println("Invalid message format", err)
					continue
				}
				switch clientMsg.Action {
				case "subscribe":
					client.subscribe(conn.Cache, clientMsg.ChannelName, redisMessages)
				case "unsubscribe":
					client.unsubscribe(clientMsg.ChannelName)
				default:
					fmt.Println("4lgkdvv, Unknown action received from WebSocket client:", clientMsg.Action)

				}
			}
			client.close()
		}()
		for msg := range redisMessages {
			err := ws.WriteMessage(websocket.TextMessage, []byte(msg.Payload))
			if err != nil {
				fmt.Println("l4ifjv, WebSocket write error:", err)
				break
			}
		}
	}
}
func (c *Client) subscribe(redisClient *redis.Client, channelName string, redisMessages chan<- *redis.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.channels[channelName]; exists {
		return
	}
	pubsub := redisClient.Subscribe(context.Background(), channelName)
	c.channels[channelName] = pubsub

	go func() {
		for msg := range pubsub.Channel() {
			redisMessages <- msg
		}
	}()

}
func (c *Client) unsubscribe(channelName string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if pubsub, exists := c.channels[channelName]; exists {
		pubsub.Close()
		delete(c.channels, channelName)
	}
}
func (c *Client) close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, pubsub := range c.channels {
		pubsub.Close()
	}
	c.channels = nil
	c.ws.Close()
}
