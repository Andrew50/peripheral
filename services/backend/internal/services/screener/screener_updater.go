/*
GOAL: 10k ticker refreshed in 10s

bucket / metric	rows per bucket	how often queried	CA?	Rationale
Pre‑market daily stats (pre_market_stats)	330 rows / ticker	Every screen refresh	Yes — keep CA	One daily bucket, refresh touches ≤ 10 k new rows/minute, huge scan avoided
Intraday 1 m / 15 m / 1 h deltas & ranges	≤ 16 min of raw data	Only needed for current screen	No — keep intraday_stats table	Bucket would be 1 minute → every insert invalidates it, refresh ≈ query cost
Historical daily references (1 w–10 y, 52 w high/low)	1 row / day	Built once per day	No (plain table is fine)	Refresh cost tiny, CA would duplicate storage
Final screener view (one row per ticker)	1 row	Always queried	No	You already up‑sert; CA adds no value


OPTIMIZATION TO TRY:

-- parallelize the screener update with golang workers



OPTIMIZATION LOG: -- baseline 3s for 1 ticker

*/

package screener

import (
	"backend/internal/data"
	"context"
	"fmt"
	"log"
	"os"
	"time"
)

const (
	refreshInterval    = 60 * time.Second  // full screener top-off frequency (fallback)
	refreshTimeout     = 300 * time.Second // per-refresh SQL timeout (increased from 60s)
	extendedCloseHour  = 20                // 8 PM Eastern – hard stop
	maxTickersPerBatch = 10                // max tickers to process per batch (0 = no limit), increased from 1 for better efficiency
)

var createStaleQueueQuery = `
CREATE TABLE IF NOT EXISTS screener_stale (
    ticker text PRIMARY KEY,
    last_update_started timestamptz DEFAULT '1970-01-01',
    stale boolean DEFAULT TRUE
);

-- Drop the old poorly selective index
DROP INDEX IF EXISTS screener_stale_due_idx;

-- Create optimized partial index for stale tickers only, with timestamp first for better selectivity
-- This index only covers stale=TRUE rows and orders by timestamp first, making range scans efficient
CREATE INDEX IF NOT EXISTS screener_stale_timestamp_partial_idx
  ON screener_stale (last_update_started, ticker)
  WHERE stale = TRUE;

-- Additional index for fast lookups when updating stale status
CREATE INDEX IF NOT EXISTS screener_stale_ticker_stale_idx
  ON screener_stale (ticker, stale);
`

// Add SQL to insert initial stale tickers for securities where maxDate is null
var insertInitialStaleTickersQuery = `
INSERT INTO screener_stale (ticker)
SELECT DISTINCT ticker FROM securities
WHERE maxDate IS NULL
ON CONFLICT (ticker) DO UPDATE SET stale = TRUE, last_update_started = '1970-01-01';
`

var createPreMarketStatsQuery = `
CREATE MATERIALIZED VIEW IF NOT EXISTS pre_market_stats
WITH (timescaledb.continuous)
AS
SELECT
    ticker,
    -- Bucket by calendar day in Eastern Time, stored as UTC timestamps
    time_bucket('1 day', "timestamp", 'America/New_York') AS trade_day,
    /* prices & volumes */
    first(open / 1000.0,  "timestamp")                    AS pre_market_open,
    last (close / 1000.0, "timestamp")                    AS pre_market_close,
    max  (high / 1000.0)                                  AS pre_market_high,
    min  (low / 1000.0)                                   AS pre_market_low,
    sum  (volume)                                AS pre_market_volume,
    sum  (volume * close / 1000.0)                        AS pre_market_dollar_volume,
    /* derived metrics */
    (max(high / 1000.0) - min(low / 1000.0)) / NULLIF(min(low / 1000.0),0) * 100        AS pre_market_range_pct,
    last(close / 1000.0, "timestamp") - first(open / 1000.0, "timestamp")   AS pre_market_change,
    (last(close / 1000.0, "timestamp") - first(open / 1000.0, "timestamp"))
        / NULLIF(first(open / 1000.0, "timestamp"),0) * 100          AS pre_market_change_pct
FROM ohlcv_1m
WHERE
    ("timestamp" AT TIME ZONE 'America/New_York')::time
        BETWEEN TIME '04:00' AND TIME '09:29:59'
GROUP BY ticker, trade_day
WITH NO DATA;

-- 2️⃣  Index so REFRESH + queries stay fast
CREATE INDEX IF NOT EXISTS pre_market_stats_ticker_day_idx
         ON pre_market_stats (ticker, trade_day);

-- 3️⃣  Keep it fresh automatically (run every 5 min, back-fills 7 days)
DO $$
BEGIN
    PERFORM add_continuous_aggregate_policy('pre_market_stats',
        start_offset => INTERVAL '7 days',
        end_offset => INTERVAL '0',
        schedule_interval => INTERVAL '5 minutes');
EXCEPTION
    WHEN OTHERS THEN
        RAISE NOTICE 'Policy for pre_market_stats may already exist: %', SQLERRM;
END $$;
`

var createOHLCVIndexesQuery = `
-- Helpful covering indexes on partitioned source tables, these are created by the migration 68.sql script and in ohlcv table post load setup, but also defined here to make sure they are active and available for the screener updater
CREATE INDEX IF NOT EXISTS ohlcv_1d_ticker_ts_desc_inc
        ON ohlcv_1d (ticker, "timestamp" DESC)
        INCLUDE (open, high, low, close, volume);

CREATE INDEX IF NOT EXISTS ohlcv_1m_ticker_ts_desc_inc
        ON ohlcv_1m (ticker, "timestamp" DESC)
        INCLUDE (open, high, low, close, volume);

`

var intradayPriceRefsQuery = `
-- Create a normal table to store only the most recent intraday stats per ticker
-- This replaces the problematic materialized view that was causing database crashes
CREATE TABLE IF NOT EXISTS intraday_stats (
    ticker text PRIMARY KEY,
    ts timestamptz NOT NULL,
    change_1h_pct numeric,
    change_4h_pct numeric,
    range_1m_pct numeric,
    range_15m_pct numeric,
    range_1h_pct numeric,
    avg_dollar_volume_1m_14 numeric,
    avg_volume_1m_14 numeric,
    relative_volume_14 numeric,
    extended_hours_change numeric,
    extended_hours_change_pct numeric,
    updated_at timestamptz DEFAULT now()
);

-- Index for efficient lookups
CREATE INDEX IF NOT EXISTS intraday_stats_ticker_idx ON intraday_stats (ticker);
CREATE INDEX IF NOT EXISTS intraday_stats_ts_idx ON intraday_stats (ts DESC);

-- Function to update intraday stats for specific tickers
-- This computes the metrics using batch processing over the entire p_tickers array
CREATE OR REPLACE FUNCTION update_intraday_stats(p_tickers text[])
RETURNS void
LANGUAGE plpgsql AS $$
DECLARE
    v_now_utc timestamptz := now();
BEGIN
    -- Safety exit
    IF array_length(p_tickers, 1) IS NULL THEN
        RETURN;
    END IF;

    -- Process all tickers in a single batch operation
    INSERT INTO intraday_stats (
        ticker, ts, change_1h_pct, change_4h_pct, range_1m_pct, range_15m_pct, range_1h_pct,
        avg_dollar_volume_1m_14, avg_volume_1m_14, relative_volume_14,
        extended_hours_change, extended_hours_change_pct, updated_at
    )
    SELECT 
        latest.ticker,
        latest.ts,
        -- 1-hour change
        CASE 
            WHEN hour_ago.close IS NOT NULL THEN 
                100.0 * (latest.close - hour_ago.close) / NULLIF(hour_ago.close, 0)
            ELSE NULL
        END AS change_1h_pct,
        -- 4-hour change
        CASE 
            WHEN four_hour_ago.close IS NOT NULL THEN 
                100.0 * (latest.close - four_hour_ago.close) / NULLIF(four_hour_ago.close, 0)
            ELSE NULL
        END AS change_4h_pct,
        -- 1-minute range
        100.0 * (latest.high - latest.low) / NULLIF(latest.low, 0) AS range_1m_pct,
        -- 15-minute range
        CASE 
            WHEN ranges_15m.high_15m IS NOT NULL AND ranges_15m.low_15m IS NOT NULL THEN
                100.0 * (ranges_15m.high_15m - ranges_15m.low_15m) / NULLIF(ranges_15m.low_15m, 0)
            ELSE NULL
        END AS range_15m_pct,
        -- 1-hour range
        CASE 
            WHEN ranges_1h.high_1h IS NOT NULL AND ranges_1h.low_1h IS NOT NULL THEN
                100.0 * (ranges_1h.high_1h - ranges_1h.low_1h) / NULLIF(ranges_1h.low_1h, 0)
            ELSE NULL
        END AS range_1h_pct,
        -- Average dollar volume (14-period)
        vol_14.avg_dollar_volume_1m_14,
        -- Average volume (14-period)
        vol_14.avg_volume_1m_14,
        -- Relative volume
        CASE 
            WHEN vol_14.avg_volume_1m_14 IS NOT NULL AND vol_14.avg_volume_1m_14 > 0 THEN
                latest.volume / vol_14.avg_volume_1m_14
            ELSE NULL
        END AS relative_volume_14,
        -- Extended hours change (simplified - just vs. 4 PM close)
        CASE 
            WHEN (latest.ts AT TIME ZONE 'America/New_York')::time BETWEEN TIME '16:00' AND TIME '20:00'
                 AND market_close.close IS NOT NULL THEN
                latest.close - market_close.close
            ELSE NULL
        END AS extended_hours_change,
        -- Extended hours change percentage
        CASE 
            WHEN (latest.ts AT TIME ZONE 'America/New_York')::time BETWEEN TIME '16:00' AND TIME '20:00'
                 AND market_close.close IS NOT NULL THEN
                100.0 * (latest.close - market_close.close) / NULLIF(market_close.close, 0)
            ELSE NULL
        END AS extended_hours_change_pct,
        v_now_utc AS updated_at
    FROM (
        -- Get the most recent 1-minute data for all tickers
        SELECT DISTINCT ON (ticker)
            ticker,
            time_bucket('1 minute', "timestamp") AS ts,
            last(close / 1000.0, "timestamp") AS close,
            last(high / 1000.0, "timestamp") AS high,
            last(low / 1000.0, "timestamp") AS low,
            last(volume, "timestamp") AS volume
        FROM ohlcv_1m
        WHERE ticker = ANY(p_tickers)
          AND "timestamp" >= v_now_utc - INTERVAL '5 minutes'
        GROUP BY ticker, time_bucket('1 minute', "timestamp")
        ORDER BY ticker, time_bucket('1 minute', "timestamp") DESC
    ) latest
    LEFT JOIN (
        -- Get close price from 1 hour ago for all tickers
        SELECT DISTINCT ON (ticker)
            ticker,
            close / 1000.0 AS close
        FROM ohlcv_1m
        WHERE ticker = ANY(p_tickers)
          AND "timestamp" <= v_now_utc - INTERVAL '1 hour'
          AND "timestamp" >= v_now_utc - INTERVAL '1 hour 5 minutes'
        ORDER BY ticker, "timestamp" DESC
    ) hour_ago ON hour_ago.ticker = latest.ticker
    LEFT JOIN (
        -- Get close price from 4 hours ago for all tickers
        SELECT DISTINCT ON (ticker)
            ticker,
            close / 1000.0 AS close
        FROM ohlcv_1m
        WHERE ticker = ANY(p_tickers)
          AND "timestamp" <= v_now_utc - INTERVAL '4 hours'
          AND "timestamp" >= v_now_utc - INTERVAL '4 hours 5 minutes'
        ORDER BY ticker, "timestamp" DESC
    ) four_hour_ago ON four_hour_ago.ticker = latest.ticker
    LEFT JOIN (
        -- Get 15-minute high/low range for all tickers
        SELECT 
            ticker,
            MAX(high / 1000.0) AS high_15m,
            MIN(low / 1000.0) AS low_15m
        FROM ohlcv_1m
        WHERE ticker = ANY(p_tickers)
          AND "timestamp" >= v_now_utc - INTERVAL '15 minutes'
          AND "timestamp" <= v_now_utc
        GROUP BY ticker
    ) ranges_15m ON ranges_15m.ticker = latest.ticker
    LEFT JOIN (
        -- Get 1-hour high/low range for all tickers
        SELECT 
            ticker,
            MAX(high / 1000.0) AS high_1h,
            MIN(low / 1000.0) AS low_1h
        FROM ohlcv_1m
        WHERE ticker = ANY(p_tickers)
          AND "timestamp" >= v_now_utc - INTERVAL '1 hour'
          AND "timestamp" <= v_now_utc
        GROUP BY ticker
    ) ranges_1h ON ranges_1h.ticker = latest.ticker
    LEFT JOIN (
        -- Get 14-period volume averages for all tickers
        SELECT 
            ticker,
            AVG(volume * close / 1000.0) AS avg_dollar_volume_1m_14,
            AVG(volume) AS avg_volume_1m_14
        FROM (
            SELECT 
                ticker,
                last(volume, "timestamp") AS volume,
                last(close / 1000.0, "timestamp") AS close
            FROM ohlcv_1m
            WHERE ticker = ANY(p_tickers)
              AND "timestamp" >= v_now_utc - INTERVAL '14 minutes'
              AND "timestamp" <= v_now_utc
            GROUP BY ticker, time_bucket('1 minute', "timestamp")
        ) recent_volume
        GROUP BY ticker
    ) vol_14 ON vol_14.ticker = latest.ticker
    LEFT JOIN (
        -- Get market close price (4 PM ET) for extended hours calculation for all tickers
        SELECT DISTINCT ON (ticker)
            ticker,
            close / 1000.0 AS close
        FROM ohlcv_1m
        WHERE ticker = ANY(p_tickers)
          AND ("timestamp" AT TIME ZONE 'America/New_York')::time = TIME '16:00'
          AND "timestamp"::date = (v_now_utc AT TIME ZONE 'America/New_York')::date
        ORDER BY ticker, "timestamp" DESC
    ) market_close ON market_close.ticker = latest.ticker
    
    ON CONFLICT (ticker) DO UPDATE SET
        ts = EXCLUDED.ts,
        change_1h_pct = EXCLUDED.change_1h_pct,
        change_4h_pct = EXCLUDED.change_4h_pct,
        range_1m_pct = EXCLUDED.range_1m_pct,
        range_15m_pct = EXCLUDED.range_15m_pct,
        range_1h_pct = EXCLUDED.range_1h_pct,
        avg_dollar_volume_1m_14 = EXCLUDED.avg_dollar_volume_1m_14,
        avg_volume_1m_14 = EXCLUDED.avg_volume_1m_14,
        relative_volume_14 = EXCLUDED.relative_volume_14,
        extended_hours_change = EXCLUDED.extended_hours_change,
        extended_hours_change_pct = EXCLUDED.extended_hours_change_pct,
        updated_at = EXCLUDED.updated_at;
END;
$$;
`

