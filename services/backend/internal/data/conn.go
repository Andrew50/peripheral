// Package data provides database connection and data access functionality
package data

import (
	"context"
	"fmt"
	"log"

	//	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v4/pgxpool"
	polygon "github.com/polygon-io/client-go/rest"
	"google.golang.org/genai"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// Conn encapsulates database connections and API clients
type Conn struct {
	//Cache *redis.Client
	DB                   *pgxpool.Pool
	Polygon              *polygon.Client
	Cache                *redis.Client
	PolygonKey           string
	PerplexityKey        string
	GrokAPIKey           string
	TwitterAPIioKey      string
	OpenAIKey            string
	XAPIKey              string
	XAPISecretKey        string
	XAccessToken         string
	XAccessSecret        string
	GeminiClient         *genai.Client
	OpenAIClient         openai.Client
	ExecutionEnvironment string
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
	polygonKey := getEnv("POLYGON_API_KEY", "")
	perplexityKey := getEnv("PERPLEXITY_API_KEY", "")
	grokAPIKey := getEnv("GROK_API_KEY", "")
	twitterAPIioKey := getEnv("TWITTER_API_IO_KEY", "")
	openAIKey := getEnv("OPENAI_API_KEY", "")

	xAPIKey := getEnv("X_API_KEY", "")
	xAPISecretKey := getEnv("X_API_SECRET", "")
	xAccessToken := getEnv("X_ACCESS_TOKEN", "")
	xAccessSecret := getEnv("X_ACCESS_SECRET", "")

	geminiAPIKey := getEnv("GEMINI_API_KEY", "AIzaSyAcmVT51iORY1nFD3RLqYIP7Q4-4e5oS74")

	executionEnvironment := getEnv("ENVIRONMENT", "")
	if executionEnvironment == "" || executionEnvironment == "dev" || executionEnvironment == "development" {
		executionEnvironment = "dev"
	} else {
		executionEnvironment = "prod"
	}

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
				poolConfig.MaxConns = 50                                // FIXED: Increased from 30 for better concurrency during streaming
				poolConfig.MinConns = 10                                // FIXED: Increased from 5 for better performance
				poolConfig.MaxConnLifetime = 60 * time.Minute           // FIXED: 1 hour to prevent stale connections while still mitigating connection churn
				poolConfig.MaxConnIdleTime = 5 * time.Minute            // FIXED: Increased from 1 minute to reduce connection churn
				poolConfig.HealthCheckPeriod = 30 * time.Second         // FIXED: Increased from 15 seconds for more frequent health checks
				poolConfig.ConnConfig.ConnectTimeout = 10 * time.Second // FIXED: Increased from 5 seconds for slower connections
				/*poolConfig.BeforeConnect = func(ctx context.Context, cc *pgx.ConnConfig) error {
					// Validate connection before use
					return nil
				}
				poolConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
					// Validate connection after creation
					return conn.Ping(ctx)
				}*/

				// Create the connection pool with our custom configuration
				dbConn, err := pgxpool.ConnectConfig(ctx, poolConfig)
				if err != nil {
					lastErr = err
					time.Sleep(1 * time.Second)
					continue
				}
				dbResult <- dbConnResult{conn: dbConn, err: nil}
				return
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
				}
				redisResult <- redisConnResult{client: cache, err: nil}
				return
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

	// Create gemini client
	geminiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  geminiAPIKey,
		Backend: genai.BackendGeminiAPI,
	})

	if err != nil {
		panic(fmt.Sprintf("Failed to create Gemini client: %v", err))
	}
	openAIClient := openai.NewClient(option.WithAPIKey(openAIKey))

	// Create local connection object (no global variable)
	localConn := &Conn{
		DB:                   dbRes.conn,
		Cache:                redisRes.client,
		Polygon:              polygonClient,
		PolygonKey:           polygonKey,
		PerplexityKey:        perplexityKey,
		GrokAPIKey:           grokAPIKey,
		TwitterAPIioKey:      twitterAPIioKey,
		OpenAIKey:            openAIKey,
		XAPIKey:              xAPIKey,
		XAPISecretKey:        xAPISecretKey,
		XAccessToken:         xAccessToken,
		XAccessSecret:        xAccessSecret,
		GeminiClient:         geminiClient,
		ExecutionEnvironment: executionEnvironment,
		OpenAIClient:         openAIClient,
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

// GetGeminiKey gets the GEMINI api key
func (c *Conn) GetGeminiKey() (string, error) {
	// Add nil pointer checks
	if c == nil {
		return "", fmt.Errorf("connection object is nil")
	}
	return getEnv("GEMINI_API_KEY", "AIzaSyAcmVT51iORY1nFD3RLqYIP7Q4-4e5oS74"), nil
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

// NoOp is a no-operation logger implementation that discards all log messages
type NoOp struct{}

// Printf implements the Printf method of the logger interface but does nothing with the input
func (NoOp) Printf(string, ...interface{}) {} // swallow logs

// Errorf implements the Errorf method of the logger interface but does nothing with the input
func (NoOp) Errorf(string, ...interface{}) {} // Add Errorf

// Warnf implements the Warnf method of the logger interface but does nothing with the input
func (NoOp) Warnf(string, ...interface{}) {} // Add Warnf

// Debugf implements the Debugf method of the logger interface but does nothing with the input
func (NoOp) Debugf(string, ...interface{}) {} // Add Debugf
