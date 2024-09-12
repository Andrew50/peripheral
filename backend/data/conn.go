package data

import (
	"context"
	"fmt"
	"log"
	"time"

	polygon "github.com/polygon-io/client-go/rest"
	polygonws "github.com/polygon-io/client-go/websocket"

	"github.com/jackc/pgx/v4/pgxpool"
)

type Conn struct {
	//Cache *redis.Client
	DB        *pgxpool.Pool
	Polygon   *polygon.Client
	PolygonWS *polygonws.Client
}

func InitConn(inContainer bool) (*Conn, func()) {
	//TODO change this sahit to use env vars as well
	var dbUrl string
	if inContainer {
		dbUrl = "postgres://postgres:pass@db:5432"
	} else {
		dbUrl = "postgres://postgres:pass@localhost:5432"
	}
	var dbConn *pgxpool.Pool
	var err error
	for true {
		dbConn, err = pgxpool.Connect(context.Background(), dbUrl)
		if err != nil {
			//if strings.Contains(err.Error(), "the database system is starting up") {
			log.Println("waiting for db")
			time.Sleep(5 * time.Second)
		} else {
			break
		}
	}
	/*
			   cache_url :="redis:6379"
			   cache := redis.NewClient(&redis.Options{Addr: cache_url,})
			   err = cache.Ping(context.Background()).Err()
			   if err != nil {
		           //god
			   }
	*/
	//return &Conn{DB: db, Cache: cache}
	//polygonConn := polygon.New(os.Getenv("POLYGON_API_KEY"))
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
	conn := &Conn{DB: dbConn, Polygon: polygonConn, PolygonWS: polygonWSConn}

	cleanup := func() {
		conn.DB.Close()
	}
	return conn, cleanup
}
