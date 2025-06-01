package data

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v4/pgxpool"
)

func InitTestConn(t *testing.T) (*Conn, func()) {
	dbConn, cleanup := initDevCopyDatabase(t)

	cache := initTestRedis(t)

	conn := &Conn{
		DB:            dbConn,
		Cache:         cache,
		Polygon:       nil,
		PolygonKey:    "",
		GeminiPool:    nil,
		PerplexityKey: "",
	}

	return conn, cleanup
}

func initDevCopyDatabase(t *testing.T) (*pgxpool.Pool, func()) {
	devConn := connectToDevDB(t)
	defer devConn.Close()

	testDBName := fmt.Sprintf("test_db_%d_%d",
		time.Now().Unix(),
		time.Now().Nanosecond())

	ctx := context.Background()

	templateDBName := "dev_template"

	// Check if template exists, if not create it
	var templateExists bool
	err := devConn.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)",
		templateDBName).Scan(&templateExists)
	if err != nil {
		t.Fatalf("Failed to check template existence: %v", err)
	}

	if !templateExists {
		sourceDBName := "postgres"
		// Create template from your dev database
		t.Logf("Creating template database from %s (this may take a moment)...", sourceDBName)

		_, err = devConn.Exec(ctx, fmt.Sprintf(
			"CREATE DATABASE %s WITH TEMPLATE %s",
			templateDBName, sourceDBName))
		if err != nil {
			t.Logf("Failed to create template database: %v", err)
			t.Fatalf("You may need to shutdown the backend to create the template")
		}
		t.Logf("✅ Created template database from dev DB")
	}

	// Create test database from template
	_, err = devConn.Exec(ctx, fmt.Sprintf(
		"CREATE DATABASE %s WITH TEMPLATE %s",
		testDBName, templateDBName))
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	t.Logf("✅ Created test database: %s", testDBName)

	// Connect to the new test database
	testConn := connectToTestDB(t, testDBName)

	cleanup := func() {
		if testConn != nil {
			testConn.Close()
		}

		devConn := connectToDevDB(t)
		defer devConn.Close()

		_, err := devConn.Exec(ctx, fmt.Sprintf(`
            SELECT pg_terminate_backend(pid)
            FROM pg_stat_activity 
            WHERE datname = '%s' AND pid <> pg_backend_pid()
        `, testDBName))
		if err != nil {
			t.Logf("Warning: Failed to terminate connections: %v", err)
		}

		// Drop test database
		_, err = devConn.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", testDBName))
		if err != nil {
			t.Logf("Warning: Failed to drop test database: %v", err)
		} else {
			t.Logf("🧹 Cleaned up test database: %s", testDBName)
		}
	}

	return testConn, cleanup
}

func connectToDevDB(t *testing.T) *pgxpool.Pool {
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "postgres")
	dbPassword := getEnv("DB_PASSWORD", "devpassword")
	dbName := getEnv("POSTGRES_DB", "postgres")

	encodedPassword := url.QueryEscape(dbPassword)
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		dbUser, encodedPassword, dbHost, dbPort, dbName)

	poolConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		t.Fatalf("Unable to parse dev database config: %v", err)
	}

	poolConfig.MaxConns = 5
	poolConfig.MinConns = 1
	poolConfig.ConnConfig.ConnectTimeout = 5 * time.Second

	dbConn, err := pgxpool.ConnectConfig(context.Background(), poolConfig)
	if err != nil {
		t.Fatalf("Unable to connect to dev database: %v", err)
	}

	return dbConn
}

func connectToTestDB(t *testing.T, testDBName string) *pgxpool.Pool {
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "postgres")
	dbPassword := getEnv("DB_PASSWORD", "devpassword")

	encodedPassword := url.QueryEscape(dbPassword)
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		dbUser, encodedPassword, dbHost, dbPort, testDBName)

	poolConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		t.Fatalf("Unable to parse test database config: %v", err)
	}

	poolConfig.MaxConns = 10
	poolConfig.MinConns = 2
	poolConfig.ConnConfig.ConnectTimeout = 5 * time.Second

	dbConn, err := pgxpool.ConnectConfig(context.Background(), poolConfig)
	if err != nil {
		t.Fatalf("Unable to connect to test database: %v", err)
	}

	return dbConn
}

func initTestRedis(t *testing.T) *redis.Client {
	redisHost := getEnv("REDIS_HOST", "localhost")
	redisPort := getEnv("REDIS_PORT", "6379")

	client := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort),
		DB:   1, // Use different Redis DB for tests
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		// FORNOW not using redis
		//t.Fatalf("Unable to ping test redis: %v", err)
	}

	return client
}