var createHistoricalPriceRefsQuery = `
-- Create a simplified continuous aggregate that complies with TimescaleDB restrictions
-- This version removes correlated subqueries, window functions, and complex nested queries
CREATE MATERIALIZED VIEW IF NOT EXISTS historical_price_refs
WITH (timescaledb.continuous)
AS
SELECT 
    time_bucket('1 day', "timestamp") AS bucket,
    ticker,
    -- Current price data (last values in the bucket)
    last(close / 1000.0, "timestamp") AS current_close,
    first(open / 1000.0, "timestamp") AS daily_open,
    max(high / 1000.0) AS daily_high,
    min(low / 1000.0) AS daily_low,
    sum(volume) AS daily_volume,
    
    -- Basic price statistics for the day
    avg(close / 1000.0) AS avg_close,
    stddev_samp(close / 1000.0) AS daily_volatility,
    
    -- Count of trading periods
    count(*) AS trading_periods
FROM ohlcv_1d
WHERE "timestamp" >= now() - INTERVAL '400 days'  -- Limit to recent data for performance
GROUP BY time_bucket('1 day', "timestamp"), ticker
WITH NO DATA;

-- Create efficient indexes for the continuous aggregate
CREATE INDEX IF NOT EXISTS historical_price_refs_ticker_bucket_idx 
    ON historical_price_refs (ticker, bucket DESC);
CREATE INDEX IF NOT EXISTS historical_price_refs_bucket_idx 
    ON historical_price_refs (bucket DESC);

-- Create a separate regular table for complex historical calculations
-- This table will store pre-computed historical references updated periodically
CREATE TABLE IF NOT EXISTS historical_price_calculations (
    ticker text PRIMARY KEY,
    current_close numeric,
    
    -- Time-based price references
    price_1w numeric,
    price_1m numeric,
    price_3m numeric,
    price_6m numeric,
    price_1y numeric,
    price_5y numeric,
    price_10y numeric,
    price_ytd numeric,
    price_all numeric,
    
    -- 52-week extremes
    price_52w_low numeric,
    price_52w_high numeric,
    
    -- Moving averages
    sma_50 numeric,
    sma_200 numeric,
    
    -- RSI (computed separately)
    rsi_14 numeric,
    
    -- Volatility measures
    stddev_7 numeric,
    stddev_30 numeric,
    
    -- Metadata
    updated_at timestamptz DEFAULT now(),
    calculation_date date DEFAULT CURRENT_DATE
);

-- Index for efficient lookups
CREATE INDEX IF NOT EXISTS historical_price_calculations_ticker_idx 
    ON historical_price_calculations (ticker);
CREATE INDEX IF NOT EXISTS historical_price_calculations_updated_idx 
    ON historical_price_calculations (updated_at DESC);

-- Function to update historical price calculations for specific tickers
-- This processes all tickers concurrently in a single batch operation for maximum performance
CREATE OR REPLACE FUNCTION update_historical_price_calculations(p_tickers text[])
RETURNS void
LANGUAGE plpgsql AS $$
DECLARE
    v_current_date date := CURRENT_DATE;
BEGIN
    -- Safety exit
    IF array_length(p_tickers, 1) IS NULL THEN
        RETURN;
    END IF;

    -- Process all tickers concurrently in a single batch operation
    INSERT INTO historical_price_calculations (
        ticker, current_close, price_1w, price_1m, price_3m, price_6m, price_1y,
        price_5y, price_10y, price_ytd, price_all, price_52w_low, price_52w_high,
        sma_50, sma_200, rsi_14, stddev_7, stddev_30, updated_at, calculation_date
    )
    SELECT 
        ticker_data.ticker,
        current_data.close,
        
        -- Historical price references using efficient batch processing
        week_data.close AS price_1w,
        month_data.close AS price_1m,
        quarter_data.close AS price_3m,
        half_year_data.close AS price_6m,
        year_data.close AS price_1y,
        five_year_data.close AS price_5y,
        ten_year_data.close AS price_10y,
        ytd_data.close AS price_ytd,
        all_time_data.close AS price_all,
        
        -- 52-week extremes
        extremes.low_52w,
        extremes.high_52w,
        
        -- Moving averages
        sma_50_data.avg_close,
        sma_200_data.avg_close,
        
        -- RSI (simplified calculation without window functions)
        CASE 
            WHEN rsi_data.avg_loss = 0 THEN 100
            WHEN rsi_data.avg_loss IS NULL THEN NULL
            ELSE 100 - 100 / (1 + rsi_data.avg_gain / rsi_data.avg_loss)
        END AS rsi_14,
        
        -- Volatility measures
        vol_data.stddev_7,
        vol_data.stddev_30,
        
        now() AS updated_at,
        v_current_date AS calculation_date
    FROM (
        -- Generate a row for each ticker in the input array
        SELECT unnest(p_tickers) AS ticker
    ) ticker_data
    LEFT JOIN LATERAL (
        -- Get current close price for each ticker
        SELECT close / 1000.0 AS close
        FROM ohlcv_1d
        WHERE ticker = ticker_data.ticker
        ORDER BY "timestamp" DESC
        LIMIT 1
    ) current_data ON TRUE
    LEFT JOIN LATERAL (
        -- 1 week ago
        SELECT close / 1000.0 AS close
        FROM ohlcv_1d
        WHERE ticker = ticker_data.ticker
          AND "timestamp" <= now() - INTERVAL '7 days'
        ORDER BY "timestamp" DESC
        LIMIT 1
    ) week_data ON TRUE
    LEFT JOIN LATERAL (
        -- 1 month ago
        SELECT close / 1000.0 AS close
        FROM ohlcv_1d
        WHERE ticker = ticker_data.ticker
          AND "timestamp" <= now() - INTERVAL '1 month'
        ORDER BY "timestamp" DESC
        LIMIT 1
    ) month_data ON TRUE
    LEFT JOIN LATERAL (
        -- 3 months ago
        SELECT close / 1000.0 AS close
        FROM ohlcv_1d
        WHERE ticker = ticker_data.ticker
          AND "timestamp" <= now() - INTERVAL '3 months'
        ORDER BY "timestamp" DESC
        LIMIT 1
    ) quarter_data ON TRUE
    LEFT JOIN LATERAL (
        -- 6 months ago
        SELECT close / 1000.0 AS close
        FROM ohlcv_1d
        WHERE ticker = ticker_data.ticker
          AND "timestamp" <= now() - INTERVAL '6 months'
        ORDER BY "timestamp" DESC
        LIMIT 1
    ) half_year_data ON TRUE
    LEFT JOIN LATERAL (
        -- 1 year ago
        SELECT close / 1000.0 AS close
        FROM ohlcv_1d
        WHERE ticker = ticker_data.ticker
          AND "timestamp" <= now() - INTERVAL '1 year'
        ORDER BY "timestamp" DESC
        LIMIT 1
    ) year_data ON TRUE
    LEFT JOIN LATERAL (
        -- 5 years ago
        SELECT close / 1000.0 AS close
        FROM ohlcv_1d
        WHERE ticker = ticker_data.ticker
          AND "timestamp" <= now() - INTERVAL '5 years'
        ORDER BY "timestamp" DESC
        LIMIT 1
    ) five_year_data ON TRUE
    LEFT JOIN LATERAL (
        -- 10 years ago
        SELECT close / 1000.0 AS close
        FROM ohlcv_1d
        WHERE ticker = ticker_data.ticker
          AND "timestamp" <= now() - INTERVAL '10 years'
        ORDER BY "timestamp" DESC
        LIMIT 1
    ) ten_year_data ON TRUE
    LEFT JOIN LATERAL (
        -- Year-to-date
        SELECT close / 1000.0 AS close
        FROM ohlcv_1d
        WHERE ticker = ticker_data.ticker
          AND EXTRACT(YEAR FROM "timestamp") = EXTRACT(YEAR FROM now())
        ORDER BY "timestamp" ASC
        LIMIT 1
    ) ytd_data ON TRUE
    LEFT JOIN LATERAL (
        -- All-time (earliest available)
        SELECT close / 1000.0 AS close
        FROM ohlcv_1d
        WHERE ticker = ticker_data.ticker
        ORDER BY "timestamp" ASC
        LIMIT 1
    ) all_time_data ON TRUE
    LEFT JOIN LATERAL (
        -- 52-week extremes
        SELECT 
            MIN(low / 1000.0) AS low_52w,
            MAX(high / 1000.0) AS high_52w
        FROM ohlcv_1d
        WHERE ticker = ticker_data.ticker
          AND "timestamp" >= now() - INTERVAL '52 weeks'
    ) extremes ON TRUE
    LEFT JOIN LATERAL (
        -- 50-day moving average
        SELECT AVG(close / 1000.0) AS avg_close
        FROM ohlcv_1d
        WHERE ticker = ticker_data.ticker
          AND "timestamp" >= now() - INTERVAL '50 days'
    ) sma_50_data ON TRUE
    LEFT JOIN LATERAL (
        -- 200-day moving average
        SELECT AVG(close / 1000.0) AS avg_close
        FROM ohlcv_1d
        WHERE ticker = ticker_data.ticker
          AND "timestamp" >= now() - INTERVAL '200 days'
    ) sma_200_data ON TRUE
    LEFT JOIN LATERAL (
        -- RSI calculation simplified - will be computed separately if needed
        -- For now, return NULL to avoid window function issues
        SELECT 
            NULL::numeric AS avg_gain,
            NULL::numeric AS avg_loss
    ) rsi_data ON TRUE
    LEFT JOIN LATERAL (
        -- Volatility measures
        SELECT 
            STDDEV_SAMP(close / 1000.0) FILTER (WHERE "timestamp" >= now() - INTERVAL '7 days') AS stddev_7,
            STDDEV_SAMP(close / 1000.0) FILTER (WHERE "timestamp" >= now() - INTERVAL '30 days') AS stddev_30
        FROM ohlcv_1d
        WHERE ticker = ticker_data.ticker
          AND "timestamp" >= now() - INTERVAL '30 days'
    ) vol_data ON TRUE
    
    ON CONFLICT (ticker) DO UPDATE SET
        current_close = EXCLUDED.current_close,
        price_1w = EXCLUDED.price_1w,
        price_1m = EXCLUDED.price_1m,
        price_3m = EXCLUDED.price_3m,
        price_6m = EXCLUDED.price_6m,
        price_1y = EXCLUDED.price_1y,
        price_5y = EXCLUDED.price_5y,
        price_10y = EXCLUDED.price_10y,
        price_ytd = EXCLUDED.price_ytd,
        price_all = EXCLUDED.price_all,
        price_52w_low = EXCLUDED.price_52w_low,
        price_52w_high = EXCLUDED.price_52w_high,
        sma_50 = EXCLUDED.sma_50,
        sma_200 = EXCLUDED.sma_200,
        rsi_14 = EXCLUDED.rsi_14,
        stddev_7 = EXCLUDED.stddev_7,
        stddev_30 = EXCLUDED.stddev_30,
        updated_at = EXCLUDED.updated_at,
        calculation_date = EXCLUDED.calculation_date;
END;
$$;

-- Add continuous aggregate policy for the simplified view
DO $$
BEGIN
    PERFORM add_continuous_aggregate_policy('historical_price_refs',
        start_offset => INTERVAL '3 days',
        end_offset => INTERVAL '1 hour',
        schedule_interval => INTERVAL '1 hour');
EXCEPTION
    WHEN OTHERS THEN
        RAISE NOTICE 'Policy for historical_price_refs may already exist: %', SQLERRM;
END $$;
`

