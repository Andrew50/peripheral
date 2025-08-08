package strategy

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// BacktestCacheKey is the Redis cache key format for storing backtest results
const BacktestCacheKey = "backtest:userID:%d:strategyID:%d:version:%d"

// SetBacktestToCache stores a backtest response in Redis cache with TTL
func SetBacktestToCache(ctx context.Context, conn *data.Conn, userID int, strategyID int, version int, response BacktestResponse) error {
	cacheKey := fmt.Sprintf(BacktestCacheKey, userID, strategyID, version)

	cacheData, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("error marshaling backtest response: %v", err)
	}
	cachedDataTTL := 128 * time.Hour
	return conn.Cache.Set(ctx, cacheKey, cacheData, cachedDataTTL).Err()
}

// GetBacktestFromCache retrieves a cached backtest response or computes and caches it on a miss.
func GetBacktestFromCache(ctx context.Context, conn *data.Conn, userID int, strategyID int, version int) (*BacktestResponse, error) {
	cacheKey := fmt.Sprintf(BacktestCacheKey, userID, strategyID, version)

	cacheData, err := conn.Cache.Get(ctx, cacheKey).Result()
	if err != nil {
		if err == redis.Nil {
			// Cache miss - run backtest and cache result
			rawArgs := json.RawMessage(fmt.Sprintf(`{"strategyId": %d, "version": %d}`, strategyID, version))
			backtestResponse, err := RunBacktest(ctx, conn, userID, rawArgs)
			if err != nil {
				return nil, fmt.Errorf("error running backtest: %v", err)
			}

			// Handle both pointer and value types conservatively.
			switch v := backtestResponse.(type) {
			case *BacktestResponse:
				return v, nil
			case BacktestResponse:
				return &v, nil
			default:
				return nil, fmt.Errorf("unexpected type %T returned from RunBacktest", backtestResponse)
			}
		}
		return nil, fmt.Errorf("error getting backtest cache: %v", err)
	}

	// Cache hit - unmarshal and return
	var response BacktestResponse
	if err := json.Unmarshal([]byte(cacheData), &response); err != nil {
		// Cache corruption - delete corrupted entry and return error
		conn.Cache.Del(ctx, cacheKey)
		return nil, fmt.Errorf("error unmarshaling cached backtest data: %v", err)
	}

	return &response, nil
}

// InvalidateBacktestInstancesCache removes a cached backtest response for the given identifiers.
func InvalidateBacktestInstancesCache(ctx context.Context, conn *data.Conn, userID int, strategyID int, version int) error {
	cacheKey := fmt.Sprintf(BacktestCacheKey, userID, strategyID, version)

	return conn.Cache.Del(ctx, cacheKey).Err()
}
