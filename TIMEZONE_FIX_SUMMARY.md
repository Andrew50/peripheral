# Timezone Fix Implementation Summary

## Problem
Strategies using `get_bar_data()` were incorrectly reporting that events happened a day before they actually did. This was caused by inconsistent timezone handling between the old and new versions of `data_accessors.py`.

## Root Cause
The new aggregation path using TimescaleDB's `time_bucket()` was performing bucketing in UTC timezone, but strategies were interpreting the resulting timestamps as Eastern Time (America/New_York). This caused a 4-5 hour offset that appeared as events happening "one day earlier".

## Solution Implemented
1. **Added timezone constants and helper function**:
   ```python
   TZ = "'America/New_York'"
   
   def _ts_to_epoch(expr: str) -> str:
       """Convert a timestamptz expression to Unix epoch seconds in America/New_York timezone"""
       return f"EXTRACT(EPOCH FROM ({expr}) AT TIME ZONE {TZ})::bigint"
   ```

2. **Standardized timestamp handling across all query paths**:
   - **Direct queries**: Use `_ts_to_epoch('o.timestamp')` instead of raw `EXTRACT(EPOCH FROM o.timestamp)`
   - **Aggregated queries**: Use `_ts_to_epoch('bucket_ts')` for final timestamp conversion
   - **Time bucketing**: Ensure `time_bucket()` result is converted back to America/New_York timezone

3. **Fixed aggregation CTE timestamp generation**:
   ```sql
   -- Before (UTC bucketing):
   time_bucket(%s, o.timestamp AT TIME ZONE 'America/New_York') AS bucket_ts
   
   -- After (Eastern bucketing):
   time_bucket(%s, o.timestamp AT TIME ZONE 'America/New_York') AT TIME ZONE 'America/New_York' AS bucket_ts
   ```

4. **Standardized extended hours filtering**:
   - All extended hours filters now use the `TZ` constant for consistency
   - Ensures all timezone references are centralized and consistent

## Files Modified
- `services/worker/src/utils/data_accessors.py`: All timestamp handling standardized

## Key Changes Made
1. Line ~25: Added `TZ` constant and `_ts_to_epoch()` helper function
2. Line ~301: Updated aggregated query timestamp conversion
3. Line ~370: Updated direct query timestamp conversion
4. Line ~572: Updated backtest mode timestamp conversion  
5. Line ~622: Updated realtime mode timestamp conversion
6. Line ~657: Updated direct table access timestamp conversion
7. Line ~1161: Fixed time_bucket to maintain Eastern timezone
8. Lines ~397, ~713: Updated extended hours filters to use TZ constant

## Benefits
- **Consistency**: Both direct and aggregated query paths now handle timestamps identically
- **Correctness**: All timestamps are properly converted to Eastern time before being returned to strategies
- **Maintainability**: Centralized timezone handling through constants and helper functions
- **Future-proofing**: Easy to change market timezone if needed (e.g., for European markets)

## Backward Compatibility
- No changes required to existing strategy code
- All strategies continue to receive Unix epoch timestamps as integers
- Performance impact is minimal (only additional timezone conversions on timestamp columns)

## Testing Recommendations
1. Test that daily bars for the same symbol return identical timestamps when fetched via:
   - Direct table access (e.g., `timeframe='1d'` with small ticker list)
   - Aggregated path (e.g., `timeframe='1d'` with large ticker list or custom aggregation)
2. Verify that the most recent bar's timestamp matches the expected market close time in Eastern timezone
3. Confirm that extended hours filtering works correctly for intraday timeframes 