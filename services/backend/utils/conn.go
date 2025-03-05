package utils

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v4/pgxpool"
	polygon "github.com/polygon-io/client-go/rest"
)

// Conn represents a structure for handling Conn data.
type Conn struct {
	//Cache *redis.Client
	DB         *pgxpool.Pool
	Polygon    *polygon.Client
	Cache      *redis.Client
	PolygonKey string
}

var conn *Conn

// InitConn performs operations related to InitConn functionality.
func InitConn(inContainer bool) (*Conn, func()) {
	// Get database connection details from environment variables
	dbHost := getEnv("DB_HOST", "db")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "postgres")
	dbPassword := getEnv("DB_PASSWORD", "")

	// Get Redis connection details from environment variables
	redisHost := getEnv("REDIS_HOST", "cache")
	redisPort := getEnv("REDIS_PORT", "6379")
	redisPassword := getEnv("REDIS_PASSWORD", "")

	var dbUrl string
	var cacheUrl string

	// URL encode the password to handle special characters
	encodedPassword := url.QueryEscape(dbPassword)

	if inContainer {
		dbUrl = fmt.Sprintf("postgres://%s:%s@%s:%s", dbUser, encodedPassword, dbHost, dbPort)
		cacheUrl = fmt.Sprintf("%s:%s", redisHost, redisPort)
	} else {
		dbUrl = fmt.Sprintf("postgres://%s:%s@localhost:%s", dbUser, encodedPassword, dbPort)
		cacheUrl = fmt.Sprintf("localhost:%s", redisPort)
	}

	var dbConn *pgxpool.Pool
	var err error
	for {
		dbConn, err = pgxpool.Connect(context.Background(), dbUrl)
		if err != nil {
			//if strings.Contains(err.Error(), "the database system is starting up") {
			log.Printf("waiting for db %v\n", err)
			time.Sleep(5 * time.Second)
		} else {
			break
		}
	}

	var cache *redis.Client
	for {
		// Use Redis password if provided
		opts := &redis.Options{
			Addr: cacheUrl,
			// Add connection pool settings
			PoolSize:     20,               // Increased from 10
			MinIdleConns: 10,               // Increased from 5
			PoolTimeout:  60 * time.Second, // Increased from 30
			// Add timeouts
			ReadTimeout:  30 * time.Second, // Increased from 10
			WriteTimeout: 30 * time.Second, // Increased from 10
			// Add retry settings
			MaxRetries:      5,
			MinRetryBackoff: 1 * time.Second,
			MaxRetryBackoff: 10 * time.Second,
			// Add dial timeout
			DialTimeout: 15 * time.Second,
		}
		if redisPassword != "" {
			opts.Password = redisPassword
		}

		cache = redis.NewClient(opts)
		err = cache.Ping(context.Background()).Err()
		if err != nil {
			//if strings.Contains(err.Error(), "the database system is starting up") {
			log.Println("waiting for cache")
			time.Sleep(5 * time.Second)
		} else {
			break
		}
	}

	polygonKey := getEnv("POLYGON_API_KEY", "ogaqqkwU1pCi_x5fl97pGAyWtdhVLJYm")

	// Configure the HTTP client with better timeout settings
	httpClient := &http.Client{
		Timeout: 120 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:          200,
			MaxIdleConnsPerHost:   50,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   15 * time.Second,
			DisableKeepAlives:     false,
			ResponseHeaderTimeout: 60 * time.Second,
			ExpectContinueTimeout: 10 * time.Second,
			MaxConnsPerHost:       100,
		},
	}

	// Create Polygon client with custom HTTP client
	polygonClient := polygon.NewWithClient(polygonKey, httpClient)

	conn = &Conn{DB: dbConn, Cache: cache, Polygon: polygonClient, PolygonKey: polygonKey}

	cleanup := func() {
		conn.DB.Close()
		conn.Cache.Close()
	}
	return conn, cleanup
}

// Helper function to get environment variables with fallback
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
