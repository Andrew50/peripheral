/*
GOAL: 10k ticker refreshed in 10s

OPTIMIZATION TO TRY:

-- parallelize the screener update with golang workers



OPTIMIZATION LOG: -- baseline 3.6s for 1 ticker

*/

package screener

import (
	"backend/internal/data"
	"context"
	"fmt"
	"log"
	"time"
)

const (
	refreshInterval    = 60 * time.Second // full screener top-off frequency (fallback)
	refreshTimeout     = 60 * time.Second // per-refresh SQL timeout
	extendedCloseHour  = 20               // 8 PM Eastern – hard stop
	maxTickersPerBatch = 1                // max tickers to process per batch (0 = no limit), used for testing
)

var createStaleQueueQuery = `
CREATE TABLE IF NOT EXISTS screener_stale (
    ticker text PRIMARY KEY,
    last_update_started timestamptz DEFAULT '1970-01-01',
    stale boolean DEFAULT TRUE
);

-- Index the stale-queue hot path for efficient lookups
CREATE INDEX IF NOT EXISTS screener_stale_due_idx
  ON screener_stale (stale, last_update_started)
  INCLUDE (ticker);
`

// Add SQL to insert initial stale tickers for securities where maxDate is null
var insertInitialStaleTickersQuery = `
INSERT INTO screener_stale (ticker)
SELECT ticker FROM securities
WHERE maxDate IS NULL
ON CONFLICT (ticker) DO NOTHING;
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
    first(open,  "timestamp")                    AS pre_market_open,
    last (close, "timestamp")                    AS pre_market_close,
    max  (high)                                  AS pre_market_high,
    min  (low)                                   AS pre_market_low,
    sum  (volume)                                AS pre_market_volume,
    sum  (volume * close)                        AS pre_market_dollar_volume,
    /* derived metrics */
    (max(high) - min(low)) / NULLIF(min(low),0) * 100        AS pre_market_range_pct,
    last(close, "timestamp") - first(open, "timestamp")   AS pre_market_change,
    (last(close, "timestamp") - first(open, "timestamp"))
        / NULLIF(first(open, "timestamp"),0) * 100          AS pre_market_change_pct
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
            last(close, "timestamp") AS close,
            last(high, "timestamp") AS high,
            last(low, "timestamp") AS low,
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
            close
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
            close
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
            MAX(high) AS high_15m,
            MIN(low) AS low_15m
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
            MAX(high) AS high_1h,
            MIN(low) AS low_1h
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
            AVG(volume * close) AS avg_dollar_volume_1m_14,
            AVG(volume) AS avg_volume_1m_14
        FROM (
            SELECT 
                ticker,
                last(volume, "timestamp") AS volume,
                last(close, "timestamp") AS close
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
            close
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
    last(close, "timestamp") AS current_close,
    first(open, "timestamp") AS daily_open,
    max(high) AS daily_high,
    min(low) AS daily_low,
    sum(volume) AS daily_volume,
    
    -- Basic price statistics for the day
    avg(close) AS avg_close,
    stddev_samp(close) AS daily_volatility,
    
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
-- This replaces the complex continuous aggregate with a more efficient approach
CREATE OR REPLACE FUNCTION update_historical_price_calculations(p_tickers text[])
RETURNS void
LANGUAGE plpgsql AS $$
DECLARE
    ticker_name text;
    v_current_date date := CURRENT_DATE;
BEGIN
    -- Safety exit
    IF array_length(p_tickers, 1) IS NULL THEN
        RETURN;
    END IF;

    -- Process each ticker individually for better performance
    FOREACH ticker_name IN ARRAY p_tickers
    LOOP
        INSERT INTO historical_price_calculations (
            ticker, current_close, price_1w, price_1m, price_3m, price_6m, price_1y,
            price_5y, price_10y, price_ytd, price_all, price_52w_low, price_52w_high,
            sma_50, sma_200, rsi_14, stddev_7, stddev_30, updated_at, calculation_date
        )
        SELECT 
            ticker_name,
            current_data.close,
            
            -- Historical price references using efficient lateral joins
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
            -- Get current close price
            SELECT close
            FROM ohlcv_1d
            WHERE ticker = ticker_name
            ORDER BY "timestamp" DESC
            LIMIT 1
        ) current_data
        LEFT JOIN LATERAL (
            -- 1 week ago
            SELECT close
            FROM ohlcv_1d
            WHERE ticker = ticker_name
              AND "timestamp" <= now() - INTERVAL '7 days'
            ORDER BY "timestamp" DESC
            LIMIT 1
        ) week_data ON TRUE
        LEFT JOIN LATERAL (
            -- 1 month ago
            SELECT close
            FROM ohlcv_1d
            WHERE ticker = ticker_name
              AND "timestamp" <= now() - INTERVAL '1 month'
            ORDER BY "timestamp" DESC
            LIMIT 1
        ) month_data ON TRUE
        LEFT JOIN LATERAL (
            -- 3 months ago
            SELECT close
            FROM ohlcv_1d
            WHERE ticker = ticker_name
              AND "timestamp" <= now() - INTERVAL '3 months'
            ORDER BY "timestamp" DESC
            LIMIT 1
        ) quarter_data ON TRUE
        LEFT JOIN LATERAL (
            -- 6 months ago
            SELECT close
            FROM ohlcv_1d
            WHERE ticker = ticker_name
              AND "timestamp" <= now() - INTERVAL '6 months'
            ORDER BY "timestamp" DESC
            LIMIT 1
        ) half_year_data ON TRUE
        LEFT JOIN LATERAL (
            -- 1 year ago
            SELECT close
            FROM ohlcv_1d
            WHERE ticker = ticker_name
              AND "timestamp" <= now() - INTERVAL '1 year'
            ORDER BY "timestamp" DESC
            LIMIT 1
        ) year_data ON TRUE
        LEFT JOIN LATERAL (
            -- 5 years ago
            SELECT close
            FROM ohlcv_1d
            WHERE ticker = ticker_name
              AND "timestamp" <= now() - INTERVAL '5 years'
            ORDER BY "timestamp" DESC
            LIMIT 1
        ) five_year_data ON TRUE
        LEFT JOIN LATERAL (
            -- 10 years ago
            SELECT close
            FROM ohlcv_1d
            WHERE ticker = ticker_name
              AND "timestamp" <= now() - INTERVAL '10 years'
            ORDER BY "timestamp" DESC
            LIMIT 1
        ) ten_year_data ON TRUE
        LEFT JOIN LATERAL (
            -- Year-to-date
            SELECT close
            FROM ohlcv_1d
            WHERE ticker = ticker_name
              AND EXTRACT(YEAR FROM "timestamp") = EXTRACT(YEAR FROM now())
            ORDER BY "timestamp" ASC
            LIMIT 1
        ) ytd_data ON TRUE
        LEFT JOIN LATERAL (
            -- All-time (earliest available)
            SELECT close
            FROM ohlcv_1d
            WHERE ticker = ticker_name
            ORDER BY "timestamp" ASC
            LIMIT 1
        ) all_time_data ON TRUE
        LEFT JOIN LATERAL (
            -- 52-week extremes
            SELECT 
                MIN(low) AS low_52w,
                MAX(high) AS high_52w
            FROM ohlcv_1d
            WHERE ticker = ticker_name
              AND "timestamp" >= now() - INTERVAL '52 weeks'
        ) extremes ON TRUE
        LEFT JOIN LATERAL (
            -- 50-day moving average
            SELECT AVG(close) AS avg_close
            FROM ohlcv_1d
            WHERE ticker = ticker_name
              AND "timestamp" >= now() - INTERVAL '50 days'
        ) sma_50_data ON TRUE
        LEFT JOIN LATERAL (
            -- 200-day moving average
            SELECT AVG(close) AS avg_close
            FROM ohlcv_1d
            WHERE ticker = ticker_name
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
                STDDEV_SAMP(close) FILTER (WHERE "timestamp" >= now() - INTERVAL '7 days') AS stddev_7,
                STDDEV_SAMP(close) FILTER (WHERE "timestamp" >= now() - INTERVAL '30 days') AS stddev_30
            FROM ohlcv_1d
            WHERE ticker = ticker_name
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
    END LOOP;
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
        /* ───── horizon % changes ───── */
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
    SELECT
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
        SELECT open, high, low, close, volume
        FROM   ohlcv_1d
        WHERE  ticker = sec.ticker
        ORDER  BY "timestamp" DESC
        LIMIT 1
    ) d ON TRUE

    /* ── previous day close (for 1‑day Δ) ── */
    LEFT JOIN LATERAL (
        SELECT "close" AS prev_close
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
                MAX(CASE WHEN "timestamp" <= v_now_utc - INTERVAL '1 minute' THEN "close" END) AS close_1m_ago,
                MAX(CASE WHEN "timestamp" <= v_now_utc - INTERVAL '15 minutes' THEN "close" END) AS close_15m_ago
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

	err := runScreenerLoopInit(conn)
	if err != nil {
		return fmt.Errorf("failed to setup incremental infrastructure: %v", err)
	}

	updateStaleScreenerValues(conn)

	ticker := time.NewTicker(refreshInterval)

	for {
		//now := time.Now().In(loc)
		/*if now.Hour() >= extendedCloseHour {
			log.Println("🌙 Post‑market closed — stopping incremental screener updater")
			return nil
		}*/

		select {
		case <-ticker.C:
			updateStaleScreenerValues(conn)

		}
	}
}

// SQL queries for reuse (avoid re-parsing)
var (
	updateAndRefreshSQLWithLimit = `
        WITH stale_tickers AS (
            SELECT ticker
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
        SELECT refresh_screener_delta(array(SELECT ticker FROM updated));
    `

	updateAndRefreshSQL = `
        WITH stale_tickers AS (
            SELECT ticker
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
        SELECT refresh_screener_delta(array(SELECT ticker FROM updated));
    `
)

func updateStaleScreenerValues(conn *data.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
	defer cancel()
	intervalStr := fmt.Sprintf("%d seconds", int(refreshInterval.Seconds()))

	// Use pre-defined SQL strings to avoid re-parsing the same SQL every minute
	log.Println("🔄 Updating stale screener values...")
	start := time.Now()
	if maxTickersPerBatch > 0 {
		if _, err := conn.DB.Exec(ctx, updateAndRefreshSQLWithLimit, intervalStr, maxTickersPerBatch); err != nil {
			log.Printf("❌ updateStaleScreenerValues: failed to refresh screener data: %v", err)
		}
	} else {
		if _, err := conn.DB.Exec(ctx, updateAndRefreshSQL, intervalStr); err != nil {
			log.Printf("❌ updateStaleScreenerValues: failed to refresh screener data: %v", err)
		}
	}
	log.Printf("🔄 updateStaleScreenerValues: %v", time.Since(start))
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
