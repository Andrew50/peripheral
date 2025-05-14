package strategy

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// SaveBacktestToCache saves the results of a backtest to Redis.
func SaveBacktestToCache(ctx context.Context, conn *data.Conn, userID int, strategyID int, results interface{}) error {
	if results == nil {
		return fmt.Errorf("cannot save nil backtest results")
	}

	// Construct the cache key
	cacheKey := fmt.Sprintf("user:%d:backtest:%d:results", userID, strategyID)

	// Serialize the results to JSON
	serializedResults, err := json.Marshal(results)
	if err != nil {
		////fmt.Printf("Failed to serialize backtest results for strategy %d: %v\n", strategyID, err)
		return fmt.Errorf("failed to serialize backtest results: %w", err)
	}

	// Define an expiration time (e.g., 24 hours)
	expiration := 24 * time.Hour

	// Save to Redis
	////fmt.Printf("Saving backtest results for strategy %d to cache key: %s\n", strategyID, cacheKey)
	err = conn.Cache.Set(ctx, cacheKey, serializedResults, expiration).Err()
	if err != nil {
		////fmt.Printf("Failed to save backtest results to Redis for strategy %d: %v\n", strategyID, err)
		return fmt.Errorf("failed to save backtest results to cache: %w", err)
	}

	////fmt.Printf("Successfully saved backtest results for strategy %d to Redis.\n", strategyID)
	return nil
}
