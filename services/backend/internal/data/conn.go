package data

import (
	"context"
	"fmt"
	"log"

	//	"log"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v4/pgxpool"
	polygon "github.com/polygon-io/client-go/rest"
)

// Conn encapsulates database connections and API clients
type Conn struct {
	//Cache *redis.Client
	DB              *pgxpool.Pool
	Polygon         *polygon.Client
	Cache           *redis.Client
	PolygonKey      string
	GeminiPool      *GeminiKeyPool
	PerplexityKey   string
	XAPIKey         string
	TwitterAPIioKey string
	OpenAIKey       string
}

// Result structs for thread-safe communication
type dbConnResult struct {
	conn *pgxpool.Pool
	err  error
}

type redisConnResult struct {
	client *redis.Client
	err    error
}

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

	// Get API keys from environment variables
	polygonKey := getEnv("POLYGON_API_KEY", "ogaqqkwU1pCi_x5fl97pGAyWtdhVLJYm")
	perplexityKey := getEnv("PERPLEXITY_API_KEY", "")
	XAPIKey := getEnv("X_API_KEY", "")
	twitterAPIioKey := getEnv("TWITTER_API_IO_KEY", "")
	openAIKey := getEnv("OPENAI_API_KEY", "")
	var dbURL string
	var cacheURL string

	// URL encode the password to handle special characters
	encodedPassword := url.QueryEscape(dbPassword)

	if inContainer {
		dbURL = fmt.Sprintf("postgres://%s:%s@%s:%s", dbUser, encodedPassword, dbHost, dbPort)
		cacheURL = fmt.Sprintf("%s:%s", redisHost, redisPort)
	} else {
		dbURL = fmt.Sprintf("postgres://%s:%s@localhost:%s", dbUser, encodedPassword, dbPort)
		cacheURL = fmt.Sprintf("localhost:%s", redisPort)
	}

	// Add timeout for database connection attempts using context
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Use channels for thread-safe communication
	dbResult := make(chan dbConnResult, 1)
	go func() {
		defer close(dbResult)
		var lastErr error
		for {
			select {
			case <-ctx.Done():
				dbResult <- dbConnResult{conn: nil, err: lastErr}
				return
			default:
				// Create a connection pool configuration
				poolConfig, parseErr := pgxpool.ParseConfig(dbURL)
				if parseErr != nil {
					lastErr = parseErr
					time.Sleep(1 * time.Second)
					continue
				}

				// Configure connection pool with better defaults
				poolConfig.MaxConns = 25                               // Increase max connections
				poolConfig.MinConns = 5                                // Increase minimum connections
				poolConfig.MaxConnLifetime = 1 * time.Hour             // Maximum lifetime of a connection
				poolConfig.MaxConnIdleTime = 30 * time.Minute          // Maximum idle time for a connection
				poolConfig.HealthCheckPeriod = 30 * time.Second        // More frequent health checks
				poolConfig.ConnConfig.ConnectTimeout = 5 * time.Second // Shorter connection timeout

				// Create the connection pool with our custom configuration
				dbConn, err := pgxpool.ConnectConfig(ctx, poolConfig)
				if err != nil {
					lastErr = err
					time.Sleep(1 * time.Second)
					continue
				} else {
					dbResult <- dbConnResult{conn: dbConn, err: nil}
					return
				}
			}
		}
	}()

	// Wait for database connection result
	dbRes := <-dbResult
	if dbRes.err != nil {
		panic(fmt.Sprintf("Failed to connect to database after 90 seconds. URL: %s, Last error: %v", dbURL, dbRes.err))
	}
	if dbRes.conn == nil {
		panic(fmt.Sprintf("Failed to connect to database after 90 seconds. URL: %s, Error: connection is nil", dbURL))
	}

	// Add timeout for Redis connection attempts using context
	redisCtx, redisCancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer redisCancel()

	redisResult := make(chan redisConnResult, 1)
	go func() {
		defer close(redisResult)
		var lastErr error
		for {
			select {
			case <-redisCtx.Done():
				redisResult <- redisConnResult{client: nil, err: lastErr}
				return
			default:
				// Use Redis password if provided
				opts := &redis.Options{
					Addr: cacheURL,
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
					DialTimeout: 5 * time.Second, // Shorter dial timeout
				}
				if redisPassword != "" {
					opts.Password = redisPassword
				}

				cache := redis.NewClient(opts)
				err := cache.Ping(redisCtx).Err()
				if err != nil {
					lastErr = err
					time.Sleep(1 * time.Second)
					continue
				} else {
					redisResult <- redisConnResult{client: cache, err: nil}
					return
				}
			}
		}
	}()

	// Wait for Redis connection result
	redisRes := <-redisResult
	if redisRes.err != nil {
		panic(fmt.Sprintf("Failed to connect to Redis after 90 seconds. URL: %s, Last error: %v", cacheURL, redisRes.err))
	}
	if redisRes.client == nil || redisRes.client.Ping(context.Background()).Err() != nil {
		panic(fmt.Sprintf("Failed to connect to Redis after 90 seconds. URL: %s, Error: connection is nil or ping failed", cacheURL))
	}

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
	polygonClient.HTTP.SetDisableWarn(true)
	polygonClient.HTTP.SetLogger(NoOp{})

	// Initialize Gemini API key pool
	geminiPool := initGeminiKeyPool()

	// Create local connection object (no global variable)
	localConn := &Conn{
		DB:              dbRes.conn,
		Cache:           redisRes.client,
		Polygon:         polygonClient,
		PolygonKey:      polygonKey,
		GeminiPool:      geminiPool,
		PerplexityKey:   perplexityKey,
		XAPIKey:         XAPIKey,
		TwitterAPIioKey: twitterAPIioKey,
		OpenAIKey:       openAIKey,
	}

	cleanup := func() {
		// Close the database connection
		if localConn.DB != nil {
			localConn.DB.Close()
		}

		// Close the Redis cache connection
		if localConn.Cache != nil {
			if err := localConn.Cache.Close(); err != nil {
				log.Printf("Error closing Redis cache connection: %v", err)
			}
		}
	}
	return localConn, cleanup
}