var incrementalRefreshQuery = `
-- Incremental screener refresh for stale tickers only
-- This function processes only the tickers that have changed since last refresh
-- NOTE: This function depends on intraday_stats table having the correct structure with updated_at column
-- 
-- IMPORTANT: If you get "column does not exist" errors, ensure that:
-- 1. The intraday_stats table exists with the structure defined in intradayPriceRefsQuery
-- 2. The historical_price_calculations table exists with the structure defined in createHistoricalPriceRefsQuery
-- 3. All required functions (update_intraday_stats, update_historical_price_calculations, pct) exist
--
-- To recreate the tables, run the intradayPriceRefsQuery and createHistoricalPriceRefsQuery

CREATE OR REPLACE FUNCTION public.refresh_screener_delta(
    p_tickers text[]
) RETURNS void
LANGUAGE plpgsql AS $$
DECLARE
    v_now_utc  timestamptz := now();
    v_trade_day date        := (v_now_utc AT TIME ZONE 'America/New_York')::date;
BEGIN
    ------------------------------------------------------------------
    -- Safety exit
    ------------------------------------------------------------------
    IF array_length(p_tickers, 1) IS NULL THEN
        RETURN;
    END IF;

    ------------------------------------------------------------------
    -- Update intraday stats and historical calculations for the specified tickers first
    -- NOTE: If this fails with "column does not exist" error, the intraday_stats table
    -- needs to be recreated with the correct structure from intradayPriceRefsQuery
    ------------------------------------------------------------------
    PERFORM update_intraday_stats(p_tickers);
    PERFORM update_historical_price_calculations(p_tickers);

    ------------------------------------------------------------------
    -- Up‑sert screener rows built from securities + latest snapshots
    -- FIX: Use DISTINCT ON to ensure only one row per ticker is processed
    ------------------------------------------------------------------
    INSERT INTO screener (
        ticker,                -- PK
        calc_time,
        security_id,
        market_cap,
        sector,
        industry,
        /* ───── pre‑market aggregates ───── */
        pre_market_range_pct,
        pre_market_change,
        pre_market_change_pct,
        pre_market_dollar_volume,
        pre_market_open,
        pre_market_high,
        pre_market_low,
        pre_market_close,
        pre_market_volume,
        /* ───── latest daily bar ───── */
        "open", "high", "low", "close", volume, dollar_volume,
        /* ───── moving‑average context ───── */
        price_over_50dma, price_over_200dma,
        /* ───── horizon % changes ───── */
        change_1d_pct, change_1w_pct, change_1m_pct, change_3m_pct,
        change_6m_pct, change_ytd_1y_pct, change_5y_pct, change_10y_pct,
        change_all_time_pct,
        /* ───── volatility & extremes ───── */
        volatility_1w, volatility_1m,
        price_over_52wk_high, price_over_52wk_low,
        wk52_low, wk52_high,
        /* ───── technicals ───── */
        rsi, dma_200, dma_50,
        /* ───── intra‑day ranges & deltas ───── */
        day_range_pct,
        change_from_open, change_from_open_pct,
        change_1_pct, change_15_pct, change_1h_pct, change_4h_pct,
        avg_dollar_volume_1m,
        range_1m_pct, range_15m_pct, range_1h_pct
    )
    SELECT DISTINCT ON (sec.ticker)
        /* ——— securities ——— */
        sec.ticker,
        v_now_utc                                        AS calc_time,
        sec.securityid                                   AS security_id,
        sec.market_cap, sec.sector, sec.industry,

        /* ——— pre‑market ——— */
        pm.pre_market_range_pct,
        pm.pre_market_change,
        pm.pre_market_change_pct,
        pm.pre_market_dollar_volume,
        pm.pre_market_open,
        pm.pre_market_high,
        pm.pre_market_low,
        pm.pre_market_close,
        pm.pre_market_volume,

        /* ——— daily bar ——— */
        d.open, d.high, d.low, d.close,
        d.volume,
        d.close * d.volume                               AS dollar_volume,

        /* ——— MA context ——— */
        NULLIF(d.close,0) / NULLIF(hp.sma_50 ,0) * 100   AS price_over_50dma,
        NULLIF(d.close,0) / NULLIF(hp.sma_200,0) * 100   AS price_over_200dma,

        /* ——— Δ vs historic closes ——— */
        (d.close - pc.prev_close) / NULLIF(pc.prev_close, 0) * 100,
        (d.close - hp.price_1w) / NULLIF(hp.price_1w, 0) * 100,
        (d.close - hp.price_1m) / NULLIF(hp.price_1m, 0) * 100,
        (d.close - hp.price_3m) / NULLIF(hp.price_3m, 0) * 100,
        (d.close - hp.price_6m) / NULLIF(hp.price_6m, 0) * 100,
        (d.close - hp.price_ytd) / NULLIF(hp.price_ytd, 0) * 100,
        (d.close - hp.price_5y) / NULLIF(hp.price_5y, 0) * 100,
        (d.close - hp.price_10y) / NULLIF(hp.price_10y, 0) * 100,
        (d.close - hp.price_all) / NULLIF(hp.price_all, 0) * 100,

        /* ——— vols & extremes ——— */
        hp.stddev_7                                      AS volatility_1w,
        hp.stddev_30                                     AS volatility_1m,
        NULLIF(d.close,0) / NULLIF(hp.price_52w_high,0) * 100 AS price_over_52wk_high,
        NULLIF(d.close,0) / NULLIF(hp.price_52w_low ,0) * 100 AS price_over_52wk_low,
        hp.price_52w_low                                 AS wk52_low,
        hp.price_52w_high                                AS wk52_high,

        /* ——— technicals ——— */
        hp.rsi_14                                        AS rsi,
        hp.sma_200                                       AS dma_200,
        hp.sma_50                                        AS dma_50,

        /* ——— intraday context ——— */
        rng.day_range_pct,
        d.close - d.open                                 AS change_from_open,
        (d.close - d.open) / NULLIF(d.open, 0) * 100,
        delt.change_1_pct,
        delt.change_15_pct,
        ist.change_1h_pct,
        ist.change_4h_pct,
        ist.avg_dollar_volume_1m_14                      AS avg_dollar_volume_1m,
        ist.range_1m_pct,
        ist.range_15m_pct,
        ist.range_1h_pct
    FROM   securities                     sec

    /* ── latest daily bar (covers open/high/low/close/volume) ── */
    JOIN LATERAL (
        SELECT open / 1000.0 AS open, high / 1000.0 AS high, low / 1000.0 AS low, close / 1000.0 AS close, volume
        FROM   ohlcv_1d
        WHERE  ticker = sec.ticker
        ORDER  BY "timestamp" DESC
        LIMIT 1
    ) d ON TRUE

    /* ── previous day close (for 1‑day Δ) ── */
    LEFT JOIN LATERAL (
        SELECT close / 1000.0 AS prev_close
        FROM   ohlcv_1d
        WHERE  ticker = sec.ticker
        ORDER  BY "timestamp" DESC
        OFFSET 1 LIMIT 1
    ) pc ON TRUE

    /* ── pre‑market snapshot ── */
    LEFT JOIN pre_market_stats            pm
           ON pm.ticker     = sec.ticker
          AND pm.trade_day  = v_trade_day

    /* ── latest intraday snapshot ── */
    LEFT JOIN LATERAL (
        SELECT 
            ticker,
            ts,
            change_1h_pct,
            change_4h_pct,
            range_1m_pct,
            range_15m_pct,
            range_1h_pct,
            avg_dollar_volume_1m_14,
            avg_volume_1m_14,
            relative_volume_14,
            extended_hours_change,
            extended_hours_change_pct
        FROM   intraday_stats
        WHERE  ticker = sec.ticker
        ORDER  BY ts DESC
        LIMIT 1
    ) ist ON TRUE

    /* ── 1‑min & 15‑min Δ (merged single scan) ── */
    LEFT JOIN LATERAL (
        SELECT 
            (d.close - close_1m_ago) / NULLIF(close_1m_ago, 0) * 100  AS change_1_pct,
            (d.close - close_15m_ago) / NULLIF(close_15m_ago, 0) * 100 AS change_15_pct
        FROM   (
            SELECT 
                MAX(CASE WHEN "timestamp" <= v_now_utc - INTERVAL '1 minute' THEN close / 1000.0 END) AS close_1m_ago,
                MAX(CASE WHEN "timestamp" <= v_now_utc - INTERVAL '15 minutes' THEN close / 1000.0 END) AS close_15m_ago
            FROM   ohlcv_1m
            WHERE  ticker = sec.ticker
              AND  "timestamp" <= v_now_utc - INTERVAL '1 minute'
              AND  "timestamp" >= v_now_utc - INTERVAL '16 minutes'
        ) delta_calc
    ) delt ON TRUE

    /* ── historic refs & technicals ── */
    LEFT JOIN historical_price_calculations      hp  ON hp.ticker = sec.ticker

    /* ── intra‑day day‑range % (high‑low vs low) ── */
    LEFT JOIN LATERAL (
        SELECT CASE WHEN d.low = 0 THEN NULL ELSE (d.high - d.low)/d.low*100 END AS day_range_pct
    ) rng ON TRUE

    WHERE  sec.ticker = ANY(p_tickers)
      AND  sec.active = TRUE
      AND  sec.maxDate IS NULL  -- Only get currently active securities
    ORDER BY sec.ticker, sec.securityid DESC  -- Order by securityid DESC to get the most recent one first
    ON CONFLICT (ticker) DO UPDATE SET
        calc_time               = EXCLUDED.calc_time,
        market_cap              = EXCLUDED.market_cap,
        sector                  = EXCLUDED.sector,
        industry                = EXCLUDED.industry,
        pre_market_range_pct    = EXCLUDED.pre_market_range_pct,
        pre_market_change       = EXCLUDED.pre_market_change,
        pre_market_change_pct   = EXCLUDED.pre_market_change_pct,
        pre_market_dollar_volume= EXCLUDED.pre_market_dollar_volume,
        pre_market_open         = EXCLUDED.pre_market_open,
        pre_market_high         = EXCLUDED.pre_market_high,
        pre_market_low          = EXCLUDED.pre_market_low,
        pre_market_close        = EXCLUDED.pre_market_close,
        pre_market_volume       = EXCLUDED.pre_market_volume,
        "open"                 = EXCLUDED."open",
        "high"                 = EXCLUDED."high",
        "low"                  = EXCLUDED."low",
        "close"                = EXCLUDED."close",
        volume                  = EXCLUDED.volume,
        dollar_volume           = EXCLUDED.dollar_volume,
        price_over_50dma        = EXCLUDED.price_over_50dma,
        price_over_200dma       = EXCLUDED.price_over_200dma,
        change_1d_pct           = EXCLUDED.change_1d_pct,
        change_1w_pct           = EXCLUDED.change_1w_pct,
        change_1m_pct           = EXCLUDED.change_1m_pct,
        change_3m_pct           = EXCLUDED.change_3m_pct,
        change_6m_pct           = EXCLUDED.change_6m_pct,
        change_ytd_1y_pct       = EXCLUDED.change_ytd_1y_pct,
        change_5y_pct           = EXCLUDED.change_5y_pct,
        change_10y_pct          = EXCLUDED.change_10y_pct,
        change_all_time_pct     = EXCLUDED.change_all_time_pct,
        volatility_1w           = EXCLUDED.volatility_1w,
        volatility_1m           = EXCLUDED.volatility_1m,
        price_over_52wk_high    = EXCLUDED.price_over_52wk_high,
        price_over_52wk_low     = EXCLUDED.price_over_52wk_low,
        wk52_low                = EXCLUDED.wk52_low,
        wk52_high               = EXCLUDED.wk52_high,
        rsi                     = EXCLUDED.rsi,
        dma_200                 = EXCLUDED.dma_200,
        dma_50                  = EXCLUDED.dma_50,
        day_range_pct           = EXCLUDED.day_range_pct,
        change_from_open        = EXCLUDED.change_from_open,
        change_from_open_pct    = EXCLUDED.change_from_open_pct,
        change_1_pct            = EXCLUDED.change_1_pct,
        change_15_pct           = EXCLUDED.change_15_pct,
        change_1h_pct           = EXCLUDED.change_1h_pct,
        change_4h_pct           = EXCLUDED.change_4h_pct,
        avg_dollar_volume_1m    = EXCLUDED.avg_dollar_volume_1m,
        range_1m_pct            = EXCLUDED.range_1m_pct,
        range_15m_pct           = EXCLUDED.range_15m_pct,
        range_1h_pct            = EXCLUDED.range_1h_pct;
END;
$$;


`

