package data

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"
)

// Metrics counters for monitoring Redis operations
var (
	tickerUpdateCount   int64
	universeUpdateCount int64
	// Per-ticker throttling metrics
	strategyRuns     int64
	skippedNoUpdate  int64
	skippedBucketDup int64
	// Stage 3 enhanced metrics
	cleanupOperations   int64
	luaIntersections    int64
	universeDiscoveries int64
)

// GetAlertMetrics returns current Redis operation metrics
func GetAlertMetrics() map[string]int64 {
	return map[string]int64{
		"ticker_updates":     atomic.LoadInt64(&tickerUpdateCount),
		"universe_updates":   atomic.LoadInt64(&universeUpdateCount),
		"strategy_runs":      atomic.LoadInt64(&strategyRuns),
		"skipped_no_update":  atomic.LoadInt64(&skippedNoUpdate),
		"skipped_bucket_dup": atomic.LoadInt64(&skippedBucketDup),
	}
}

// IncrementStrategyRuns increments the count of strategy runs.
func IncrementStrategyRuns() {
	atomic.AddInt64(&strategyRuns, 1)
}

// IncrementSkippedNoUpdate increments the count of skipped runs due to no update.
func IncrementSkippedNoUpdate() {
	atomic.AddInt64(&skippedNoUpdate, 1)
}

// IncrementSkippedBucketDup increments the count of skipped runs due to duplicate buckets.
func IncrementSkippedBucketDup() {
	atomic.AddInt64(&skippedBucketDup, 1)
}

// MarkTickerUpdated records that a ticker received a price update at the given timestamp
// This is used to track which tickers have been updated for alert processing
func MarkTickerUpdated(conn *Conn, ticker string, timestampMs int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Use ZADD with CH option to update existing scores
	// Key: TICK:UPD, Score: timestampMs, Member: ticker
	err := conn.Cache.ZAdd(ctx, "TICK:UPD", &redis.Z{
		Score:  float64(timestampMs),
		Member: ticker,
	}).Err()

	if err != nil {
		log.Printf("⚠️ Failed to mark ticker %s as updated: %v", ticker, err)
		return err
	}

	atomic.AddInt64(&tickerUpdateCount, 1)
	return nil
}

// SetStrategyUniverse stores the complete ticker universe for a strategy in Redis
// This replaces any existing universe for the strategy
func SetStrategyUniverse(conn *Conn, strategyID int, tickers []string) error {
	if len(tickers) == 0 {
		// For global strategies or empty universes, we don't store anything
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := fmt.Sprintf("STRAT:%d:UNIV", strategyID)

	// Use a pipeline for efficiency
	pipe := conn.Cache.Pipeline()

	// Clear existing set
	pipe.Del(ctx, key)

	// Add all tickers to the set
	members := make([]interface{}, len(tickers))
	for i, ticker := range tickers {
		members[i] = ticker
	}
	pipe.SAdd(ctx, key, members...)

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		log.Printf("⚠️ Failed to set strategy %d universe: %v", strategyID, err)
		return err
	}

	atomic.AddInt64(&universeUpdateCount, 1)
	log.Printf("📝 Set strategy %d universe with %d tickers: %v", strategyID, len(tickers), tickers)
	return nil
}

// GetStrategyUniverse retrieves the ticker universe for a strategy from Redis
func GetStrategyUniverse(conn *Conn, strategyID int) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := fmt.Sprintf("STRAT:%d:UNIV", strategyID)

	members, err := conn.Cache.SMembers(ctx, key).Result()
	if err != nil {
		log.Printf("⚠️ Failed to get strategy %d universe: %v", strategyID, err)
		return nil, err
	}

	return members, nil
}

// GetTickersUpdatedSince returns all tickers that have been updated since the given timestamp
func GetTickersUpdatedSince(conn *Conn, sinceMs int64) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use ZRANGEBYSCORE to get all tickers updated since sinceMs
	tickers, err := conn.Cache.ZRangeByScore(ctx, "TICK:UPD", &redis.ZRangeBy{
		Min: strconv.FormatInt(sinceMs, 10),
		Max: "+inf",
	}).Result()

	if err != nil {
		log.Printf("⚠️ Failed to get tickers updated since %d: %v", sinceMs, err)
		return nil, err
	}

	return tickers, nil
}

