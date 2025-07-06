package strategy

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

func SetBacktestToCache(ctx context.Context, conn *data.Conn, userID int, strategyID int, response BacktestResponse) error {
	cacheKey := fmt.Sprintf(BacktestCacheKey, userID, strategyID)

	cacheData, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("error marshaling backtest response: %v", err)
	}
	cachedDataTTL := 48 * time.Hour
	return conn.Cache.Set(ctx, cacheKey, cacheData, cachedDataTTL).Err()
}

func GetBacktestFromCache(ctx context.Context, conn *data.Conn, userID int, strategyID int) (*BacktestResponse, error) {
	cacheKey := fmt.Sprintf(BacktestCacheKey, userID, strategyID)

	cacheData, err := conn.Cache.Get(ctx, cacheKey).Result()
	if err != nil {
		if err == redis.Nil {
			// Cache miss - run backtest and cache result
			rawArgs := json.RawMessage(fmt.Sprintf(`{"strategyId": %d}`, strategyID))
			backtestResponse, err := RunBacktest(ctx, conn, userID, rawArgs)
			if err != nil {
				return nil, fmt.Errorf("error running backtest: %v", err)
			}
			return backtestResponse.(*BacktestResponse), nil
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
func InvalidateBacktestInstancesCache(ctx context.Context, conn *data.Conn, userID int, strategyID int) error {
	cacheKey := fmt.Sprintf(BacktestCacheKey, userID, strategyID)

	return conn.Cache.Del(ctx, cacheKey).Err()
}