// Helper function to get environment variables with fallback
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// GeminiKeyInfo represents information about a Gemini API key
type GeminiKeyInfo struct {
	Key            string
	IsPaid         bool
	RequestCount   int
	LastReset      time.Time
	RateLimitTotal int // Total requests allowed per minute
	Mutex          sync.Mutex
}

// GeminiKeyPool manages a pool of Gemini API keys
type GeminiKeyPool struct {
	Keys        []*GeminiKeyInfo
	Mutex       sync.RWMutex
	LastUsedIdx int
}

// initGeminiKeyPool initializes the Gemini API key pool
func initGeminiKeyPool() *GeminiKeyPool {
	pool := &GeminiKeyPool{
		Keys:        make([]*GeminiKeyInfo, 0),
		LastUsedIdx: -1,
	}

	// Get paid key from environment variable
	paidKey := getEnv("GEMINI_API_KEY", "AIzaSyAcmVT51iORY1nFD3RLqYIP7Q4-4e5oS74")

	// Paid key has a higher limit, though it varies by plan
	paidRateLimit := 1000

	// Add paid key if provided
	if paidKey != "" {
		pool.Keys = append(pool.Keys, &GeminiKeyInfo{
			Key:            paidKey,
			IsPaid:         true,
			RequestCount:   0,
			LastReset:      time.Now(),
			RateLimitTotal: paidRateLimit,
		})
	}

	// Start a goroutine to reset request counts every minute
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			pool.resetCounts()
		}
	}()

	return pool
}

// resetCounts resets the request counts for all keys in the pool
func (p *GeminiKeyPool) resetCounts() {
	p.Mutex.Lock()
	defer p.Mutex.Unlock()

	now := time.Now()
	for _, keyInfo := range p.Keys {
		keyInfo.Mutex.Lock()
		// Only reset if a minute has passed since the last reset
		if now.Sub(keyInfo.LastReset) >= time.Minute {
			keyInfo.RequestCount = 0
			keyInfo.LastReset = now
		}
		keyInfo.Mutex.Unlock()
	}
}

// GetNextKey returns the paid API key if available and under rate limit
func (p *GeminiKeyPool) GetNextKey() (string, error) {
	p.Mutex.Lock()
	defer p.Mutex.Unlock()

	if len(p.Keys) == 0 {
		return "", fmt.Errorf("no API keys available in the pool")
	}

	// Since we only have the paid key, just use it
	keyInfo := p.Keys[0]

	keyInfo.Mutex.Lock()
	defer keyInfo.Mutex.Unlock()

	// Check if this key is under its rate limit
	if keyInfo.RequestCount < keyInfo.RateLimitTotal {
		keyInfo.RequestCount++
		return keyInfo.Key, nil
	}

	// If we get here, the paid key has reached its rate limit
	return "", fmt.Errorf("API key has reached its rate limit")
}

// GetGeminiKey is a convenience method on Conn to get the next available Gemini API key
func (c *Conn) GetGeminiKey() (string, error) {
	// Add nil pointer checks
	if c == nil {
		return "", fmt.Errorf("connection object is nil")
	}
	if c.GeminiPool == nil {
		return "", fmt.Errorf("gemini pool is not initialized")
	}
	return c.GeminiPool.GetNextKey()
}

// TestRedisConnectivity tests the Redis connection and returns success status and error message
func (c *Conn) TestRedisConnectivity(ctx context.Context, userID int) (bool, string) {
	// Add nil pointer check for the connection struct itself
	if c == nil {
		return false, "Connection object is nil"
	}

	// Add nil pointer check for the Redis cache
	if c.Cache == nil {
		return false, "Redis cache client is not initialized"
	}

	testKey := fmt.Sprintf("redis_test_key:%d", userID)
	testValue := fmt.Sprintf("test_value_%d_%d", userID, time.Now().Unix())

	// Try to write to Redis
	err := c.Cache.Set(ctx, testKey, testValue, 5*time.Minute).Err()
	if err != nil {
		return false, fmt.Sprintf("Redis write test failed: %v", err)
	}

	// Try to read from Redis
	val, err := c.Cache.Get(ctx, testKey).Result()
	if err != nil {
		return false, fmt.Sprintf("Redis read test failed: %v", err)
	}

	if val != testValue {
		return false, fmt.Sprintf("Redis read test returned unexpected value: %s", val)
	}

	return true, "Redis connection test successful"
}

type NoOp struct{}

func (NoOp) Printf(string, ...interface{}) {} // swallow logs
func (NoOp) Errorf(string, ...interface{}) {} // Add Errorf
func (NoOp) Warnf(string, ...interface{})  {} // Add Warnf
func (NoOp) Debugf(string, ...interface{}) {} // Add Debugf