func StartScreenerUpdaterLoop(conn *data.Conn) error {

	/*loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		log.Fatalf("❌ cannot load ET timezone: %v", err)
	}*/

	// Optimize database connection settings for better performance
	if err := optimizeDatabaseConnection(conn); err != nil {
		log.Printf("⚠️  Failed to optimize database connection: %v", err)
	}

	err := runScreenerLoopInit(conn)
	if err != nil {
		return fmt.Errorf("failed to setup incremental infrastructure: %v", err)
	}

	// Track startup time
	startTime := time.Now()
	log.Printf("🚀 Screener updater started at %s", startTime.Format("2006-01-02 15:04:05"))

	updateStaleScreenerValues(conn)

	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	// Add counters for monitoring
	var updateCount int
	var totalDuration time.Duration

	for {
		//now := time.Now().In(loc)
		/*if now.Hour() >= extendedCloseHour {
			log.Println("🌙 Post‑market closed — stopping incremental screener updater")
			return nil
		}*/

		select {
		case <-ticker.C:
			updateStart := time.Now()
			updateStaleScreenerValues(conn)
			updateDuration := time.Since(updateStart)

			updateCount++
			totalDuration += updateDuration

			if updateCount%10 == 0 {
				avgDuration := totalDuration / time.Duration(updateCount)
				log.Printf("📊 Screener update stats: %d updates, avg duration: %v", updateCount, avgDuration)
			}
		}
	}
}

// optimizeDatabaseConnection applies performance optimizations to the database connection
func optimizeDatabaseConnection(conn *data.Conn) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Apply connection-level optimizations
	optimizations := []string{
		"SET statement_timeout = '300s'", // Match our refresh timeout
		/* effective_cache_size = '4GB'",   // Adjust based on your system
		"SET random_page_cost = 1.1",         // Optimize for SSD storage
		"SET seq_page_cost = 1.0",            // Optimize for SSD storage
		"SET cpu_tuple_cost = 0.01",          // Optimize for modern CPUs
		"SET cpu_index_tuple_cost = 0.005",   // Optimize for modern CPUs
		"SET cpu_operator_cost = 0.0025",     // Optimize for modern CPUs
		"SET effective_io_concurrency = 200", // Optimize for SSD
		"SET synchronous_commit = off",       // Improve write performance (careful with this)
		"SET checkpoint_completion_target = 0.9",
		"SET wal_buffers = '16MB'",
		"SET shared_preload_libraries = 'pg_stat_statements'",*/
	}

	successCount := 0
	for _, opt := range optimizations {
		if _, err := conn.DB.Exec(ctx, opt); err != nil {
			// Some settings might not be changeable at runtime - that's OK
			continue
		}
		successCount++
	}

	// Add this after the loop
	var currentWorkMem string
	err := conn.DB.QueryRow(context.Background(), "SHOW work_mem;").Scan(&currentWorkMem)
	if err != nil {
		log.Printf("Failed to check work_mem: %v", err)
	} else {
		log.Printf("Effective work_mem in session: %s", currentWorkMem)
	}

	log.Printf("✅ Applied %d/%d database optimizations", successCount, len(optimizations))
	return nil
}

// SQL queries for reuse (avoid re-parsing)
var (
	updateAndRefreshSQLWithLimit = `
        WITH stale_tickers AS (
            SELECT DISTINCT ticker
            FROM   screener_stale
            WHERE  stale = TRUE
              AND  last_update_started <= now() - $1::interval
            LIMIT  $2
        ), updated AS (
            UPDATE screener_stale s
            SET    last_update_started = now(),
                   stale = FALSE
            FROM   stale_tickers st
            WHERE  s.ticker = st.ticker
            RETURNING s.ticker
        )
        SELECT refresh_screener_delta(array(SELECT DISTINCT ticker FROM updated));
    `

	updateAndRefreshSQL = `
        WITH stale_tickers AS (
            SELECT DISTINCT ticker
            FROM   screener_stale
            WHERE  stale = TRUE
              AND  last_update_started <= now() - $1::interval
        ), updated AS (
            UPDATE screener_stale s
            SET    last_update_started = now(),
                   stale = FALSE
            FROM   stale_tickers st
            WHERE  s.ticker = st.ticker
            RETURNING s.ticker
        )
        SELECT refresh_screener_delta(array(SELECT DISTINCT ticker FROM updated));
    `
)

func updateStaleScreenerValues(conn *data.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
	defer cancel()
	intervalStr := fmt.Sprintf("%d seconds", int(refreshInterval.Seconds()))

	// Log current working directory for debugging
	if cwd, err := os.Getwd(); err == nil {
		log.Printf("📊 Current working directory: %s", cwd)
	}

	// Only enable minimal logging to avoid performance overhead
	if err := enableMinimalSessionLogging(ctx, conn); err != nil {
		log.Printf("⚠️  Failed to enable minimal session logging: %v", err)
	}

	log.Printf("🔄 Updating stale screener values (batch size: %d, timeout: %v)...", maxTickersPerBatch, refreshTimeout)
	start := time.Now()

	var mainQuery string
	var params []interface{}

	if maxTickersPerBatch > 0 {
		mainQuery = updateAndRefreshSQLWithLimit
		params = []interface{}{intervalStr, maxTickersPerBatch}
	} else {
		mainQuery = updateAndRefreshSQL
		params = []interface{}{intervalStr}
	}

	// Execute the main query
	_, err := conn.DB.Exec(ctx, mainQuery, params...)
	if err != nil {
		log.Printf("❌ updateStaleScreenerValues: failed to refresh screener data: %v", err)
		log.Printf("🔄 updateStaleScreenerValues: %v (failed)", time.Since(start))
		return
	}

	duration := time.Since(start)
	log.Printf("✅ Screener refresh completed successfully in %v (batch limit: %d)", duration, maxTickersPerBatch)

	// Only run detailed analysis if we're in debug mode or if the operation took too long
	// Lowered threshold to 5 seconds and expanded conditions to help with debugging
	if duration > 5*time.Second || maxTickersPerBatch <= 10 {
		log.Printf("📊 Starting performance analysis (duration: %v, batch size: %d)", duration, maxTickersPerBatch)
		go func() {
			// Run analysis in background to avoid blocking the main loop
			if err := runPerformanceAnalysis(conn, intervalStr); err != nil {
				log.Printf("⚠️  Background performance analysis failed: %v", err)
			}
		}()
	} else {
		log.Printf("📊 Skipping performance analysis (duration: %v, batch size: %d)", duration, maxTickersPerBatch)
	}

	log.Printf("🔄 updateStaleScreenerValues: %v", duration)
}

// enableMinimalSessionLogging only enables essential logging to avoid performance overhead
func enableMinimalSessionLogging(ctx context.Context, conn *data.Conn) error {
	// Only enable the most important settings that don't cause significant overhead
	settings := []string{
		"SET log_min_duration_statement = 5000", // Only log queries taking more than 5 seconds
		"SET log_lock_waits = on",
		"SET track_activities = on",
		"SET track_io_timing = on",
	}

	for _, setting := range settings {
		if _, err := conn.DB.Exec(ctx, setting); err != nil {
			// Don't log every failure to reduce noise
			continue
		}
	}
	return nil
}

