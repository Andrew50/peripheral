package utils

import (
	"context"
	"fmt"
	"log"
	"time"
    "github.com/go-redis/redis/v8"
	polygon "github.com/polygon-io/client-go/rest"
	polygonws "github.com/polygon-io/client-go/websocket"
	"github.com/jackc/pgx/v4/pgxpool"
)

type Conn struct {
	//Cache *redis.Client
	DB      *pgxpool.Pool
	Polygon *polygon.Client
    Cache   *redis.Client
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
			log.Println("waiting for db %v",err)
			time.Sleep(5 * time.Second)
		} else {
			break
		}
	}
    var cache *redis.Client
	for true {
       cache = redis.NewClient(&redis.Options{Addr: cacheUrl,})
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
	defer polygonWSConn.Close()
	if err := polygonWSConn.Connect(); err != nil {
		fmt.Printf("Error connecting to polygonWS")
	}
    conn := &Conn{DB: dbConn, Cache:cache,Polygon: polygonConn, PolygonWS: polygonWSConn}

	cleanup := func() {
		conn.DB.Close()
        conn.Cache.Close()
	}
	return conn, cleanup
}
