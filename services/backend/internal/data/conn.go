package data

import (
	"context"
	"fmt"
	"log"

	//	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v4/pgxpool"
	polygon "github.com/polygon-io/client-go/rest"
)

// Conn represents a structure for handling Conn data.
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

	var dbConn *pgxpool.Pool
	var err error

	// Add timeout for database connection attempts using context
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	done := make(chan bool)
	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Create a connection pool configuration
				poolConfig, parseErr := pgxpool.ParseConfig(dbURL)
				if parseErr != nil {
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
				dbConn, err = pgxpool.ConnectConfig(ctx, poolConfig)
				if err != nil {
					time.Sleep(1 * time.Second)
					continue
				} else {
					return
				}
			}
		}
	}()

	select {
	case <-done:
		// Connection successful
	case <-ctx.Done():
		// Timeout occurred
		panic(fmt.Sprintf("Failed to connect to database after 90 seconds. URL: %s, Last error: %v", dbURL, err))
	}

	// If database connection still failed, panic with informative error
	if dbConn == nil {
		panic(fmt.Sprintf("Failed to connect to database after 90 seconds. URL: %s, Error: %v", dbURL, err))
	}

	var cache *redis.Client

	// Add timeout for Redis connection attempts using context
	redisCtx, redisCancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer redisCancel()

	redisDone := make(chan bool)
	go func() {
		defer close(redisDone)
		for {
			select {
			case <-redisCtx.Done():
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

				cache = redis.NewClient(opts)
				err = cache.Ping(redisCtx).Err()
				if err != nil {
					time.Sleep(1 * time.Second)
					continue
				} else {
					return
				}
			}
		}
	}()

	select {
	case <-redisDone:
		// Connection successful
	case <-redisCtx.Done():
		// Timeout occurred
		panic(fmt.Sprintf("Failed to connect to Redis after 90 seconds. URL: %s, Last error: %v", cacheURL, err))
	}

	// If Redis connection still failed, panic with informative error
	if cache == nil || cache.Ping(context.Background()).Err() != nil {
		panic(fmt.Sprintf("Failed to connect to Redis after 90 seconds. URL: %s, Error: %v", cacheURL, err))
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

	conn = &Conn{
		DB:              dbConn,
		Cache:           cache,
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
		conn.DB.Close()

		// Close the Redis cache connection
		if err := conn.Cache.Close(); err != nil {
			log.Printf("Error closing Redis cache connection: %v", err)
		}
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

	// Get keys from environment variables
	// Format expected: GEMINI_FREE_KEYS=key1,key2,key3 and GEMINI_PAID_KEY=paidkey
	freeKeysStr := getEnv("GEMINI_FREE_KEYS", "")
	paidKey := getEnv("GEMINI_PAID_KEY", "")

	// Default rate limit for free keys (Gemini typically allows 60 requests per minute for free tier)
	freeRateLimit := 15
	// Paid key has a higher limit, though it varies by plan
	paidRateLimit := 1000

	// Parse and add free keys
	if freeKeysStr != "" {
		// Split the comma-separated string
		freeKeys := strings.Split(freeKeysStr, ",")
		for _, key := range freeKeys {
			key = strings.TrimSpace(key)
			if key != "" {
				pool.Keys = append(pool.Keys, &GeminiKeyInfo{
					Key:            key,
					IsPaid:         false,
					RequestCount:   0,
					LastReset:      time.Now(),
					RateLimitTotal: freeRateLimit,
				})
			}
		}
	}

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

// GetNextKey returns the next available API key based on the rotation strategy
func (p *GeminiKeyPool) GetNextKey() (string, error) {
	p.Mutex.Lock()
	defer p.Mutex.Unlock()

	if len(p.Keys) == 0 {
		return "", fmt.Errorf("no API keys available in the pool")
	}

	// First try to find a non-paid key under the rate limit
	for i := 0; i < len(p.Keys); i++ {
		// Round-robin selection, starting after the last used index
		idx := (p.LastUsedIdx + 1 + i) % len(p.Keys)
		keyInfo := p.Keys[idx]

		// Skip paid keys in the first pass
		if keyInfo.IsPaid {
			continue
		}

		keyInfo.Mutex.Lock()
		// Check if this key is under its rate limit
		if keyInfo.RequestCount < keyInfo.RateLimitTotal {
			keyInfo.RequestCount++
			p.LastUsedIdx = idx
			keyInfo.Mutex.Unlock()
			return keyInfo.Key, nil
		}
		keyInfo.Mutex.Unlock()
	}

	// If all free keys are at their limit, try the paid key
	for i := 0; i < len(p.Keys); i++ {
		idx := (p.LastUsedIdx + 1 + i) % len(p.Keys)
		keyInfo := p.Keys[idx]

		// Now only consider paid keys
		if !keyInfo.IsPaid {
			continue
		}

		keyInfo.Mutex.Lock()
		// Check if this paid key is under its rate limit
		if keyInfo.RequestCount < keyInfo.RateLimitTotal {
			keyInfo.RequestCount++
			p.LastUsedIdx = idx
			keyInfo.Mutex.Unlock()
			return keyInfo.Key, nil
		}
		keyInfo.Mutex.Unlock()
	}

	// If we get here, all keys (including paid ones) are at their rate limit
	return "", fmt.Errorf("all API keys have reached their rate limits")
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