// analyzeDatabaseConfiguration analyzes database configuration and version
func analyzeDatabaseConfiguration(ctx context.Context, conn *data.Conn, logFile *os.File) error {
	fmt.Fprintln(logFile, "📊 Database Configuration Analysis:")

	// Get database version
	var version string
	err := conn.DB.QueryRow(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		return fmt.Errorf("failed to get database version: %v", err)
	}
	fmt.Fprintf(logFile, "📊 Database Version: %s\n", version)

	// Get key configuration parameters
	configQuery := `
		SELECT name, setting, unit, context, source
		FROM pg_settings 
		WHERE name IN (
			'shared_buffers', 'work_mem', 'maintenance_work_mem', 'effective_cache_size',
			'random_page_cost', 'seq_page_cost', 'cpu_tuple_cost', 'cpu_index_tuple_cost',
			'cpu_operator_cost', 'effective_io_concurrency', 'max_worker_processes',
			'max_parallel_workers_per_gather', 'max_parallel_workers', 'wal_buffers',
			'checkpoint_completion_target', 'synchronous_commit', 'default_statistics_target'
		)
		ORDER BY name
	`

	rows, err := conn.DB.Query(ctx, configQuery)
	if err != nil {
		return fmt.Errorf("failed to get configuration: %v", err)
	}
	defer rows.Close()

	fmt.Fprintln(logFile, "📊 Key Configuration Parameters:")
	for rows.Next() {
		var name, setting, unit, context, source string
		if err := rows.Scan(&name, &setting, &unit, &context, &source); err != nil {
			continue
		}
		fmt.Fprintf(logFile, "📊   %s: %s %s (context: %s, source: %s)\n", name, setting, unit, context, source)
	}

	// Get database size info
	sizeQuery := `
		SELECT 
			pg_size_pretty(pg_database_size(current_database())) as db_size,
			pg_size_pretty(pg_total_relation_size('ohlcv_1m')) as ohlcv_1m_size,
			pg_size_pretty(pg_total_relation_size('ohlcv_1d')) as ohlcv_1d_size,
			pg_size_pretty(pg_total_relation_size('screener')) as screener_size
	`

	var dbSize, ohlcv1mSize, ohlcv1dSize, screenerSize string
	err = conn.DB.QueryRow(ctx, sizeQuery).Scan(&dbSize, &ohlcv1mSize, &ohlcv1dSize, &screenerSize)
	if err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to get size info: %v\n", err)
	} else {
		fmt.Fprintf(logFile, "📊 Database Size: %s\n", dbSize)
		fmt.Fprintf(logFile, "📊 OHLCV 1m Size: %s\n", ohlcv1mSize)
		fmt.Fprintf(logFile, "📊 OHLCV 1d Size: %s\n", ohlcv1dSize)
		fmt.Fprintf(logFile, "📊 Screener Size: %s\n", screenerSize)
	}

	fmt.Fprintln(logFile, "")
	return nil
}

// analyzeLockActivity analyzes current lock activity and blocking queries
func analyzeLockActivity(ctx context.Context, conn *data.Conn, logFile *os.File) error {
	fmt.Fprintln(logFile, "📊 Lock Activity Analysis:")

	lockQuery := `
		SELECT 
			pg_locks.locktype,
			pg_locks.mode,
			pg_locks.granted,
			pg_stat_activity.pid,
			pg_stat_activity.usename,
			pg_stat_activity.query,
			pg_stat_activity.state,
			now() - pg_stat_activity.query_start AS duration
		FROM pg_locks
		JOIN pg_stat_activity ON pg_locks.pid = pg_stat_activity.pid
		WHERE NOT pg_locks.granted
		   OR pg_locks.mode IN ('AccessExclusiveLock', 'ExclusiveLock', 'ShareUpdateExclusiveLock')
		ORDER BY duration DESC
		LIMIT 10
	`

	rows, err := conn.DB.Query(ctx, lockQuery)
	if err != nil {
		return fmt.Errorf("failed to analyze locks: %v", err)
	}
	defer rows.Close()

	lockCount := 0
	for rows.Next() {
		var lockType, mode, username, query, state string
		var granted bool
		var pid int
		var duration time.Duration

		if err := rows.Scan(&lockType, &mode, &granted, &pid, &username, &query, &state, &duration); err != nil {
			continue
		}

		lockCount++
		status := "GRANTED"
		if !granted {
			status = "BLOCKED"
		}

		fmt.Fprintf(logFile, "📊 Lock #%d: %s - %s (%s)\n", lockCount, lockType, mode, status)
		fmt.Fprintf(logFile, "📊   PID: %d, User: %s, Duration: %v\n", pid, username, duration)
		fmt.Fprintf(logFile, "📊   Query: %s\n", query[:min(len(query), 100)])
		fmt.Fprintln(logFile, "📊   ---")
	}

	if lockCount == 0 {
		fmt.Fprintln(logFile, "📊 No significant locks found")
	}

	fmt.Fprintln(logFile, "")
	return nil
}

// analyzeWaitEvents analyzes what the database is waiting for
func analyzeWaitEvents(ctx context.Context, conn *data.Conn, logFile *os.File) error {
	fmt.Fprintln(logFile, "📊 Wait Events Analysis:")

	waitQuery := `
		SELECT 
			wait_event_type,
			wait_event,
			COUNT(*) as count,
			AVG(EXTRACT(EPOCH FROM now() - query_start)) as avg_wait_time
		FROM pg_stat_activity
		WHERE wait_event IS NOT NULL
		  AND state = 'active'
		GROUP BY wait_event_type, wait_event
		ORDER BY count DESC, avg_wait_time DESC
		LIMIT 10
	`

	rows, err := conn.DB.Query(ctx, waitQuery)
	if err != nil {
		return fmt.Errorf("failed to analyze wait events: %v", err)
	}
	defer rows.Close()

	waitCount := 0
	for rows.Next() {
		var waitEventType, waitEvent string
		var count int
		var avgWaitTime float64

		if err := rows.Scan(&waitEventType, &waitEvent, &count, &avgWaitTime); err != nil {
			continue
		}

		waitCount++
		fmt.Fprintf(logFile, "📊 Wait Event: %s:%s (count: %d, avg: %.2fs)\n",
			waitEventType, waitEvent, count, avgWaitTime)
	}

	if waitCount == 0 {
		fmt.Fprintln(logFile, "📊 No wait events found")
	}

	fmt.Fprintln(logFile, "")
	return nil
}

// analyzeQueryPlans analyzes execution plans for key screener queries
func analyzeQueryPlans(ctx context.Context, conn *data.Conn, logFile *os.File) error {
	fmt.Fprintln(logFile, "📊 Query Plans Analysis:")

	// Test query plan for the main screener refresh query
	planQuery := `
		EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) 
		SELECT refresh_screener_delta(ARRAY['AAPL', 'GOOGL', 'MSFT'])
	`

	var planJSON string
	err := conn.DB.QueryRow(ctx, planQuery).Scan(&planJSON)
	if err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to get query plan: %v\n", err)
		return err
	}

	fmt.Fprintln(logFile, "📊 Execution Plan for refresh_screener_delta:")
	fmt.Fprintf(logFile, "📊 %s\n", planJSON)

	// Test historical price calculations
	historicalPlanQuery := `
		EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) 
		SELECT update_historical_price_calculations(ARRAY['AAPL', 'GOOGL', 'MSFT'])
	`

	err = conn.DB.QueryRow(ctx, historicalPlanQuery).Scan(&planJSON)
	if err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to get historical price query plan: %v\n", err)
	} else {
		fmt.Fprintln(logFile, "📊 Execution Plan for update_historical_price_calculations:")
		fmt.Fprintf(logFile, "📊 %s\n", planJSON)
	}

	fmt.Fprintln(logFile, "")
	return nil
}