// GetStrategyLastBuckets retrieves the last trigger bucket timestamps for specific tickers in a strategy
func GetStrategyLastBuckets(conn *Conn, strategyID int, tickers []string) (map[string]int64, error) {
	if len(tickers) == 0 {
		return make(map[string]int64), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := fmt.Sprintf("STRAT:%d:LAST", strategyID)

	// Convert tickers to interface{} slice for HMGET
	fields := make([]string, len(tickers))
	copy(fields, tickers)

	values, err := conn.Cache.HMGet(ctx, key, fields...).Result()
	if err != nil {
		log.Printf("⚠️ Failed to get strategy %d last buckets: %v", strategyID, err)
		return nil, err
	}

	result := make(map[string]int64)
	for i, value := range values {
		if value != nil {
			if timestampStr, ok := value.(string); ok {
				if timestamp, parseErr := strconv.ParseInt(timestampStr, 10, 64); parseErr == nil {
					result[tickers[i]] = timestamp
				}
			}
		}
		// If value is nil or can't be parsed, ticker won't be in result map (treated as never triggered)
	}

	return result, nil
}

// SetStrategyLastBuckets updates the last trigger bucket timestamps for specific tickers in a strategy
func SetStrategyLastBuckets(conn *Conn, strategyID int, tickerBuckets map[string]int64) error {
	if len(tickerBuckets) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := fmt.Sprintf("STRAT:%d:LAST", strategyID)

	// Convert to string map for Redis
	fields := make(map[string]interface{})
	for ticker, bucketMs := range tickerBuckets {
		fields[ticker] = strconv.FormatInt(bucketMs, 10)
	}

	err := conn.Cache.HMSet(ctx, key, fields).Err()
	if err != nil {
		log.Printf("⚠️ Failed to set strategy %d last buckets: %v", strategyID, err)
		return err
	}

	log.Printf("⏰ Updated strategy %d last buckets for %d tickers", strategyID, len(tickerBuckets))
	return nil
}

// CleanupTickerUpdates removes old entries from TICK:UPD to prevent unbounded growth
// Keeps entries from the last maxDays days to handle the longest possible bucket timeframes
func CleanupTickerUpdates(conn *Conn, maxDays int) error {
	ctx := context.Background()

	// Calculate cutoff time (maxDays ago)
	cutoffTime := time.Now().AddDate(0, 0, -maxDays)
	cutoffMs := cutoffTime.UnixMilli()

	// Remove entries older than cutoff
	removed, err := conn.Cache.ZRemRangeByScore(ctx, "TICK:UPD", "0", fmt.Sprintf("%d", cutoffMs)).Result()
	if err != nil {
		return fmt.Errorf("failed to cleanup TICK:UPD: %w", err)
	}

	if removed > 0 {
		log.Printf("🧹 Cleaned up %d old ticker update entries (older than %d days)", removed, maxDays)
		atomic.AddInt64(&cleanupOperations, 1)
	}

	return nil
}

// GetUniverseSize returns the size of a strategy's universe for metrics
func GetUniverseSize(conn *Conn, strategyID int) (int, error) {
	ctx := context.Background()
	key := fmt.Sprintf("STRAT:%d:UNIV", strategyID)

	size, err := conn.Cache.SCard(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get universe size for strategy %d: %w", strategyID, err)
	}

	return int(size), nil
}

// GetTickerUpdateCount returns the total number of tracked ticker updates
func GetTickerUpdateCount(conn *Conn) (int, error) {
	ctx := context.Background()

	count, err := conn.Cache.ZCard(ctx, "TICK:UPD").Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get ticker update count: %w", err)
	}

	return int(count), nil
}

// IntersectTickersServerSide performs ticker intersection using Lua script for large universes
func IntersectTickersServerSide(conn *Conn, strategyID int, sinceMs int64) ([]string, error) {
	ctx := context.Background()

	// Lua script to intersect ZRANGEBYSCORE result with SMEMBERS result
	luaScript := `
		local strategy_key = KEYS[1]
		local tick_key = KEYS[2] 
		local since_ms = ARGV[1]
		
		-- Get updated tickers since timestamp
		local updated = redis.call('ZRANGEBYSCORE', tick_key, since_ms, '+inf')
		
		-- Get strategy universe
		local universe = redis.call('SMEMBERS', strategy_key)
		
		-- Create lookup table for universe
		local universe_set = {}
		for _, ticker in ipairs(universe) do
			universe_set[ticker] = true
		end
		
		-- Find intersection
		local result = {}
		for _, ticker in ipairs(updated) do
			if universe_set[ticker] then
				table.insert(result, ticker)
			end
		end
		
		return result
	`

	strategyKey := fmt.Sprintf("STRAT:%d:UNIV", strategyID)
	tickKey := "TICK:UPD"

	result, err := conn.Cache.Eval(ctx, luaScript, []string{strategyKey, tickKey}, sinceMs).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to execute server-side intersection: %w", err)
	}

	// Convert result to string slice
	tickers := make([]string, 0)
	if resultSlice, ok := result.([]interface{}); ok {
		for _, item := range resultSlice {
			if ticker, ok := item.(string); ok {
				tickers = append(tickers, ticker)
			}
		}
	}

	return tickers, nil
}

// IncrementCleanupOperations tracks cleanup operations
func IncrementCleanupOperations() {
	atomic.AddInt64(&cleanupOperations, 1)
}

// IncrementLuaIntersections tracks server-side intersections
func IncrementLuaIntersections() {
	atomic.AddInt64(&luaIntersections, 1)
}

// IncrementUniverseDiscoveries tracks worker-reported universe updates
func IncrementUniverseDiscoveries() {
	atomic.AddInt64(&universeDiscoveries, 1)
}

// GetDetailedAlertMetrics returns enhanced metrics including performance data
func GetDetailedAlertMetrics(conn *Conn) map[string]interface{} {
	// Add Redis data sizes
	tickerCount, _ := GetTickerUpdateCount(conn)

	return map[string]interface{}{
		"ticker_updates":       atomic.LoadInt64(&tickerUpdateCount),
		"universe_updates":     atomic.LoadInt64(&universeUpdateCount),
		"strategy_runs":        atomic.LoadInt64(&strategyRuns),
		"skipped_no_update":    atomic.LoadInt64(&skippedNoUpdate),
		"skipped_bucket_dup":   atomic.LoadInt64(&skippedBucketDup),
		"cleanup_operations":   atomic.LoadInt64(&cleanupOperations),
		"lua_intersections":    atomic.LoadInt64(&luaIntersections),
		"universe_discoveries": atomic.LoadInt64(&universeDiscoveries),
		"total_ticker_updates": tickerCount,
	}
}
