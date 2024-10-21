#!/bin/bash

echo "Running RedisTimeSeries OHLCV initialization..."

# Wait for Redis to be ready
until redis-cli ping | grep -q "PONG"; do
  echo "Waiting for Redis to be ready..."
  sleep 1
done

# Create raw time series for tick data (OHLCV)
echo "Creating raw_data time series with OHLCV..."
redis-cli TS.CREATE raw_ohlcv

# Preload historical OHLCV data (optional)
# Example: TS.ADD raw_ohlcv <timestamp> <open> <high> <low> <close> <volume>
# Add historical data
redis-cli TS.ADD raw_ohlcv 1697809200000 OPEN=100 HIGH=105 LOW=95 CLOSE=102 VOLUME=1000
redis-cli TS.ADD raw_ohlcv 1697809260000 OPEN=102 HIGH=106 LOW=96 CLOSE=103 VOLUME=1200

# Aggregation for OHLCV into different timeframes
echo "Creating aggregated time series for OHLCV..."

# Create the aggregated time series
redis-cli TS.CREATE 1s
redis-cli TS.CREATE 1m
redis-cli TS.CREATE 1h
redis-cli TS.CREATE 1d

redis-cli TS.CREATERULE raw_ohlcv agg_1m_ohlcv AGGREGATION FIRST 60000 LABELS OPEN
redis-cli TS.CREATERULE raw_ohlcv agg_1m_ohlcv AGGREGATION MAX 60000 LABELS HIGH
redis-cli TS.CREATERULE raw_ohlcv agg_1m_ohlcv AGGREGATION MIN 60000 LABELS LOW
redis-cli TS.CREATERULE raw_ohlcv agg_1m_ohlcv AGGREGATION LAST 60000 LABELS CLOSE
redis-cli TS.CREATERULE raw_ohlcv agg_1m_ohlcv AGGREGATION SUM 60000 LABELS VOLUME

redis-cli TS.CREATERULE raw_ohlcv agg_1h_ohlcv AGGREGATION FIRST 3600000 LABELS OPEN
redis-cli TS.CREATERULE raw_ohlcv agg_1h_ohlcv AGGREGATION MAX 3600000 LABELS HIGH
redis-cli TS.CREATERULE raw_ohlcv agg_1h_ohlcv AGGREGATION MIN 3600000 LABELS LOW
redis-cli TS.CREATERULE raw_ohlcv agg_1h_ohlcv AGGREGATION LAST 3600000 LABELS CLOSE
redis-cli TS.CREATERULE raw_ohlcv agg_1h_ohlcv AGGREGATION SUM 3600000 LABELS VOLUME

redis-cli TS.CREATERULE raw_ohlcv agg_1d_ohlcv AGGREGATION FIRST 86400000 LABELS OPEN
redis-cli TS.CREATERULE raw_ohlcv agg_1d_ohlcv AGGREGATION MAX 86400000 LABELS HIGH
redis-cli TS.CREATERULE raw_ohlcv agg_1d_ohlcv AGGREGATION MIN 86400000 LABELS LOW
redis-cli TS.CREATERULE raw_ohlcv agg_1d_ohlcv AGGREGATION LAST 86400000 LABELS CLOSE
redis-cli TS.CREATERULE raw_ohlcv agg_1d_ohlcv AGGREGATION SUM 86400000 LABELS VOLUME

echo "RedisTimeSeries initialized with OHLCV."
tail -f /dev/null