// analyzeTableStatistics analyzes table and index statistics
func analyzeTableStatistics(ctx context.Context, conn *data.Conn, logFile *os.File) error {
	fmt.Fprintln(logFile, "📊 Table Statistics Analysis:")

	tableStatsQuery := `
		SELECT 
			schemaname,
			tablename,
			n_tup_ins,
			n_tup_upd,
			n_tup_del,
			n_live_tup,
			n_dead_tup,
			last_vacuum,
			last_autovacuum,
			last_analyze,
			last_autoanalyze,
			vacuum_count,
			autovacuum_count,
			analyze_count,
			autoanalyze_count
		FROM pg_stat_user_tables
		WHERE tablename IN ('ohlcv_1m', 'ohlcv_1d', 'screener', 'screener_stale', 'securities', 'intraday_stats', 'historical_price_calculations')
		ORDER BY n_live_tup DESC
	`

	rows, err := conn.DB.Query(ctx, tableStatsQuery)
	if err != nil {
		return fmt.Errorf("failed to get table statistics: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var schemaName, tableName string
		var nTupIns, nTupUpd, nTupDel, nLiveTup, nDeadTup int64
		var lastVacuum, lastAutovacuum, lastAnalyze, lastAutoanalyze *time.Time
		var vacuumCount, autovacuumCount, analyzeCount, autoanalyzeCount int64

		if err := rows.Scan(&schemaName, &tableName, &nTupIns, &nTupUpd, &nTupDel, &nLiveTup, &nDeadTup,
			&lastVacuum, &lastAutovacuum, &lastAnalyze, &lastAutoanalyze,
			&vacuumCount, &autovacuumCount, &analyzeCount, &autoanalyzeCount); err != nil {
			continue
		}

		fmt.Fprintf(logFile, "📊 Table: %s.%s\n", schemaName, tableName)
		fmt.Fprintf(logFile, "📊   Live tuples: %d, Dead tuples: %d (%.2f%% dead)\n",
			nLiveTup, nDeadTup, float64(nDeadTup)/float64(max64(nLiveTup+nDeadTup, 1))*100)
		fmt.Fprintf(logFile, "📊   Inserts: %d, Updates: %d, Deletes: %d\n", nTupIns, nTupUpd, nTupDel)

		if lastVacuum != nil {
			fmt.Fprintf(logFile, "📊   Last vacuum: %v\n", *lastVacuum)
		}
		if lastAutovacuum != nil {
			fmt.Fprintf(logFile, "📊   Last autovacuum: %v\n", *lastAutovacuum)
		}
		if lastAnalyze != nil {
			fmt.Fprintf(logFile, "📊   Last analyze: %v\n", *lastAnalyze)
		}
		if lastAutoanalyze != nil {
			fmt.Fprintf(logFile, "📊   Last autoanalyze: %v\n", *lastAutoanalyze)
		}

		fmt.Fprintf(logFile, "📊   Vacuum count: %d, Autovacuum count: %d\n", vacuumCount, autovacuumCount)
		fmt.Fprintf(logFile, "📊   Analyze count: %d, Autoanalyze count: %d\n", analyzeCount, autoanalyzeCount)
		fmt.Fprintln(logFile, "📊   ---")
	}

	fmt.Fprintln(logFile, "")
	return nil
}

// analyzeIndexUsage analyzes index usage and effectiveness
func analyzeIndexUsage(ctx context.Context, conn *data.Conn, logFile *os.File) error {
	fmt.Fprintln(logFile, "📊 Index Usage Analysis:")

	indexQuery := `
		SELECT 
			schemaname,
			tablename,
			indexname,
			idx_scan,
			idx_tup_read,
			idx_tup_fetch,
			pg_size_pretty(pg_relation_size(indexrelid)) as index_size
		FROM pg_stat_user_indexes
		WHERE tablename IN ('ohlcv_1m', 'ohlcv_1d', 'screener', 'screener_stale', 'securities', 'intraday_stats', 'historical_price_calculations')
		ORDER BY idx_scan DESC
		LIMIT 20
	`

	rows, err := conn.DB.Query(ctx, indexQuery)
	if err != nil {
		return fmt.Errorf("failed to get index usage: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var schemaName, tableName, indexName, indexSize string
		var idxScan, idxTupRead, idxTupFetch int64

		if err := rows.Scan(&schemaName, &tableName, &indexName, &idxScan, &idxTupRead, &idxTupFetch, &indexSize); err != nil {
			continue
		}

		fmt.Fprintf(logFile, "📊 Index: %s on %s.%s\n", indexName, schemaName, tableName)
		fmt.Fprintf(logFile, "📊   Scans: %d, Tuples read: %d, Tuples fetched: %d\n", idxScan, idxTupRead, idxTupFetch)
		fmt.Fprintf(logFile, "📊   Size: %s\n", indexSize)

		// Calculate efficiency metrics
		if idxScan > 0 {
			fmt.Fprintf(logFile, "📊   Avg tuples per scan: %.2f\n", float64(idxTupRead)/float64(idxScan))
		}
		if idxScan == 0 {
			fmt.Fprintf(logFile, "📊   ⚠️  Index not being used!\n")
		}
		fmt.Fprintln(logFile, "📊   ---")
	}

	fmt.Fprintln(logFile, "")
	return nil
}

// analyzeMemoryUsage analyzes memory usage and temp file creation
func analyzeMemoryUsage(ctx context.Context, conn *data.Conn, logFile *os.File) error {
	fmt.Fprintln(logFile, "📊 Memory Usage Analysis:")

	// Get temp file usage
	tempQuery := `
		SELECT 
			datname,
			temp_files,
			temp_bytes,
			pg_size_pretty(temp_bytes) as temp_size
		FROM pg_stat_database
		WHERE datname = current_database()
	`

	var datname, tempSize string
	var tempFiles, tempBytes int64
	err := conn.DB.QueryRow(ctx, tempQuery).Scan(&datname, &tempFiles, &tempBytes, &tempSize)
	if err != nil {
		return fmt.Errorf("failed to get temp file usage: %v", err)
	}

	fmt.Fprintf(logFile, "📊 Database: %s\n", datname)
	fmt.Fprintf(logFile, "📊 Temp files created: %d\n", tempFiles)
	fmt.Fprintf(logFile, "📊 Temp bytes used: %s\n", tempSize)

	if tempFiles > 0 {
		fmt.Fprintf(logFile, "📊 ⚠️  Temp files indicate work_mem may be too small!\n")
	}

	// Get buffer hit ratios
	bufferQuery := `
		SELECT 
			buffers_clean,
			buffers_checkpoint,
			buffers_backend,
			buffers_backend_fsync,
			buffers_alloc
		FROM pg_stat_bgwriter
	`

	var buffersClean, buffersCheckpoint, buffersBackend, buffersBackendFsync, buffersAlloc int64
	err = conn.DB.QueryRow(ctx, bufferQuery).Scan(&buffersClean, &buffersCheckpoint, &buffersBackend, &buffersBackendFsync, &buffersAlloc)
	if err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to get buffer stats: %v\n", err)
	} else {
		fmt.Fprintf(logFile, "📊 Buffer stats:\n")
		fmt.Fprintf(logFile, "📊   Clean: %d, Checkpoint: %d, Backend: %d\n", buffersClean, buffersCheckpoint, buffersBackend)
		fmt.Fprintf(logFile, "📊   Backend fsync: %d, Allocated: %d\n", buffersBackendFsync, buffersAlloc)
	}

	fmt.Fprintln(logFile, "")
	return nil
}

// analyzeMaintenanceStatus analyzes vacuum and analyze status
func analyzeMaintenanceStatus(ctx context.Context, conn *data.Conn, logFile *os.File) error {
	fmt.Fprintln(logFile, "📊 Maintenance Status Analysis:")

	maintenanceQuery := `
		SELECT 
			schemaname,
			tablename,
			n_dead_tup,
			n_live_tup,
			CASE 
				WHEN n_live_tup = 0 THEN 0
				ELSE (n_dead_tup::float / n_live_tup::float) * 100
			END as dead_tuple_pct,
			last_vacuum,
			last_autovacuum,
			last_analyze,
			last_autoanalyze
		FROM pg_stat_user_tables
		WHERE tablename IN ('ohlcv_1m', 'ohlcv_1d', 'screener', 'screener_stale', 'securities', 'intraday_stats', 'historical_price_calculations')
		ORDER BY dead_tuple_pct DESC
	`

	rows, err := conn.DB.Query(ctx, maintenanceQuery)
	if err != nil {
		return fmt.Errorf("failed to get maintenance status: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var schemaName, tableName string
		var nDeadTup, nLiveTup int64
		var deadTuplePct float64
		var lastVacuum, lastAutovacuum, lastAnalyze, lastAutoanalyze *time.Time

		if err := rows.Scan(&schemaName, &tableName, &nDeadTup, &nLiveTup, &deadTuplePct,
			&lastVacuum, &lastAutovacuum, &lastAnalyze, &lastAutoanalyze); err != nil {
			continue
		}

		fmt.Fprintf(logFile, "📊 Table: %s.%s\n", schemaName, tableName)
		fmt.Fprintf(logFile, "📊   Dead tuples: %d (%.2f%% of live)\n", nDeadTup, deadTuplePct)

		if deadTuplePct > 20 {
			fmt.Fprintf(logFile, "📊   ⚠️  High dead tuple percentage - needs vacuum!\n")
		}

		now := time.Now()
		if lastVacuum != nil {
			fmt.Fprintf(logFile, "📊   Last vacuum: %v (%v ago)\n", *lastVacuum, now.Sub(*lastVacuum))
		}
		if lastAutovacuum != nil {
			fmt.Fprintf(logFile, "📊   Last autovacuum: %v (%v ago)\n", *lastAutovacuum, now.Sub(*lastAutovacuum))
		}
		if lastAnalyze != nil {
			fmt.Fprintf(logFile, "📊   Last analyze: %v (%v ago)\n", *lastAnalyze, now.Sub(*lastAnalyze))
		}
		if lastAutoanalyze != nil {
			fmt.Fprintf(logFile, "📊   Last autoanalyze: %v (%v ago)\n", *lastAutoanalyze, now.Sub(*lastAutoanalyze))
		}

		fmt.Fprintln(logFile, "📊   ---")
	}

	fmt.Fprintln(logFile, "")
	return nil
}

// analyzeConcurrentQueries analyzes concurrent query impact
func analyzeConcurrentQueries(ctx context.Context, conn *data.Conn, logFile *os.File) error {
	fmt.Fprintln(logFile, "📊 Concurrent Query Impact Analysis:")

	concurrentQuery := `
		SELECT 
			COUNT(*) as total_active,
			COUNT(*) FILTER (WHERE query ILIKE '%screener%') as screener_queries,
			COUNT(*) FILTER (WHERE query ILIKE '%ohlcv%') as ohlcv_queries,
			COUNT(*) FILTER (WHERE state = 'active') as active_queries,
			COUNT(*) FILTER (WHERE wait_event IS NOT NULL) as waiting_queries,
			AVG(EXTRACT(EPOCH FROM now() - query_start)) as avg_query_duration
		FROM pg_stat_activity
		WHERE state != 'idle'
		  AND query NOT LIKE '%pg_stat_activity%'
	`

	var totalActive, screenerQueries, ohlcvQueries, activeQueries, waitingQueries int
	var avgQueryDuration float64

	err := conn.DB.QueryRow(ctx, concurrentQuery).Scan(&totalActive, &screenerQueries, &ohlcvQueries,
		&activeQueries, &waitingQueries, &avgQueryDuration)
	if err != nil {
		return fmt.Errorf("failed to get concurrent query stats: %v", err)
	}

	fmt.Fprintf(logFile, "📊 Total active connections: %d\n", totalActive)
	fmt.Fprintf(logFile, "📊 Screener-related queries: %d\n", screenerQueries)
	fmt.Fprintf(logFile, "📊 OHLCV-related queries: %d\n", ohlcvQueries)
	fmt.Fprintf(logFile, "📊 Active queries: %d\n", activeQueries)
	fmt.Fprintf(logFile, "📊 Waiting queries: %d\n", waitingQueries)
	fmt.Fprintf(logFile, "📊 Average query duration: %.2f seconds\n", avgQueryDuration)

	if waitingQueries > 0 {
		fmt.Fprintf(logFile, "📊 ⚠️  %d queries are waiting - possible contention!\n", waitingQueries)
	}

	// Get specific concurrent queries
	specificQuery := `
		SELECT 
			pid,
			usename,
			state,
			wait_event_type,
			wait_event,
			EXTRACT(EPOCH FROM now() - query_start) as duration,
			left(query, 100) as query_start
		FROM pg_stat_activity
		WHERE state = 'active'
		  AND query NOT LIKE '%pg_stat_activity%'
		  AND pid != pg_backend_pid()
		ORDER BY duration DESC
		LIMIT 10
	`

	rows, err := conn.DB.Query(ctx, specificQuery)
	if err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to get specific concurrent queries: %v\n", err)
		return err
	}
	defer rows.Close()

	queryCount := 0
	for rows.Next() {
		var pid int
		var username, state, queryStart string
		var waitEventType, waitEvent *string
		var duration float64

		if err := rows.Scan(&pid, &username, &state, &waitEventType, &waitEvent, &duration, &queryStart); err != nil {
			continue
		}

		queryCount++
		waitInfo := "None"
		if waitEventType != nil && waitEvent != nil {
			waitInfo = fmt.Sprintf("%s:%s", *waitEventType, *waitEvent)
		}

		fmt.Fprintf(logFile, "📊 Concurrent Query #%d:\n", queryCount)
		fmt.Fprintf(logFile, "📊   PID: %d, User: %s, State: %s, Duration: %.2fs\n", pid, username, state, duration)
		fmt.Fprintf(logFile, "📊   Wait: %s\n", waitInfo)
		fmt.Fprintf(logFile, "📊   Query: %s\n", queryStart)
		fmt.Fprintln(logFile, "📊   ---")
	}

	fmt.Fprintln(logFile, "")
	return nil
}

// analyzeScreenerQueryPerformance runs specific performance tests on screener queries
func analyzeScreenerQueryPerformance(ctx context.Context, conn *data.Conn, logFile *os.File, tickers []string) error {
	fmt.Fprintln(logFile, "📊 Screener Query Performance Analysis:")

	if len(tickers) == 0 {
		fmt.Fprintln(logFile, "📊 No tickers to analyze")
		return nil
	}

	// Test a sample of tickers
	sampleTickers := tickers[:min(len(tickers), 3)]

	// Test individual function performance
	functions := []struct {
		name  string
		query string
	}{
		{"update_intraday_stats", "SELECT update_intraday_stats($1)"},
		{"update_historical_price_calculations", "SELECT update_historical_price_calculations($1)"},
		{"refresh_screener_delta", "SELECT refresh_screener_delta($1)"},
	}

	for _, fn := range functions {
		fmt.Fprintf(logFile, "📊 Testing %s with %d tickers...\n", fn.name, len(sampleTickers))

		start := time.Now()
		_, err := conn.DB.Exec(ctx, fn.query, sampleTickers)
		duration := time.Since(start)

		if err != nil {
			fmt.Fprintf(logFile, "📊   ❌ Failed: %v\n", err)
		} else {
			fmt.Fprintf(logFile, "📊   ✅ Success: %v (%.2f ms per ticker)\n",
				duration, float64(duration.Nanoseconds())/float64(len(sampleTickers))/1000000)
		}
	}

	// Test query components individually
	componentTests := []struct {
		name  string
		query string
	}{
		{"OHLCV 1m latest", "SELECT ticker, close/1000.0 FROM ohlcv_1m WHERE ticker = ANY($1) ORDER BY timestamp DESC LIMIT 10"},
		{"OHLCV 1d latest", "SELECT ticker, close/1000.0 FROM ohlcv_1d WHERE ticker = ANY($1) ORDER BY timestamp DESC LIMIT 10"},
		{"Securities lookup", "SELECT ticker, market_cap, sector FROM securities WHERE ticker = ANY($1)"},
		{"Intraday stats", "SELECT ticker, change_1h_pct FROM intraday_stats WHERE ticker = ANY($1)"},
		{"Historical calculations", "SELECT ticker, price_1w, price_1m FROM historical_price_calculations WHERE ticker = ANY($1)"},
	}

	for _, test := range componentTests {
		start := time.Now()
		rows, err := conn.DB.Query(ctx, test.query, sampleTickers)
		if err != nil {
			fmt.Fprintf(logFile, "📊 %s: ❌ %v\n", test.name, err)
			continue
		}

		rowCount := 0
		for rows.Next() {
			rowCount++
			var ticker string
			var value float64
			rows.Scan(&ticker, &value)
		}
		rows.Close()

		duration := time.Since(start)
		fmt.Fprintf(logFile, "📊 %s: ✅ %v (%d rows, %.2f ms per row)\n",
			test.name, duration, rowCount, float64(duration.Nanoseconds())/float64(max(rowCount, 1))/1000000)
	}

	fmt.Fprintln(logFile, "")
	return nil
}

// Helper function to get maximum value
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Helper function to get maximum value for int64
func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// runPerformanceAnalysis runs a comprehensive performance analysis
func runPerformanceAnalysis(conn *data.Conn, intervalStr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second) // Increased timeout for comprehensive analysis
	defer cancel()

	// Create analysis log file with absolute path to ensure it's written to the correct location
	// In Docker container, /app is the working directory which is volume mounted to the host
	logFilePath := "/app/screener_analysis.log"

	// Add debug logging to help diagnose the issue
	log.Printf("📊 Creating performance analysis log at: %s", logFilePath)

	logFile, err := os.Create(logFilePath)
	if err != nil {
		log.Printf("❌ Failed to create analysis log file at %s: %v", logFilePath, err)

		// Try fallback location in case the primary path fails
		fallbackPath := "./screener_analysis.log"
		log.Printf("📊 Trying fallback location: %s", fallbackPath)

		logFile, err = os.Create(fallbackPath)
		if err != nil {
			log.Printf("❌ Failed to create analysis log file at fallback location %s: %v", fallbackPath, err)
			return fmt.Errorf("failed to create analysis log file: %v", err)
		}
		logFilePath = fallbackPath
	}
	defer logFile.Close()

	log.Printf("✅ Successfully created analysis log file at: %s", logFilePath)

	// Write header
	fmt.Fprintf(logFile, "=== SCREENER PERFORMANCE ANALYSIS LOG - %s ===\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(logFile, "Batch limit: %d\n", maxTickersPerBatch)
	fmt.Fprintf(logFile, "Interval: %s\n", intervalStr)
	fmt.Fprintf(logFile, "Timeout: %v\n", refreshTimeout)
	fmt.Fprintf(logFile, "Log file path: %s\n\n", logFilePath)

	// Get database version and configuration
	if err := analyzeDatabaseConfiguration(ctx, conn, logFile); err != nil {
		fmt.Fprintf(logFile, "⚠️  Database configuration analysis failed: %v\n", err)
	}

	// Get the tickers that would be processed
	tickerQuery := `
		SELECT ticker, last_update_started, stale
		FROM screener_stale
		WHERE stale = TRUE
		  AND last_update_started <= now() - $1::interval
		ORDER BY last_update_started ASC
		LIMIT $2
	`

	var tickerParams []interface{}
	if maxTickersPerBatch > 0 {
		tickerParams = []interface{}{intervalStr, maxTickersPerBatch}
	} else {
		tickerQuery = `
			SELECT ticker, last_update_started, stale
			FROM screener_stale
			WHERE stale = TRUE
			  AND last_update_started <= now() - $1::interval
			ORDER BY last_update_started ASC
		`
		tickerParams = []interface{}{intervalStr}
	}

	rows, err := conn.DB.Query(ctx, tickerQuery, tickerParams...)
	if err != nil {
		fmt.Fprintf(logFile, "⚠️  Failed to get ticker list: %v\n", err)
		return err
	}
	defer rows.Close()

	var tickers []string
	for rows.Next() {
		var ticker string
		var lastUpdate time.Time
		var stale bool
		if err := rows.Scan(&ticker, &lastUpdate, &stale); err == nil {
			tickers = append(tickers, ticker)
		}
	}

	if len(tickers) == 0 {
		fmt.Fprintln(logFile, "📊 No stale tickers found")
		return nil
	}

	fmt.Fprintf(logFile, "\n📊 Total tickers to process: %d\n", len(tickers))
	fmt.Fprintf(logFile, "📊 Sample tickers: %v\n\n", tickers[:min(len(tickers), 5)])

	// Run comprehensive analysis
	analyses := []struct {
		name string
		fn   func(context.Context, *data.Conn, *os.File) error
	}{
		{"Database Activity", analyzeDatabaseActivity},
		{"Lock Analysis", analyzeLockActivity},
		{"Wait Events", analyzeWaitEvents},
		{"Query Performance", analyzePgStatStatements},
		{"Query Plans", analyzeQueryPlans},
		{"Table Statistics", analyzeTableStatistics},
		{"Index Usage", analyzeIndexUsage},
		{"Memory Usage", analyzeMemoryUsage},
		{"Vacuum & Analyze Status", analyzeMaintenanceStatus},
		{"Concurrent Query Impact", analyzeConcurrentQueries},
	}

	for _, analysis := range analyses {
		fmt.Fprintf(logFile, "🔍 Running %s Analysis...\n", analysis.name)
		if err := analysis.fn(ctx, conn, logFile); err != nil {
			fmt.Fprintf(logFile, "⚠️  %s analysis failed: %v\n\n", analysis.name, err)
		}
	}

	// Run specific screener query analysis
	if err := analyzeScreenerQueryPerformance(ctx, conn, logFile, tickers); err != nil {
		fmt.Fprintf(logFile, "⚠️  Screener query analysis failed: %v\n", err)
	}

	fmt.Fprintf(logFile, "\n📊 Analysis complete at %s\n", time.Now().Format("2006-01-02 15:04:05"))

	// Ensure the file is written to disk
	if err := logFile.Sync(); err != nil {
		log.Printf("⚠️  Failed to sync log file: %v", err)
	}

	log.Printf("📊 Performance analysis complete - logs written to: %s", logFilePath)

	return nil
}

// setupIncrementalInfrastructure sets up the stale queue, triggers, and functions
func runScreenerLoopInit(conn *data.Conn) error {
	log.Println("🔧 Setting up incremental infrastructure (time-based batching)...")

	log.Println("🔧 Dropping all materialized views and tables...")
	if _, err := conn.DB.Exec(context.Background(), initQuery); err != nil {
		return fmt.Errorf("failed to drop all materialized views and tables: %v", err)
	} /**/

	log.Println("📊 Creating stale queue table...")
	if _, err := conn.DB.Exec(context.Background(), createStaleQueueQuery); err != nil {
		return fmt.Errorf("failed to create processing state table: %v", err)
	}

	// Insert initial stale tickers from securities where maxDate is null
	log.Println("📊 Inserting initial stale tickers from securities where maxDate is null...")
	if _, err := conn.DB.Exec(context.Background(), insertInitialStaleTickersQuery); err != nil {
		return fmt.Errorf("failed to insert initial stale tickers: %v", err)
	}

	log.Println("📊 Creating OHLCV indexes...")
	if _, err := conn.DB.Exec(context.Background(), createOHLCVIndexesQuery); err != nil {
		return fmt.Errorf("failed to create OHLCV indexes: %v", err)
	}

	log.Println("📊 Creating pre-market stats materialized view...")
	if _, err := conn.DB.Exec(context.Background(), createPreMarketStatsQuery); err != nil {
		return fmt.Errorf("failed to create pre-market stats view: %v", err)
	}

	log.Println("📊 Creating intraday stats table and function...")
	if _, err := conn.DB.Exec(context.Background(), intradayPriceRefsQuery); err != nil {
		return fmt.Errorf("failed to create intraday stats table: %v", err)
	}

	log.Println("📊 Creating historical price refs materialized view and calculations table...")
	if _, err := conn.DB.Exec(context.Background(), createHistoricalPriceRefsQuery); err != nil {
		return fmt.Errorf("failed to create historical price refs view: %v", err)
	}

	log.Println("🔄 Creating incremental refresh function (delta)...")
	if _, err := conn.DB.Exec(context.Background(), incrementalRefreshQuery); err != nil {
		return fmt.Errorf("failed to create incremental refresh function: %v", err)
	}

	log.Println("✅ Incremental infrastructure (time-based batching) setup complete")
	return nil
}

// dropAllViewsQuery - UNUSED - For fresh restart when needed
// This query drops all materialized views and aggregates created by the screener updater
// Use with caution - will require full rebuild of all screener infrastructure

var initQuery = `
-- Drop all continuous aggregate policies first (with proper error handling)
-- Check if continuous aggregate exists and has policies before removing
DO $$
BEGIN
    -- Remove pre_market_stats policy if it exists
    IF EXISTS (
        SELECT 1 FROM timescaledb_information.continuous_aggregates 
        WHERE view_name = 'pre_market_stats'
    ) THEN
        BEGIN
            PERFORM remove_continuous_aggregate_policy('pre_market_stats', true);
        EXCEPTION
            WHEN OTHERS THEN
                RAISE NOTICE 'Could not remove pre_market_stats policy: %', SQLERRM;
        END;
    END IF;

    -- Remove historical_price_refs policy if it exists
    IF EXISTS (
        SELECT 1 FROM timescaledb_information.continuous_aggregates 
        WHERE view_name = 'historical_price_refs'
    ) THEN
        BEGIN
            PERFORM remove_continuous_aggregate_policy('historical_price_refs', true);
        EXCEPTION
            WHEN OTHERS THEN
                RAISE NOTICE 'Could not remove historical_price_refs policy: %', SQLERRM;
        END;
    END IF;
END $$;

-- Drop all materialized views and continuous aggregates
DROP MATERIALIZED VIEW IF EXISTS pre_market_stats CASCADE;
DROP TABLE IF EXISTS intraday_stats CASCADE;
DROP MATERIALIZED VIEW IF EXISTS historical_price_refs CASCADE;
DROP TABLE IF EXISTS historical_price_calculations CASCADE;

-- Drop supporting tables and functions
DROP TABLE IF EXISTS screener_stale CASCADE;
DROP FUNCTION IF EXISTS public.refresh_screener_delta(text[]) CASCADE;
DROP FUNCTION IF EXISTS update_intraday_stats(text[]) CASCADE;
DROP FUNCTION IF EXISTS update_historical_price_calculations(text[]) CASCADE;

-- Drop old indexes that may exist
DROP INDEX IF EXISTS screener_stale_due_idx;
DROP INDEX IF EXISTS screener_stale_timestamp_partial_idx;
DROP INDEX IF EXISTS screener_stale_ticker_stale_idx;

-- Drop indexes (will be dropped with CASCADE but listed for clarity)
-- DROP INDEX IF EXISTS pre_market_stats_ticker_day_idx;
-- DROP INDEX IF EXISTS intraday_stats_ticker_idx;
-- DROP INDEX IF EXISTS intraday_stats_ts_idx;
-- DROP INDEX IF EXISTS historical_price_refs_ticker_idx;
-- DROP INDEX IF EXISTS historical_price_refs_ticker_bucket_idx;
-- DROP INDEX IF EXISTS historical_price_refs_bucket_idx;
-- DROP INDEX IF EXISTS historical_price_calculations_ticker_idx;
-- DROP INDEX IF EXISTS historical_price_calculations_updated_idx;
-- DROP INDEX IF EXISTS ohlcv_1d_ticker_ts_desc_inc;
-- DROP INDEX IF EXISTS ohlcv_1m_ticker_ts_desc_inc;

-- Note: After running this cleanup, you must restart the screener updater
-- to recreate all views, functions, and policies









`

// Alternative approach: Enable detailed session logging for comprehensive query analysis
// This approach captures ALL SQL statements executed within stored procedures
func enableDetailedSessionLogging(ctx context.Context, conn *data.Conn) error {
	// Only enable settings that can be changed at runtime for a session
	settings := []string{
		"SET log_statement = 'all'",
		"SET log_duration = on",
		"SET log_min_duration_statement = 0",
		"SET log_lock_waits = on",
		"SET log_temp_files = 0",
		"SET track_activities = on",
		"SET track_counts = on",
		"SET track_io_timing = on",
		"SET track_functions = 'all'",
		"SET log_parser_stats = on",
		"SET log_planner_stats = on",
		"SET log_executor_stats = on",
		// Remove log_statement_stats as it conflicts with individual stats
		"SET auto_explain.log_min_duration = 0",
		"SET auto_explain.log_analyze = on",
		"SET auto_explain.log_buffers = on",
		"SET auto_explain.log_timing = on",
		"SET auto_explain.log_triggers = on",
		"SET auto_explain.log_verbose = on",
		"SET auto_explain.log_nested_statements = on",
		"SET auto_explain.log_format = 'text'",
	}

	successCount := 0
	for _, setting := range settings {
		if _, err := conn.DB.Exec(ctx, setting); err != nil {
			log.Printf("⚠️  Failed to set %s: %v", setting, err)
			// Continue with other settings even if one fails
		} else {
			successCount++
		}
	}

	log.Printf("✅ Successfully set %d/%d logging parameters", successCount, len(settings))
	return nil
}

// Alternative approach: Use pg_stat_statements to get detailed query statistics
func analyzePgStatStatements(ctx context.Context, conn *data.Conn, logFile *os.File) error {
	fmt.Fprintln(logFile, "📊 Analyzing pg_stat_statements for query performance...")

	// First, check which columns exist in pg_stat_statements (column names changed in different PostgreSQL versions)
	checkColumnsQuery := `
		SELECT 
			CASE WHEN EXISTS (
				SELECT 1 FROM information_schema.columns 
				WHERE table_name = 'pg_stat_statements' AND column_name = 'total_time'
			) THEN 'total_time' ELSE 'total_exec_time' END AS time_column,
			CASE WHEN EXISTS (
				SELECT 1 FROM information_schema.columns 
				WHERE table_name = 'pg_stat_statements' AND column_name = 'mean_time'
			) THEN 'mean_time' ELSE 'mean_exec_time' END AS mean_column,
			EXISTS (
				SELECT 1 FROM information_schema.columns 
				WHERE table_name = 'pg_stat_statements' AND column_name = 'blk_read_time'
			) AS has_io_timing,
			EXISTS (
				SELECT 1 FROM information_schema.columns 
				WHERE table_name = 'pg_stat_statements' AND column_name = 'max_exec_time'
			) AS has_max_exec_time
	`

	var timeColumn, meanColumn string
	var hasIoTiming, hasMaxExecTime bool
	err := conn.DB.QueryRow(ctx, checkColumnsQuery).Scan(&timeColumn, &meanColumn, &hasIoTiming, &hasMaxExecTime)
	if err != nil {
		// Fallback to newer column names if the check fails
		timeColumn = "total_exec_time"
		meanColumn = "mean_exec_time"
		hasIoTiming = false
		hasMaxExecTime = true
		fmt.Fprintf(logFile, "⚠️  Failed to check pg_stat_statements columns, using fallback: %v\n", err)
	}

	fmt.Fprintf(logFile, "📊 Using columns: time=%s, mean=%s, has_io_timing=%v, has_max_exec_time=%v\n",
		timeColumn, meanColumn, hasIoTiming, hasMaxExecTime)

	// Build the query dynamically based on available columns
	var selectClause string
	var maxExecClause string
	var ioTimingClause string

	if hasMaxExecTime {
		maxExecClause = "max_exec_time, min_exec_time,"
	} else {
		maxExecClause = "NULL as max_exec_time, NULL as min_exec_time,"
	}

	if hasIoTiming {
		ioTimingClause = "blk_read_time, blk_write_time"
	} else {
		ioTimingClause = "NULL as blk_read_time, NULL as blk_write_time"
	}

	selectClause = fmt.Sprintf(`
		SELECT 
			substring(query for 100) as short_query,
			calls,
			%s as total_time,
			%s as mean_time,
			%s
			rows,
			100.0 * shared_blks_hit / nullif(shared_blks_hit + shared_blks_read, 0) AS hit_percent,
			shared_blks_read,
			shared_blks_hit,
			shared_blks_dirtied,
			shared_blks_written,
			local_blks_read,
			local_blks_hit,
			local_blks_dirtied,
			local_blks_written,
			temp_blks_read,
			temp_blks_written,
			%s
	`, timeColumn, meanColumn, maxExecClause, ioTimingClause)

	pgStatQuery := fmt.Sprintf(`
		%s
		FROM pg_stat_statements 
		WHERE query ILIKE '%%screener%%' 
		   OR query ILIKE '%%ohlcv%%'
		   OR query ILIKE '%%refresh_screener_delta%%'
		   OR query ILIKE '%%update_intraday_stats%%'
		   OR query ILIKE '%%update_historical_price_calculations%%'
		ORDER BY %s DESC
		LIMIT 20
	`, selectClause, timeColumn)

	rows, err := conn.DB.Query(ctx, pgStatQuery)
	if err != nil {
		return fmt.Errorf("failed to query pg_stat_statements: %v", err)
	}
	defer rows.Close()

	fmt.Fprintln(logFile, "📊 Top 20 most expensive screener-related queries:")
	queryCount := 0
	for rows.Next() {
		var shortQuery string
		var calls int64
		var totalTime, meanTime float64
		var maxTime, minTime *float64
		var rowsProcessed int64
		var hitPercent *float64
		var sharedBlksRead, sharedBlksHit, sharedBlksDirtied, sharedBlksWritten int64
		var localBlksRead, localBlksHit, localBlksDirtied, localBlksWritten int64
		var tempBlksRead, tempBlksWritten int64
		var blkReadTime, blkWriteTime *float64

		if err := rows.Scan(&shortQuery, &calls, &totalTime, &meanTime, &maxTime, &minTime,
			&rowsProcessed, &hitPercent, &sharedBlksRead, &sharedBlksHit, &sharedBlksDirtied,
			&sharedBlksWritten, &localBlksRead, &localBlksHit, &localBlksDirtied, &localBlksWritten,
			&tempBlksRead, &tempBlksWritten, &blkReadTime, &blkWriteTime); err != nil {
			fmt.Fprintf(logFile, "⚠️  Failed to scan pg_stat_statements row: %v\n", err)
			continue
		}

		queryCount++
		hitPercentStr := "N/A"
		if hitPercent != nil {
			hitPercentStr = fmt.Sprintf("%.2f%%", *hitPercent)
		}

		maxTimeStr := "N/A"
		if maxTime != nil {
			maxTimeStr = fmt.Sprintf("%.2fms", *maxTime)
		}

		readTimeStr := "N/A"
		writeTimeStr := "N/A"
		if blkReadTime != nil {
			readTimeStr = fmt.Sprintf("%.2fms", *blkReadTime)
		}
		if blkWriteTime != nil {
			writeTimeStr = fmt.Sprintf("%.2fms", *blkWriteTime)
		}

		fmt.Fprintf(logFile, "📊 Query #%d: %s\n", queryCount, shortQuery)
		fmt.Fprintf(logFile, "📊   Performance: %d calls, %.2fms total, %.2fms mean, %s max\n",
			calls, totalTime, meanTime, maxTimeStr)
		fmt.Fprintf(logFile, "📊   Rows: %d processed (%.2f rows/call)\n",
			rowsProcessed, float64(rowsProcessed)/float64(max64(calls, 1)))
		fmt.Fprintf(logFile, "📊   Cache: %s hit ratio, %d reads, %d hits\n",
			hitPercentStr, sharedBlksRead, sharedBlksHit)

		// Performance warnings
		if calls > 0 && meanTime > 1000 {
			fmt.Fprintf(logFile, "📊   ⚠️  SLOW: Mean time > 1s - optimization needed!\n")
		}
		if hitPercent != nil && *hitPercent < 90 {
			fmt.Fprintf(logFile, "📊   ⚠️  LOW CACHE HIT: %.2f%% - consider increasing shared_buffers\n", *hitPercent)
		}
		if tempBlksRead > 0 || tempBlksWritten > 0 {
			fmt.Fprintf(logFile, "📊   ⚠️  TEMP FILES: %d read, %d written - increase work_mem\n",
				tempBlksRead, tempBlksWritten)
		}

		// I/O analysis
		if blkReadTime != nil && blkWriteTime != nil {
			fmt.Fprintf(logFile, "📊   I/O: Read %s, Write %s\n", readTimeStr, writeTimeStr)
		}

		fmt.Fprintln(logFile, "📊   ---")
	}

	if queryCount == 0 {
		fmt.Fprintln(logFile, "📊 No screener-related queries found in pg_stat_statements")
	}

	fmt.Fprintln(logFile, "") // Add spacing after section
	return nil
}

// Helper function to get current database activity and locks
func analyzeDatabaseActivity(ctx context.Context, conn *data.Conn, logFile *os.File) error {
	fmt.Fprintln(logFile, "📊 Analyzing current database activity...")

	// Get current active queries
	activityQuery := `
		SELECT 
			pid,
			now() - pg_stat_activity.query_start AS duration,
			query,
			state,
			wait_event_type,
			wait_event
		FROM pg_stat_activity 
		WHERE state = 'active' 
		  AND query NOT LIKE '%pg_stat_activity%'
		ORDER BY duration DESC
		LIMIT 10
	`

	rows, err := conn.DB.Query(ctx, activityQuery)
	if err != nil {
		return fmt.Errorf("failed to query pg_stat_activity: %v", err)
	}
	defer rows.Close()

	fmt.Fprintln(logFile, "📊 Current active queries:")
	for rows.Next() {
		var pid int
		var duration time.Duration
		var query, state string
		var waitEventType, waitEvent *string

		if err := rows.Scan(&pid, &duration, &query, &state, &waitEventType, &waitEvent); err != nil {
			fmt.Fprintf(logFile, "⚠️  Failed to scan activity row: %v\n", err)
			continue
		}

		waitInfo := "None"
		if waitEventType != nil && waitEvent != nil {
			waitInfo = fmt.Sprintf("%s:%s", *waitEventType, *waitEvent)
		}

		fmt.Fprintf(logFile, "📊 PID: %d, Duration: %v, State: %s, Wait: %s\n", pid, duration, state, waitInfo)
		fmt.Fprintf(logFile, "📊   Query: %s\n", query[:min(len(query), 100)])
		fmt.Fprintln(logFile, "📊   ---")
	}

	fmt.Fprintln(logFile, "") // Add spacing after section
	return nil
}

// Helper function to get minimum value
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
