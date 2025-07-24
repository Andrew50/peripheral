# Stage Table Optimization for refresh_static_refs Functions

## Overview

This optimization replaces the pattern of creating and dropping temporary tables multiple times per refresh cycle with persistent UNLOGGED stage tables that are reused via `TRUNCATE` operations.

## Problem Statement

The original issue was that stage tables were being created and dropped 49× per run during the `refresh_static_refs_*` function executions. Even at 15ms each, this added approximately 735ms of overhead per full refresh cycle.

## Solution

### 1. Persistent UNLOGGED Stage Tables

Created persistent UNLOGGED tables with the following characteristics:
- `autovacuum_enabled = false` for better performance
- Indexed on `ticker` column for fast lookups
- Reused across function calls with `TRUNCATE` instead of `DROP/CREATE`

### 2. Stage Tables Created

#### Static Refs Tables:
- `static_refs_active_securities_stage` - Active securities lookup
- `static_refs_1m_prices_stage` - 1-minute price lookups
- `static_refs_daily_prices_stage` - Daily price lookups

#### Screener Tables (for large batch processing):
- `screener_stale_tickers_stage` - Stale tickers list
- `screener_latest_daily_stage` - Latest daily OHLCV data
- `screener_latest_minute_stage` - Latest minute OHLCV data
- `screener_security_info_stage` - Security metadata

### 3. Optimized Functions

#### `refresh_static_refs_1m()`
- Step 1: Populate active securities stage table
- Step 2: Bulk-populate 1m prices using LATERAL joins
- Step 3: Bulk upsert to final table

#### `refresh_static_refs()`
- Step 1: Populate active securities stage table
- Step 2: Bulk-populate daily prices using LATERAL joins
- Step 3: Bulk upsert to final table

#### `refresh_screener_staged()` (Alternative Implementation)
- Provides staged approach for screener refresh when processing large batches
- Can be used instead of CTE-based approach for better performance with large datasets

## Performance Benefits

1. **Eliminates Table Creation Overhead**: ~735ms saved per refresh cycle
2. **Reduces Memory Fragmentation**: No repeated allocation/deallocation
3. **Better Cache Locality**: Stage tables remain in buffer cache
4. **Maintains Transactional Safety**: UNLOGGED tables still provide ACID within transaction

## Usage

### Automatic Optimization
The existing `refresh_static_refs()` and `refresh_static_refs_1m()` functions are automatically optimized and maintain the same interface.

### Alternative Screener Function
For large batch processing, you can optionally use:
```sql
SELECT refresh_screener_staged(1000);
```

### Maintenance
If needed, you can clean up stage tables:
```sql
SELECT cleanup_static_refs_stage_tables();
```

## Migration

The optimization is implemented in migration `074_optimize_static_refs_with_persistent_stage_tables.sql` and can be applied without downtime. The functions maintain backward compatibility.

## Monitoring

Monitor the performance improvement by comparing execution times of:
- `refresh_static_refs()`
- `refresh_static_refs_1m()`

Expected improvement: ~735ms reduction per full refresh cycle. 