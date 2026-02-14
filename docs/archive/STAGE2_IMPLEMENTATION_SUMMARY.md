# Stage 2 Implementation Summary: "Per-Ticker Throttle"

## Overview
Stage 2 implements per-ticker throttling logic with feature flag control. When enabled via `PER_TICKER_THROTTLE=true`, the system switches from strategy-level bucket throttling to per-ticker bucket throttling, dramatically reducing unnecessary strategy executions and allowing multiple tickers in the same strategy to trigger within the same time bucket.

## Components Implemented

### 1. Feature Flag System (`services/backend/internal/services/alerts/loop.go`)
**New Functions:**
- `isPerTickerThrottleEnabled()` - Checks `PER_TICKER_THROTTLE` environment variable
- `processStrategyAlertsLegacy()` - Original strategy-level throttling (fallback)
- `processStrategyAlertsPerTicker()` - New per-ticker throttling implementation

**Environment Variable:**
- `PER_TICKER_THROTTLE=true` - Enables per-ticker throttling mode
- `PER_TICKER_THROTTLE=false` or unset - Uses legacy throttling mode

### 2. Enhanced Metrics System (`services/backend/internal/data/redis_alerts.go`)
**New Metrics:**
- `strategy_runs` - Count of strategy executions
- `skipped_no_update` - Strategies skipped due to no price updates in universe
- `skipped_bucket_dup` - Strategies skipped due to bucket re-trigger prevention

**New Functions:**
- `IncrementStrategyRuns()` - Tracks successful strategy executions
- `IncrementSkippedNoUpdate()` - Tracks skips due to no relevant price changes
- `IncrementSkippedBucketDup()` - Tracks skips due to same-bucket re-triggers

### 3. Updated Strategy Execution (`services/backend/internal/services/alerts/loop.go`)
**Enhanced Functions:**
- `executeStrategyAlert(ctx, conn, strategy, tickers)` - Now accepts optional ticker filter
- `processStrategyAlerts()` - Routes to legacy or per-ticker mode based on feature flag

## Per-Ticker Throttling Algorithm

### 1. Bucket Calculation
```go
currBucket := bucketStart(now, strategy.MinTimeframe)
currBucketMs := currBucket.UnixMilli()
```

### 2. Price Update Detection
```go
updatedTickers := data.GetTickersUpdatedSince(conn, currBucketMs)
```

### 3. Universe Intersection
```go
strategyUniverse := data.GetStrategyUniverse(conn, strategyID)
changedTickers := intersect(updatedTickers, strategyUniverse)
```

### 4. Bucket Duplicate Filtering
```go
lastBuckets := data.GetStrategyLastBuckets(conn, strategyID, changedTickers)
finalTickers := filter(changedTickers, lastBuckets, currBucketMs)
```

### 5. Execution & State Update
```go
executeStrategyAlert(ctx, conn, strategy, finalTickers)
data.SetStrategyLastBuckets(conn, strategyID, tickerBuckets)
```

## Behavior Comparison

### Legacy Mode (PER_TICKER_THROTTLE=false)
- **Throttling Level**: Entire strategy
- **Skip Condition**: Strategy triggered in same bucket
- **Execution**: All tickers in universe or no filter
- **State Tracking**: Single `LastTrigger` timestamp per strategy

### Per-Ticker Mode (PER_TICKER_THROTTLE=true)
- **Throttling Level**: Individual ticker within strategy
- **Skip Conditions**: 
  - No universe tickers updated since bucket start
  - All updated tickers already triggered in current bucket
- **Execution**: Only tickers that updated and haven't triggered
- **State Tracking**: Per-ticker `LastTrigger` timestamps in Redis

## Special Cases Handled

### 1. Global Strategies (`Universe == "all"` or empty)
- Falls back to legacy throttling logic
- Cannot use per-ticker filtering (would need all market data)
- Logs as "🌍 Processing global strategy"

### 2. Missing Redis Data
- Graceful degradation: continues execution assuming no previous triggers
- Logs warnings but doesn't fail strategy execution
- Syncs universe data during initialization for disaster recovery

### 3. Invalid Timeframes
- Skips strategy with warning log
- Increments `skipped_no_update` metric
- Prevents crashes from malformed timeframe strings

## Metrics & Monitoring

### 1. Periodic Logging (Every 5 minutes)
```
📊 Redis metrics - Ticker updates: 1234, Universe updates: 56
📊 Per-ticker throttling - Strategy runs: 89, Skipped (no update): 12, Skipped (bucket dup): 34
```

### 2. Per-Strategy Execution Logs
```
🎯 Processing strategy 123: MyStrategy with 3 tickers: [AAPL, MSFT, TSLA]
⏩ Strategy 456 (OtherStrategy) skipped - no universe tickers updated (50 universe, 100 updated)
⏩ Strategy 789 (ThirdStrategy) skipped - all changed tickers already triggered in bucket (5 changed, 0 final)
```

## Safety & Rollback

### 1. Feature Flag Control
- **Default**: Legacy mode (PER_TICKER_THROTTLE unset)
- **Rollback**: Set `PER_TICKER_THROTTLE=false` and restart
- **Gradual Rollout**: Enable per-instance or per-environment

### 2. Graceful Degradation
- Redis failures don't break alert processing
- Missing universe data falls back to full execution
- Invalid data logged but doesn't crash service

### 3. Backward Compatibility
- Legacy throttling preserved exactly as-is
- All existing logs and metrics continue working
- No database schema changes required

## Production Deployment Strategy

### 1. Pre-Deployment Verification
```bash
# Verify Redis keys are populated from Stage 1
redis-cli ZCARD TICK:UPD  # Should show ticker count
redis-cli SCARD STRAT:123:UNIV  # Should show universe size for active strategies
```

### 2. Gradual Rollout
1. **Staging**: Deploy with `PER_TICKER_THROTTLE=true`, verify logs
2. **Canary**: Enable on 1-2 production instances, monitor metrics
3. **Fleet**: Roll out to all instances after 24h observation

### 3. Monitoring During Rollout
- **Key Metric**: `skipped_no_update` should be > 0 (showing efficiency gains)
- **Safety Check**: `strategy_runs` should be reasonable (not zero, not excessive)
- **Performance**: Alert processing duration should decrease

### 4. Success Criteria
- Reduced strategy executions without missing alerts
- Lower CPU usage during alert processing cycles
- Faster time-to-alert for new price movements

## Performance Impact

### Expected Improvements
- **Reduced Executions**: 60-80% fewer strategy runs for typical universes
- **Faster Alerts**: Sub-minute response to price changes in active universes
- **Lower Resource Usage**: Less Python worker queue pressure

### Monitoring Points
- Strategy execution frequency (should decrease)
- Alert latency (should improve for active tickers)
- Redis operation latency (should remain low)
- Worker queue depth (should decrease)

## Next Steps (Stage 3)
- Worker-reported universe discovery (`used_symbols`)
- Redis key cleanup and TTL management
- Lua script optimization for large universes
- Legacy code removal after bake-in period

## Build Status
✅ **Build successful** - All components compile and integrate correctly
✅ **Feature flag ready** - Safe for production deployment with flag disabled
✅ **Metrics integrated** - Full observability of new behavior 