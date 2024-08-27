package data

import (
	polygon "github.com/polygon-io/client-go/rest"
	//"github.com/polygon-io/client-go/rest/models"
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

type Conn struct {
	//Cache *redis.Client
	DB      *pgxpool.Pool
	Polygon *polygon.Client
}

func InitConn() (*Conn, func()) {
	//TODO change this shit to use env vars as well
	db_url := "postgres://postgres:pass@db:5432"
	var dbConn *pgxpool.Pool
	var err error
	for true {
		dbConn, err = pgxpool.Connect(context.Background(), db_url)
		if err != nil {
			//if strings.Contains(err.Error(), "the database system is starting up") {
			if true {
				log.Println("waiting for db")
			} else {
				log.Fatalf("Unable to connect to database: %v\n", err)
			}
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
	       log.Fatalf("Unable to connect to cache: %v\n", err)
	   }
	*/
	//return &Conn{DB: db, Cache: cache}
	//polygonConn := polygon.New(os.Getenv("POLYGON_API_KEY"))
	polygonConn := polygon.New("ogaqqkwU1pCi_x5fl97pGAyWtdhVLJYm")
	conn := &Conn{DB: dbConn, Polygon: polygonConn}

	cleanup := func() {
		conn.DB.Close()
	}
	return conn, cleanup
}
func GetConn(container bool) *Conn {
	var db_url string
	if container {
		db_url = "postgres://postgres:pass@db:5432"
	} else {
		db_url = "postgres://postgres:pass@localhost:5432"
	}
	var db *pgxpool.Pool
	var err error
	for true {
		db, err = pgxpool.Connect(context.Background(), db_url)
		if err != nil {
			//if strings.Contains(err.Error(), "the database system is starting up") {
			if true {
				log.Println("waiting for db")
			} else {
				log.Fatalf("Unable to connect to database: %v\n", err)
			}
			time.Sleep(5 * time.Second)
		} else {
			break
		}
	}
	polygonConn := polygon.New("ogaqqkwU1pCi_x5fl97pGAyWtdhVLJYm")
	return &Conn{DB: db, Polygon: polygonConn}
}
