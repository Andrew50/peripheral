
-- docker exec -i dev-db-1 psql -U postgres -d postgres < services/db/migrations/combined.sql
BEGIN;

-- =========================================================
-- DROP EXISTING OBJECTS FIRST
-- =========================================================

-- Drop continuous aggregates first (they depend on functions)
-- Use CASCADE to handle any dependencies
DROP MATERIALIZED VIEW IF EXISTS mv_ohlcv_1m_latest CASCADE;
DROP MATERIALIZED VIEW IF EXISTS mv_ohlcv_1d_latest CASCADE;
-- Drop existing pre-market and extended-hours caggs to recreate them
DROP MATERIALIZED VIEW IF EXISTS cagg_pre_market CASCADE;
DROP MATERIALIZED VIEW IF EXISTS cagg_extended_hours CASCADE;
-- Drop old caggs
DROP MATERIALIZED VIEW IF EXISTS cagg_14_minute CASCADE;
DROP MATERIALIZED VIEW IF EXISTS cagg_14_day CASCADE;
DROP MATERIALIZED VIEW IF EXISTS cagg_200_day CASCADE;
DROP MATERIALIZED VIEW IF EXISTS cagg_50_day CASCADE;
DROP MATERIALIZED VIEW IF EXISTS cagg_30_day CASCADE;
DROP MATERIALIZED VIEW IF EXISTS cagg_7_day CASCADE;
DROP MATERIALIZED VIEW IF EXISTS cagg_4_hour CASCADE;
DROP MATERIALIZED VIEW IF EXISTS cagg_60_minute CASCADE;
DROP MATERIALIZED VIEW IF EXISTS cagg_15_minute CASCADE;
DROP MATERIALIZED VIEW IF EXISTS cagg_1440_minute CASCADE;

-- Now drop functions (after dependent objects are removed)
DROP FUNCTION IF EXISTS refresh_screener(integer);
DROP FUNCTION IF EXISTS refresh_static_refs();
DROP FUNCTION IF EXISTS refresh_static_refs_1m();
DROP FUNCTION IF EXISTS cleanup_static_refs_stage_tables();
DROP FUNCTION IF EXISTS safe_div(numeric, numeric);

-- Drop indexes
DROP INDEX IF EXISTS idx_static_refs_daily_prices_stage_ticker;
DROP INDEX IF EXISTS idx_static_refs_1m_prices_stage_ticker;
DROP INDEX IF EXISTS idx_static_refs_active_securities_stage_ticker;
DROP INDEX IF EXISTS idx_static_refs_daily_actives_stage_ticker;
DROP INDEX IF EXISTS idx_static_refs_1m_actives_stage_ticker;
DROP INDEX IF EXISTS idx_screener_stale_stale;
DROP INDEX IF EXISTS idx_screener_change_1d_pct;
DROP INDEX IF EXISTS idx_screener_rsi;
DROP INDEX IF EXISTS idx_screener_volume;
DROP INDEX IF EXISTS idx_screener_market_cap;
DROP INDEX IF EXISTS idx_screener_ticker;
DROP INDEX IF EXISTS idx_mv_ohlcv_1m_latest_ticker;
DROP INDEX IF EXISTS idx_mv_ohlcv_1d_latest_ticker;
DROP INDEX IF EXISTS idx_cagg_pre_market_ticker_trade_day;
DROP INDEX IF EXISTS idx_cagg_extended_hours_ticker_trade_day;

-- Drop stage tables
DROP TABLE IF EXISTS static_refs_daily_prices_stage;
DROP TABLE IF EXISTS static_refs_1m_prices_stage;
DROP TABLE IF EXISTS static_refs_active_securities_stage;
DROP TABLE IF EXISTS static_refs_daily_actives_stage;
DROP TABLE IF EXISTS static_refs_1m_actives_stage;
DROP TABLE IF EXISTS ohlcv_1d_stage;
DROP TABLE IF EXISTS ohlcv_1m_stage;

-- Drop main tables
DROP TABLE IF EXISTS screener_stale;
DROP TABLE IF EXISTS static_refs_1m;
DROP TABLE IF EXISTS static_refs_daily;

-- Continuous aggregates already dropped above

-- Remove constraint that might conflict
ALTER TABLE securities DROP CONSTRAINT IF EXISTS unique_ticker_active;

-- =========================================================
-- Combined Migration: Squashed migrations 72-78
-- Description: Complete screener system with continuous aggregates, 
--              static refs tables, and optimization features
-- =========================================================

-- Insert schema version
INSERT INTO schema_versions (version, description)
VALUES (
    78,
    'Combined screener system with static refs optimization - retired caggs except pre-market/extended-hours'
) ON CONFLICT (version) DO NOTHING;

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS timescaledb_toolkit;

-- Enable window functions in continuous aggregates
SET timescaledb.enable_cagg_window_functions = true;

-- Safe division function to handle division by zero (needed by continuous aggregates)
CREATE OR REPLACE FUNCTION safe_div(
  num numeric,
  den numeric
) RETURNS numeric
LANGUAGE SQL
IMMUTABLE
PARALLEL SAFE
STRICT
AS $$
  SELECT COALESCE(num / NULLIF(den, 0), 0);
$$;

-- =========================================================
-- CONTINUOUS AGGREGATES
-- =========================================================

-- cagg_pre_market for pre-market metrics
-- this makes sense to be a materialized view because non of the vlaues cross bucket boundaries
CREATE MATERIALIZED VIEW IF NOT EXISTS cagg_pre_market
WITH (timescaledb.continuous, timescaledb.materialized_only = false) AS
SELECT
    time_bucket('1 day', "timestamp", 'America/New_York') AS trade_day,
    ticker,

    /* ─── Pre‑market 04:00‑09:29 ─── */
    first(open/1000.0,  "timestamp")  FILTER (WHERE is_pm)
        AS pm_open,

    last(close/1000.0,  "timestamp")  FILTER (WHERE is_pm)
        AS pm_close,

    max (high /1000.0)                 FILTER (WHERE is_pm)
        AS pm_high,

    min (low  /1000.0)                 FILTER (WHERE is_pm)
        AS pm_low,

    sum (volume)                       FILTER (WHERE is_pm)
        AS pm_volume,

    sum (volume * close / 1000.0)      FILTER (WHERE is_pm)
        AS pm_dollar_volume
FROM ohlcv_1m
CROSS JOIN LATERAL (
    VALUES (("timestamp" AT TIME ZONE 'America/New_York')::time
            BETWEEN '04:00' AND '09:29:59')
) AS sess(is_pm)
WHERE "timestamp" >= now() - INTERVAL '3 days'
GROUP BY trade_day, ticker
WITH NO DATA;

SELECT add_retention_policy('cagg_pre_market',
    drop_after => INTERVAL '3 days',
    schedule_interval => INTERVAL '60 seconds');

-- Add continuous aggregate policy for cagg_pre_market (pre-market - needs frequent updates)
SELECT add_continuous_aggregate_policy('cagg_pre_market',
    start_offset => INTERVAL '2 days',
    end_offset => INTERVAL '0 minutes',
    schedule_interval => INTERVAL '5 minutes');

-- cagg_extended_hours for extended-hours metrics
-- this makes sense to be a materialized view because non of the vlaues cross bucket boundaries
CREATE MATERIALIZED VIEW IF NOT EXISTS cagg_extended_hours
WITH (timescaledb.continuous, timescaledb.materialized_only = false) AS
SELECT
    time_bucket('1 day', "timestamp", 'America/New_York') AS trade_day,
    ticker,

    /* ─── Extended‑hours 16:00‑20:00 ─── */
    first(open/1000.0,  "timestamp")  FILTER (WHERE is_eh)
        AS eh_open,

    last(close/1000.0,  "timestamp")  FILTER (WHERE is_eh)
        AS eh_close,

    max (high /1000.0)                 FILTER (WHERE is_eh)
        AS eh_high,

    min (low  /1000.0)                 FILTER (WHERE is_eh)
        AS eh_low,

    sum (volume)                       FILTER (WHERE is_eh)
        AS eh_volume,

    sum (volume * close / 1000.0)      FILTER (WHERE is_eh)
        AS eh_dollar_volume
FROM ohlcv_1m
CROSS JOIN LATERAL (
    VALUES (("timestamp" AT TIME ZONE 'America/New_York')::time
            BETWEEN '16:00' AND '20:00')
) AS sess(is_eh)
WHERE "timestamp" >= now() - INTERVAL '3 days'
GROUP BY trade_day, ticker
WITH NO DATA;

SELECT add_retention_policy('cagg_extended_hours',
    drop_after => INTERVAL '3 days',
    schedule_interval => INTERVAL '60 seconds');

-- Add continuous aggregate policy for cagg_extended_hours (extended hours - needs frequent updates)
SELECT add_continuous_aggregate_policy('cagg_extended_hours',
    start_offset => INTERVAL '2 days',
    end_offset => INTERVAL '0 minutes',
    schedule_interval => INTERVAL '5 minutes');



-- =========================================================
-- LATEST BAR MATERIALIZED VIEWS (PERFORMANCE CRITICAL)
-- =========================================================

-- Latest daily OHLCV bar per ticker (eliminates chunk scanning in refresh_screener)  
-- Note: Using recent data window to avoid full table scans - looks back 2 days to ensure latest bar coverage
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_ohlcv_1d_latest AS
SELECT DISTINCT ON (ticker)
    ticker,
    open / 1000.0 AS open,
    high / 1000.0 AS high,
    low / 1000.0 AS low,
    close / 1000.0 AS close,
    volume,
    "timestamp" AS latest_timestamp
FROM ohlcv_1d
WHERE "timestamp" >= (now() - INTERVAL '7 days')
ORDER BY ticker, "timestamp" DESC
WITH NO DATA;

-- Latest minute OHLCV bar per ticker (eliminates chunk scanning in refresh_screener)
-- Note: Using recent data window to avoid full table scans - looks back 2 days to ensure latest bar coverage
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_ohlcv_1m_latest AS  
SELECT DISTINCT ON (ticker)
    ticker,
    open / 1000.0 AS open,
    high / 1000.0 AS high,
    low / 1000.0 AS low,
    close / 1000.0 AS close,
    volume,
    "timestamp" AS latest_timestamp
FROM ohlcv_1m
WHERE "timestamp" >= (now() - INTERVAL '7 days')
ORDER BY ticker, "timestamp" DESC
WITH NO DATA;

-- =========================================================
-- STATIC REFS TABLES
-- =========================================================

-- Static refs table for daily prices
CREATE TABLE IF NOT EXISTS static_refs_daily (
    ticker text PRIMARY KEY,
    price_prev_close numeric,
    price_1d numeric,
    price_1w numeric,
    price_1m numeric,
    price_3m numeric,
    price_6m numeric,
    price_1y numeric,
    price_5y numeric,
    price_10y numeric,
    price_ytd numeric,
    price_all numeric, -- open of first day of stock
    price_52w_low numeric,
    price_52w_high numeric,
    -- New columns for daily metrics
    dma_50 numeric,
    dma_200 numeric,
    volatility_1w_pct numeric,
    volatility_1m_pct numeric,
    avg_volume_14d numeric,
    avg_dollar_volume_14d numeric,
    updated_at timestamptz DEFAULT now()
);

-- Static refs table for intraday prices
CREATE TABLE IF NOT EXISTS static_refs_1m (
    ticker text PRIMARY KEY,
    price_1m numeric,
    price_15m numeric,
    price_1h numeric,
    price_4h numeric,
    -- New columns for minute metrics
    range_15m_pct numeric,
    range_1h_pct numeric,
    avg_volume_1m_14 numeric,
    avg_dollar_volume_1m_14 numeric,
    updated_at timestamptz DEFAULT now()
);

-- =========================================================
-- SCREENER TABLES
-- =========================================================

-- Create screener_stale table
DROP TABLE IF EXISTS screener_stale;
CREATE TABLE IF NOT EXISTS screener_stale (
    ticker text PRIMARY KEY,
    stale boolean NOT NULL DEFAULT TRUE,
    last_update_time timestamptz DEFAULT now()
);

-- =========================================================
-- STAGE TABLES FOR OPTIMIZATION
-- =========================================================

-- Create unlogged staging tables
CREATE UNLOGGED TABLE IF NOT EXISTS ohlcv_1m_stage (LIKE ohlcv_1m INCLUDING DEFAULTS) WITH (autovacuum_enabled = false);
CREATE UNLOGGED TABLE IF NOT EXISTS ohlcv_1d_stage (LIKE ohlcv_1d INCLUDING DEFAULTS) WITH (autovacuum_enabled = false);

-- Persistent UNLOGGED stage tables for static refs operations - separate for each function
CREATE UNLOGGED TABLE IF NOT EXISTS static_refs_daily_actives_stage (
    ticker text NOT NULL
) WITH (autovacuum_enabled = false);

CREATE UNLOGGED TABLE IF NOT EXISTS static_refs_1m_actives_stage (
    ticker text NOT NULL
) WITH (autovacuum_enabled = false);

CREATE UNLOGGED TABLE IF NOT EXISTS static_refs_1m_prices_stage (
    ticker text NOT NULL,
    price_1m numeric,
    price_15m numeric,
    price_1h numeric,
    price_4h numeric,
    -- New columns for minute metrics
    range_15m_pct numeric,
    range_1h_pct numeric,
    avg_volume_1m_14 numeric,
    avg_dollar_volume_1m_14 numeric
) WITH (autovacuum_enabled = false);

CREATE UNLOGGED TABLE IF NOT EXISTS static_refs_daily_prices_stage (
    ticker text NOT NULL,
    price_prev_close numeric,
    price_1d numeric,
    price_1w numeric,
    price_1m numeric,
    price_3m numeric,
    price_6m numeric,
    price_1y numeric,
    price_5y numeric,
    price_10y numeric,
    price_ytd numeric,
    price_all numeric,
    price_52w_low numeric,
    price_52w_high numeric,
    -- New columns for daily metrics
    dma_50 numeric,
    dma_200 numeric,
    volatility_1w_pct numeric,
    volatility_1m_pct numeric,
    avg_volume_14d numeric,
    avg_dollar_volume_14d numeric
) WITH (autovacuum_enabled = false);



-- =========================================================
-- INDEXES
-- =========================================================

-- On continuous aggregates
CREATE INDEX IF NOT EXISTS idx_cagg_pre_market_ticker_trade_day ON cagg_pre_market(ticker, trade_day DESC);
CREATE INDEX IF NOT EXISTS idx_cagg_extended_hours_ticker_trade_day ON cagg_extended_hours(ticker, trade_day DESC);

-- On latest bar materialized views (primary key is ticker)
CREATE UNIQUE INDEX IF NOT EXISTS idx_mv_ohlcv_1d_latest_ticker ON mv_ohlcv_1d_latest(ticker);
CREATE UNIQUE INDEX IF NOT EXISTS idx_mv_ohlcv_1m_latest_ticker ON mv_ohlcv_1m_latest(ticker);

-- On screener table to speed up common queries
CREATE INDEX IF NOT EXISTS idx_screener_ticker ON screener(ticker);
CREATE INDEX IF NOT EXISTS idx_screener_market_cap ON screener(market_cap);
CREATE INDEX IF NOT EXISTS idx_screener_volume ON screener(volume);
CREATE INDEX IF NOT EXISTS idx_screener_rsi ON screener(rsi);
CREATE INDEX IF NOT EXISTS idx_screener_change_1d_pct ON screener(change_1d_pct);

-- On staleness lookup - optimized for stale ticker selection with ordering
CREATE INDEX IF NOT EXISTS idx_screener_stale_stale ON screener_stale(stale);
-- Covering index to optimize "SELECT ticker FROM screener_stale WHERE stale = TRUE ORDER BY last_update_time ASC LIMIT n"
CREATE INDEX IF NOT EXISTS idx_screener_stale_stale_update_time ON screener_stale(stale, last_update_time);
-- Alternative: partial index (smaller, more efficient for the specific query pattern)
CREATE INDEX IF NOT EXISTS idx_screener_stale_partial_update_time ON screener_stale(last_update_time) WHERE stale = TRUE;

-- Create indexes on stage tables for better performance
CREATE INDEX IF NOT EXISTS idx_static_refs_daily_actives_stage_ticker 
    ON static_refs_daily_actives_stage(ticker);
CREATE INDEX IF NOT EXISTS idx_static_refs_1m_actives_stage_ticker 
    ON static_refs_1m_actives_stage(ticker);
CREATE INDEX IF NOT EXISTS idx_static_refs_1m_prices_stage_ticker 
    ON static_refs_1m_prices_stage(ticker);
CREATE INDEX IF NOT EXISTS idx_static_refs_daily_prices_stage_ticker 
    ON static_refs_daily_prices_stage(ticker);

-- =========================================================
-- DATA CLEANUP (from migration 77)
-- =========================================================

-- Clean up duplicate securities records
WITH duplicates_to_delete AS (
    SELECT
        ctid
    FROM (
        SELECT
            ctid,
            ROW_NUMBER() OVER(PARTITION BY ticker, active ORDER BY minDate ASC, securityid ASC) as rn
        FROM
            securities
    ) ranked_rows
    WHERE ranked_rows.rn > 1
)
DELETE FROM securities
WHERE ctid IN (SELECT ctid FROM duplicates_to_delete);

-- Add unique constraint on ticker and active
ALTER TABLE securities
ADD CONSTRAINT unique_ticker_active UNIQUE (ticker, active);

-- =========================================================
-- UTILITY FUNCTIONS
-- =========================================================

-- Cleanup function for stage tables
CREATE OR REPLACE FUNCTION cleanup_static_refs_stage_tables()
RETURNS void
LANGUAGE plpgsql AS $$
BEGIN
    TRUNCATE static_refs_daily_actives_stage;
    TRUNCATE static_refs_1m_actives_stage;
    TRUNCATE static_refs_1m_prices_stage;
    TRUNCATE static_refs_daily_prices_stage;
END;
$$;

-- Function to refresh latest bar materialized views (performance critical)
CREATE OR REPLACE FUNCTION refresh_latest_bar_views()
RETURNS void
LANGUAGE plpgsql AS $$
BEGIN
    -- Refresh both latest bar views (non-concurrent to avoid index issues)
    -- These views are small (1 row per ticker) so blocking time is minimal
    REFRESH MATERIALIZED VIEW mv_ohlcv_1d_latest;
    REFRESH MATERIALIZED VIEW mv_ohlcv_1m_latest;
END;
$$;

-- =========================================================
-- OPTIMIZED REFRESH FUNCTIONS
-- =========================================================

-- Optimized function to refresh static_refs_1m using persistent stage tables
-- none of these should be materialized views becuase they are pure offset queries or they cross bucket boundaries
CREATE OR REPLACE FUNCTION refresh_static_refs_1m()
RETURNS void
LANGUAGE plpgsql AS $$
DECLARE
    v_now_utc timestamptz := now();
BEGIN
    -- Step 1: Truncate and populate 1m-specific active securities stage table
    TRUNCATE static_refs_1m_actives_stage;
    
    INSERT INTO static_refs_1m_actives_stage (ticker)
    SELECT DISTINCT s.ticker
    FROM securities s
    WHERE s.active = TRUE;

    -- Step 2: Truncate and bulk-populate 1m prices stage table
    TRUNCATE static_refs_1m_prices_stage;
    
    INSERT INTO static_refs_1m_prices_stage (
        ticker, price_1m, price_15m, price_1h, price_4h,
        range_15m_pct, range_1h_pct, avg_volume_1m_14, avg_dollar_volume_1m_14
    )
    SELECT 
        s.ticker,
        p1.close,
        p15.close,
        p60.close,
        p240.close,
        ranges.range_15m_pct,
        ranges.range_1h_pct,
        volume_avgs.avg_volume_1m_14,
        volume_avgs.avg_dollar_volume_1m_14
    FROM static_refs_1m_actives_stage s
    LEFT JOIN LATERAL (
        SELECT close / NULLIF(1000.0, 0) AS close
        FROM ohlcv_1m
        WHERE ticker = s.ticker
          AND "timestamp" >= (v_now_utc - INTERVAL '3 days')  -- Broad filter first
        ORDER BY ABS(EXTRACT(EPOCH FROM ("timestamp" - (v_now_utc - INTERVAL '1 minute')))) ASC
        LIMIT 1
    ) p1 ON TRUE
    LEFT JOIN LATERAL (
        SELECT close / NULLIF(1000.0, 0) AS close
        FROM ohlcv_1m
        WHERE ticker = s.ticker
          AND "timestamp" >= (v_now_utc - INTERVAL '3 days')  -- Broad filter first
        ORDER BY ABS(EXTRACT(EPOCH FROM ("timestamp" - (v_now_utc - INTERVAL '15 minutes')))) ASC
        LIMIT 1
    ) p15 ON TRUE
    LEFT JOIN LATERAL (
        SELECT close / NULLIF(1000.0, 0) AS close
        FROM ohlcv_1m
        WHERE ticker = s.ticker
          AND "timestamp" >= (v_now_utc - INTERVAL '3 days')  -- Broad filter first
        ORDER BY ABS(EXTRACT(EPOCH FROM ("timestamp" - (v_now_utc - INTERVAL '1 hour')))) ASC
        LIMIT 1
    ) p60 ON TRUE
    LEFT JOIN LATERAL (
        SELECT close / NULLIF(1000.0, 0) AS close
        FROM ohlcv_1m
        WHERE ticker = s.ticker
          AND "timestamp" >= (v_now_utc - INTERVAL '3 days')  -- Broad filter first
        ORDER BY ABS(EXTRACT(EPOCH FROM ("timestamp" - (v_now_utc - INTERVAL '4 hours')))) ASC
        LIMIT 1
    ) p240 ON TRUE
    LEFT JOIN LATERAL (
        -- Range calculations for 15m and 1h
        WITH recent_bars AS (
            SELECT 
                high / 1000.0 AS high,
                low / 1000.0 AS low,
                ROW_NUMBER() OVER (ORDER BY "timestamp" DESC) AS rn
            FROM ohlcv_1m
            WHERE ticker = s.ticker
              AND "timestamp" >= (v_now_utc - INTERVAL '3 days')  -- Broad filter
            ORDER BY "timestamp" DESC
            LIMIT 60  -- Max needed for 1h range
        )
        SELECT
            CASE 
                WHEN COUNT(*) FILTER (WHERE rn <= 15) < 15 THEN NULL
                WHEN MIN(low) FILTER (WHERE rn <= 15) = 0 OR MIN(low) FILTER (WHERE rn <= 15) IS NULL THEN NULL
                ELSE (MAX(high) FILTER (WHERE rn <= 15) / MIN(low) FILTER (WHERE rn <= 15) - 1) * 100
            END AS range_15m_pct,
            CASE 
                WHEN COUNT(*) FILTER (WHERE rn <= 60) < 60 THEN NULL
                WHEN MIN(low) FILTER (WHERE rn <= 60) = 0 OR MIN(low) FILTER (WHERE rn <= 60) IS NULL THEN NULL
                ELSE (MAX(high) FILTER (WHERE rn <= 60) / MIN(low) FILTER (WHERE rn <= 60) - 1) * 100
            END AS range_1h_pct
        FROM recent_bars
    ) ranges ON TRUE
    LEFT JOIN LATERAL (
        -- Volume averages for 14-period
        SELECT
            AVG(volume) AS avg_volume_1m_14,
            AVG(volume * close / 1000.0) AS avg_dollar_volume_1m_14
        FROM (
            SELECT volume, close
            FROM ohlcv_1m
            WHERE ticker = s.ticker
              AND "timestamp" >= (v_now_utc - INTERVAL '3 days')  -- Broad filter
            ORDER BY "timestamp" DESC
            LIMIT 14
        ) recent_volumes
    ) volume_avgs ON TRUE;

    -- Step 3: Bulk upsert from stage table to final table
    INSERT INTO static_refs_1m (
        ticker, price_1m, price_15m, price_1h, price_4h,
        range_15m_pct, range_1h_pct, avg_volume_1m_14, avg_dollar_volume_1m_14,
        updated_at
    )
    SELECT 
        ticker, price_1m, price_15m, price_1h, price_4h,
        range_15m_pct, range_1h_pct, avg_volume_1m_14, avg_dollar_volume_1m_14,
        v_now_utc
    FROM static_refs_1m_prices_stage
    ON CONFLICT (ticker) DO UPDATE SET
        price_1m = EXCLUDED.price_1m,
        price_15m = EXCLUDED.price_15m,
        price_1h = EXCLUDED.price_1h,
        price_4h = EXCLUDED.price_4h,
        range_15m_pct = EXCLUDED.range_15m_pct,
        range_1h_pct = EXCLUDED.range_1h_pct,
        avg_volume_1m_14 = EXCLUDED.avg_volume_1m_14,
        avg_dollar_volume_1m_14 = EXCLUDED.avg_dollar_volume_1m_14,
        updated_at = EXCLUDED.updated_at;
        
    -- Step 4: Cleanup
    TRUNCATE static_refs_1m_actives_stage;
END;
$$;

-- Optimized function to refresh static_refs using persistent stage tables
-- none of these should be materialized views becuase they are pure offset queries or they cross bucket boundaries
CREATE OR REPLACE FUNCTION refresh_static_refs()
RETURNS void
LANGUAGE plpgsql AS $$
DECLARE
    v_now_utc timestamptz := now();
BEGIN
    -- Step 1: Truncate and populate daily-specific active securities stage table
    TRUNCATE static_refs_daily_actives_stage;
    
    INSERT INTO static_refs_daily_actives_stage (ticker)
    SELECT DISTINCT s.ticker
    FROM securities s
    WHERE s.active = TRUE;

    -- Step 2: Truncate and bulk-populate daily prices stage table
    TRUNCATE static_refs_daily_prices_stage;
    
    INSERT INTO static_refs_daily_prices_stage (
        ticker, price_prev_close, price_1d, price_1w, price_1m, price_3m, price_6m, price_1y,
        price_5y, price_10y, price_ytd, price_all, price_52w_low, price_52w_high,
        dma_50, dma_200, volatility_1w_pct, volatility_1m_pct, avg_volume_14d, avg_dollar_volume_14d
    )
    SELECT
        s.ticker,
        prev_close.close,
        d1.close,
        w1.close,
        m1.close,
        m3.close,
        m6.close,
        y1.close,
        y5.close,
        y10.close,
        ytd.ytd_open,
        all_time.all_open,
        extremes.low_52w,
        extremes.high_52w,
        moving_avgs.dma_50,
        moving_avgs.dma_200,
        volatility.volatility_1w_pct,
        volatility.volatility_1m_pct,
        volume_avgs.avg_volume_14d,
        volume_avgs.avg_dollar_volume_14d
    FROM static_refs_daily_actives_stage s
    LEFT JOIN LATERAL (
        SELECT close / NULLIF(1000.0, 0) AS close
        FROM ohlcv_1d
        WHERE ticker = s.ticker
        ORDER BY "timestamp" DESC
        OFFSET 1 LIMIT 1
    ) prev_close ON TRUE
    LEFT JOIN LATERAL (
        SELECT close / NULLIF(1000.0, 0) AS close
        FROM ohlcv_1d
        WHERE ticker = s.ticker
          AND "timestamp" >= (v_now_utc - INTERVAL '2 years')  -- Broad filter for daily data
        ORDER BY ABS(EXTRACT(EPOCH FROM ("timestamp" - (v_now_utc - INTERVAL '1 day')))) ASC
        LIMIT 1
    ) d1 ON TRUE
    LEFT JOIN LATERAL (
        SELECT close / NULLIF(1000.0, 0) AS close
        FROM ohlcv_1d
        WHERE ticker = s.ticker
          AND "timestamp" >= (v_now_utc - INTERVAL '2 years')  -- Broad filter for daily data
        ORDER BY ABS(EXTRACT(EPOCH FROM ("timestamp" - (v_now_utc - INTERVAL '1 week')))) ASC
        LIMIT 1
    ) w1 ON TRUE
    LEFT JOIN LATERAL (
        SELECT close / NULLIF(1000.0, 0) AS close
        FROM ohlcv_1d
        WHERE ticker = s.ticker
          AND "timestamp" >= (v_now_utc - INTERVAL '2 years')  -- Broad filter for daily data
        ORDER BY ABS(EXTRACT(EPOCH FROM ("timestamp" - (v_now_utc - INTERVAL '1 month')))) ASC
        LIMIT 1
    ) m1 ON TRUE
    LEFT JOIN LATERAL (
        SELECT close / NULLIF(1000.0, 0) AS close
        FROM ohlcv_1d
        WHERE ticker = s.ticker
          AND "timestamp" >= (v_now_utc - INTERVAL '2 years')  -- Broad filter for daily data
        ORDER BY ABS(EXTRACT(EPOCH FROM ("timestamp" - (v_now_utc - INTERVAL '3 months')))) ASC
        LIMIT 1
    ) m3 ON TRUE
    LEFT JOIN LATERAL (
        SELECT close / NULLIF(1000.0, 0) AS close
        FROM ohlcv_1d
        WHERE ticker = s.ticker
          AND "timestamp" >= (v_now_utc - INTERVAL '2 years')  -- Broad filter for daily data
        ORDER BY ABS(EXTRACT(EPOCH FROM ("timestamp" - (v_now_utc - INTERVAL '6 months')))) ASC
        LIMIT 1
    ) m6 ON TRUE
    LEFT JOIN LATERAL (
        SELECT close / NULLIF(1000.0, 0) AS close
        FROM ohlcv_1d
        WHERE ticker = s.ticker
          AND "timestamp" >= (v_now_utc - INTERVAL '12 years')  -- Broader filter for longer periods
        ORDER BY ABS(EXTRACT(EPOCH FROM ("timestamp" - (v_now_utc - INTERVAL '1 year')))) ASC
        LIMIT 1
    ) y1 ON TRUE
    LEFT JOIN LATERAL (
        SELECT close / NULLIF(1000.0, 0) AS close
        FROM ohlcv_1d
        WHERE ticker = s.ticker
          AND "timestamp" >= (v_now_utc - INTERVAL '12 years')  -- Broader filter for longer periods
        ORDER BY ABS(EXTRACT(EPOCH FROM ("timestamp" - (v_now_utc - INTERVAL '5 years')))) ASC
        LIMIT 1
    ) y5 ON TRUE
    LEFT JOIN LATERAL (
        SELECT close / NULLIF(1000.0, 0) AS close
        FROM ohlcv_1d
        WHERE ticker = s.ticker
          AND "timestamp" >= (v_now_utc - INTERVAL '12 years')  -- Broader filter for longer periods
        ORDER BY ABS(EXTRACT(EPOCH FROM ("timestamp" - (v_now_utc - INTERVAL '10 years')))) ASC
        LIMIT 1
    ) y10 ON TRUE
    LEFT JOIN LATERAL (
        SELECT open / NULLIF(1000.0, 0) AS ytd_open
        FROM ohlcv_1d
        WHERE ticker = s.ticker AND date_trunc('year', "timestamp" AT TIME ZONE 'America/New_York') = date_trunc('year', v_now_utc AT TIME ZONE 'America/New_York')
        ORDER BY ABS(EXTRACT(EPOCH FROM ("timestamp" - date_trunc('year', v_now_utc AT TIME ZONE 'America/New_York')))) ASC
        LIMIT 1
    ) ytd ON TRUE
    LEFT JOIN LATERAL (
        SELECT open / NULLIF(1000.0, 0) AS all_open
        FROM ohlcv_1d
        WHERE ticker = s.ticker
        -- No time filter - need the very first candle ever
        ORDER BY "timestamp" ASC LIMIT 1
    ) all_time ON TRUE
    LEFT JOIN LATERAL (
        SELECT MAX(high / NULLIF(1000.0, 0)) AS high_52w, MIN(low / NULLIF(1000.0, 0)) AS low_52w
        FROM ohlcv_1d
        WHERE ticker = s.ticker 
        AND "timestamp" >= v_now_utc - INTERVAL '52 weeks'  -- Exactly 52 weeks for highs/lows
    ) extremes ON TRUE
    LEFT JOIN LATERAL (
        -- Moving averages using window function for efficiency
        WITH daily_prices AS (
            SELECT close / 1000.0 AS close
            FROM ohlcv_1d
            WHERE ticker = s.ticker
              AND "timestamp" >= (v_now_utc - INTERVAL '2 years')  -- Broad filter
            ORDER BY "timestamp" DESC
            LIMIT 200  -- Max needed for 200-DMA
        )
        SELECT
            AVG(close) FILTER (WHERE rownum <= 50) AS dma_50,
            AVG(close) FILTER (WHERE rownum <= 200) AS dma_200
        FROM (
            SELECT close, ROW_NUMBER() OVER () AS rownum
            FROM daily_prices
        ) numbered
    ) moving_avgs ON TRUE
    LEFT JOIN LATERAL (
        -- Volatility calculations
        WITH recent_closes AS (
            SELECT 
                close / 1000.0 AS close,
                ROW_NUMBER() OVER (ORDER BY "timestamp" DESC) AS rn
            FROM ohlcv_1d
            WHERE ticker = s.ticker
              AND "timestamp" >= (v_now_utc - INTERVAL '90 days')  -- Broad filter
            ORDER BY "timestamp" DESC
            LIMIT 30  -- Max needed for 30-day volatility
        )
        SELECT
            CASE 
                WHEN COUNT(*) FILTER (WHERE rn <= 7) < 7 THEN NULL
                WHEN AVG(close) FILTER (WHERE rn <= 7) = 0 THEN NULL
                ELSE (STDDEV_SAMP(close) FILTER (WHERE rn <= 7) / AVG(close) FILTER (WHERE rn <= 7)) * 100
            END AS volatility_1w_pct,
            CASE 
                WHEN COUNT(*) FILTER (WHERE rn <= 30) < 30 THEN NULL
                WHEN AVG(close) FILTER (WHERE rn <= 30) = 0 THEN NULL
                ELSE (STDDEV_SAMP(close) FILTER (WHERE rn <= 30) / AVG(close) FILTER (WHERE rn <= 30)) * 100
            END AS volatility_1m_pct
        FROM recent_closes
    ) volatility ON TRUE
    LEFT JOIN LATERAL (
        -- Volume averages
        SELECT
            AVG(volume) AS avg_volume_14d,
            AVG(volume * close / 1000.0) AS avg_dollar_volume_14d
        FROM (
            SELECT volume, close
            FROM ohlcv_1d
            WHERE ticker = s.ticker
              AND "timestamp" >= (v_now_utc - INTERVAL '30 days')  -- Broad filter
            ORDER BY "timestamp" DESC
            LIMIT 14
        ) recent_volumes
    ) volume_avgs ON TRUE;

    -- Step 3: Bulk upsert from stage table to final table
    INSERT INTO static_refs_daily (
        ticker, price_prev_close, price_1d, price_1w, price_1m, price_3m, price_6m, price_1y,
        price_5y, price_10y, price_ytd, price_all, price_52w_low, price_52w_high,
        dma_50, dma_200, volatility_1w_pct, volatility_1m_pct, avg_volume_14d, avg_dollar_volume_14d,
        updated_at
    )
    SELECT 
        ticker, price_prev_close, price_1d, price_1w, price_1m, price_3m, price_6m, price_1y,
        price_5y, price_10y, price_ytd, price_all, price_52w_low, price_52w_high,
        dma_50, dma_200, volatility_1w_pct, volatility_1m_pct, avg_volume_14d, avg_dollar_volume_14d,
        v_now_utc
    FROM static_refs_daily_prices_stage
    ON CONFLICT (ticker) DO UPDATE SET
        price_prev_close = EXCLUDED.price_prev_close,
        price_1d = EXCLUDED.price_1d,
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
        dma_50 = EXCLUDED.dma_50,
        dma_200 = EXCLUDED.dma_200,
        volatility_1w_pct = EXCLUDED.volatility_1w_pct,
        volatility_1m_pct = EXCLUDED.volatility_1m_pct,
        avg_volume_14d = EXCLUDED.avg_volume_14d,
        avg_dollar_volume_14d = EXCLUDED.avg_dollar_volume_14d,
        updated_at = EXCLUDED.updated_at;
        
    -- Step 4: Cleanup
    TRUNCATE static_refs_daily_actives_stage;
END;
$$;

-- =========================================================
-- MAIN SCREENER REFRESH FUNCTION 
-- =========================================================

-- function that combines helper tables / caggs / materialized views to refresh the screener, should contain only simple operations for maximum refresh speed

CREATE OR REPLACE FUNCTION refresh_screener(p_limit integer)
RETURNS VOID
LANGUAGE plpgsql AS $$
DECLARE
    v_now timestamptz := now();
    stale_ticker_count integer;
    inserted_rows_count integer;
BEGIN
    -- Bulk refresh in a single statement
    WITH stale_tickers AS (
        SELECT ticker
        FROM screener_stale
        WHERE stale = TRUE
        ORDER BY last_update_time ASC
        LIMIT p_limit
        FOR UPDATE SKIP LOCKED
    ),
    -- Store the tickers for later UPDATE
    processed_tickers AS (
        SELECT ticker FROM stale_tickers
    ),
    logged_stale_tickers AS (
        SELECT t.*,
               (SELECT count(*) FROM stale_tickers) as total_count
        FROM stale_tickers t
    ),
    latest_daily AS (
        SELECT
            st.ticker,
            cd.open,
            cd.high,
            cd.low,
            cd.close,
            cd.volume
        FROM logged_stale_tickers st
        LEFT JOIN mv_ohlcv_1d_latest cd ON cd.ticker = st.ticker
    ),
    latest_minute AS (
        SELECT
            st.ticker,
            cm.open AS m_open,
            cm.high AS m_high,
            cm.low AS m_low,
            cm.close AS m_close,
            cm.volume AS m_volume
        FROM logged_stale_tickers st
        LEFT JOIN mv_ohlcv_1m_latest cm ON cm.ticker = st.ticker
    ),
    security_info AS (
        SELECT DISTINCT ON (st.ticker)
            st.ticker,
            s.securityid,
            s.market_cap,
            s.sector,
            s.industry
        FROM logged_stale_tickers st
        LEFT JOIN securities s ON s.ticker = st.ticker AND s.active = true
        ORDER BY st.ticker, s.securityid DESC
    ),
    cagg_data AS (
        SELECT
            st.ticker,
            cpm.pm_open AS pre_market_open,
            cpm.pm_close AS pre_market_close,
            cpm.pm_high AS pre_market_high,
            cpm.pm_low AS pre_market_low,
            cpm.pm_volume AS pre_market_volume,
            cpm.pm_dollar_volume AS pre_market_dollar_volume,
            ceh.eh_open AS extended_open,
            ceh.eh_close AS extended_close,
            ceh.eh_high AS extended_high,
            ceh.eh_low AS extended_low,
            ceh.eh_volume AS extended_volume,
            ceh.eh_dollar_volume AS extended_dollar_volume,
            -- From static_refs_1m
            sr1m.range_15m_pct,
            sr1m.range_1h_pct,
            sr1m.price_4h AS c4h_close,
            sr1m.avg_volume_1m_14,
            sr1m.avg_dollar_volume_1m_14,
            -- From static_refs_daily
            srd.volatility_1w_pct,
            srd.volatility_1m_pct,
            srd.dma_50,
            srd.dma_200,
            srd.avg_volume_14d,
            srd.avg_dollar_volume_14d
        FROM logged_stale_tickers st
        LEFT JOIN LATERAL (
            SELECT *
            FROM cagg_pre_market
            WHERE ticker = st.ticker
            ORDER BY trade_day DESC
            LIMIT 1
        ) cpm ON TRUE
        LEFT JOIN LATERAL (
            SELECT *
            FROM cagg_extended_hours
            WHERE ticker = st.ticker
            ORDER BY trade_day DESC
            LIMIT 1
        ) ceh ON TRUE
        LEFT JOIN static_refs_1m sr1m ON sr1m.ticker = st.ticker
        LEFT JOIN static_refs_daily srd ON srd.ticker = st.ticker
    ),
    rsi_calc AS (
        SELECT
            st.ticker,
            CASE
                WHEN avg_gain IS NULL AND avg_loss IS NULL THEN NULL          -- not enough data
                WHEN avg_loss = 0 THEN 100                                    -- all gains / flat up
                WHEN avg_gain IS NULL THEN 0                                  -- all losses / flat down
                ELSE 100 - (100 / (1 + safe_div(avg_gain, avg_loss)))        -- RSI formula (SMA seed)
            END AS rsi_14
        FROM logged_stale_tickers st
        LEFT JOIN LATERAL (
            WITH last15 AS (
                SELECT close/1000.0 AS c, timestamp
                FROM ohlcv_1d
                WHERE ticker = st.ticker 
                  AND "timestamp" >= (now() - INTERVAL '60 days')            -- Broader filter for RSI calculation
                ORDER BY timestamp DESC      -- grab most recent 15 daily bars
                LIMIT 15
            ),
            chron AS (
                SELECT c, timestamp
                FROM last15
                ORDER BY timestamp            -- oldest→newest so LAG works chronologically
            )
            SELECT
                avg(gain) AS avg_gain,
                avg(loss) AS avg_loss
            FROM (
                SELECT
                    COALESCE(GREATEST(c - LAG(c) OVER w, 0), 0) AS gain,
                    COALESCE(GREATEST(LAG(c) OVER w - c, 0), 0) AS loss
                FROM chron
                WINDOW w AS (ORDER BY timestamp)
            ) diffs
        ) r ON TRUE
    ),
    historical_prices AS (
        SELECT 
            st.ticker,
            srd.price_prev_close,
            srd.price_1d,
            srd.price_1w,
            srd.price_1m,
            srd.price_3m,
            srd.price_6m,
            srd.price_1y,
            srd.price_5y,
            srd.price_10y,
            srd.price_ytd,
            srd.price_all,
            srd.price_52w_low,
            srd.price_52w_high
        FROM logged_stale_tickers st
        LEFT JOIN static_refs_daily srd ON srd.ticker = st.ticker
    ),
    intraday_prices AS (
        SELECT
            st.ticker,
            sr1m.price_1m AS price_1m_min,
            sr1m.price_15m AS price_15m_min,
            sr1m.price_1h AS price_1h_min,
            sr1m.price_4h AS price_4h_min
        FROM logged_stale_tickers st
        LEFT JOIN static_refs_1m sr1m ON sr1m.ticker = st.ticker
    ),
    market_close_data AS (
        SELECT
            st.ticker,
            mc.close AS market_close
        FROM logged_stale_tickers st
        LEFT JOIN LATERAL (
            SELECT close / 1000.0 AS close
            FROM ohlcv_1m
            WHERE ticker = st.ticker
            AND "timestamp" >= (v_now - INTERVAL '2 days')  -- Limit to recent data first
            AND (timestamp AT TIME ZONE 'America/New_York')::time = '16:00'
            AND timestamp::date = v_now::date
            ORDER BY timestamp DESC LIMIT 1
        ) mc ON TRUE
    ),
    avg_volumes AS (
        SELECT
            st.ticker,
            avg(o.volume) AS avg_volume_1m,
            avg(o.volume * o.close) AS avg_dollar_volume_1m
        FROM logged_stale_tickers st
        LEFT JOIN LATERAL (
            SELECT volume, close / 1000.0 AS close
            FROM ohlcv_1d
            WHERE ticker = st.ticker
            AND "timestamp" >= (v_now - INTERVAL '60 days')  -- Broad filter for volume calculations
            ORDER BY timestamp DESC
            LIMIT 30
        ) o ON TRUE
        GROUP BY st.ticker
    ),
    spy_metrics AS (
        SELECT
            o.close / 1000.0 AS spy_ld_close,
            (SELECT close / 1000.0 FROM ohlcv_1d WHERE ticker = 'SPY' AND "timestamp" >= (v_now - INTERVAL '2 years') ORDER BY ABS(EXTRACT(EPOCH FROM ("timestamp" - (v_now - INTERVAL '1 month')))) ASC LIMIT 1) AS spy_price_1m,
            (SELECT close / 1000.0 FROM ohlcv_1d WHERE ticker = 'SPY' AND "timestamp" >= (v_now - INTERVAL '2 years') ORDER BY ABS(EXTRACT(EPOCH FROM ("timestamp" - (v_now - INTERVAL '1 year')))) ASC LIMIT 1) AS spy_price_1y
        FROM ohlcv_1d o
        WHERE o.ticker = 'SPY'
          AND o."timestamp" >= (v_now - INTERVAL '7 days')  -- Only need recent SPY close
        ORDER BY o.timestamp DESC
        LIMIT 1
    ),
    computed_metrics AS (
        SELECT
            si.ticker,
            v_now AS calc_time,
            si.securityid AS security_id,
            ld.open,
            ld.high,
            ld.low,
            ld.close,
            hp.price_52w_low AS wk52_low,
            hp.price_52w_high AS wk52_high,
            cd.pre_market_open,
            cd.pre_market_high,
            cd.pre_market_low,
            cd.pre_market_close,
            si.market_cap,
            si.sector,
            si.industry,
            (cd.pre_market_close - cd.pre_market_open) AS pre_market_change,
            CASE WHEN cd.pre_market_open = 0 OR cd.pre_market_open IS NULL THEN NULL ELSE ((cd.pre_market_close - cd.pre_market_open) / cd.pre_market_open) * 100 END AS pre_market_change_pct,
            CASE WHEN (v_now AT TIME ZONE 'America/New_York')::time BETWEEN '16:00' AND '20:00' THEN lm.m_close - mcd.market_close ELSE NULL END AS extended_hours_change,
            CASE WHEN (v_now AT TIME ZONE 'America/New_York')::time BETWEEN '16:00' AND '20:00' AND mcd.market_close != 0 AND mcd.market_close IS NOT NULL THEN ((lm.m_close - mcd.market_close) / mcd.market_close) * 100 ELSE NULL END AS extended_hours_change_pct,
            CASE WHEN ip.price_1m_min = 0 OR ip.price_1m_min IS NULL THEN NULL ELSE (lm.m_close - ip.price_1m_min) / ip.price_1m_min * 100 END AS change_1_pct,
            CASE WHEN ip.price_15m_min = 0 OR ip.price_15m_min IS NULL THEN NULL ELSE (lm.m_close - ip.price_15m_min) / ip.price_15m_min * 100 END AS change_15_pct,
            CASE WHEN ip.price_1h_min = 0 OR ip.price_1h_min IS NULL THEN NULL ELSE (lm.m_close - ip.price_1h_min) / ip.price_1h_min * 100 END AS change_1h_pct,
            CASE WHEN ip.price_4h_min = 0 OR ip.price_4h_min IS NULL THEN NULL ELSE (lm.m_close - ip.price_4h_min) / ip.price_4h_min * 100 END AS change_4h_pct,
            CASE WHEN hp.price_1d = 0 OR hp.price_1d IS NULL THEN NULL ELSE (ld.close - hp.price_1d) / hp.price_1d * 100 END AS change_1d_pct,
            CASE WHEN hp.price_1w = 0 OR hp.price_1w IS NULL THEN NULL ELSE (ld.close - hp.price_1w) / hp.price_1w * 100 END AS change_1w_pct,
            CASE WHEN hp.price_1m = 0 OR hp.price_1m IS NULL THEN NULL ELSE (ld.close - hp.price_1m) / hp.price_1m * 100 END AS change_1m_pct,
            CASE WHEN hp.price_3m = 0 OR hp.price_3m IS NULL THEN NULL ELSE (ld.close - hp.price_3m) / hp.price_3m * 100 END AS change_3m_pct,
            CASE WHEN hp.price_6m = 0 OR hp.price_6m IS NULL THEN NULL ELSE (ld.close - hp.price_6m) / hp.price_6m * 100 END AS change_6m_pct,
            CASE WHEN hp.price_ytd = 0 OR hp.price_ytd IS NULL THEN NULL ELSE (ld.close - hp.price_ytd) / hp.price_ytd * 100 END AS change_ytd_pct,
            CASE WHEN hp.price_1y = 0 OR hp.price_1y IS NULL THEN NULL ELSE (ld.close - hp.price_1y) / hp.price_1y * 100 END AS change_1y_pct,
            CASE WHEN hp.price_5y = 0 OR hp.price_5y IS NULL THEN NULL ELSE (ld.close - hp.price_5y) / hp.price_5y * 100 END AS change_5y_pct,
            CASE WHEN hp.price_10y = 0 OR hp.price_10y IS NULL THEN NULL ELSE (ld.close - hp.price_10y) / hp.price_10y * 100 END AS change_10y_pct,
            CASE WHEN hp.price_all = 0 OR hp.price_all IS NULL THEN NULL ELSE (ld.close - hp.price_all) / hp.price_all * 100 END AS change_all_time_pct,
            (ld.close - ld.open) AS change_from_open,
            CASE WHEN ld.open = 0 OR ld.open IS NULL THEN NULL ELSE ((ld.close - ld.open) / ld.open) * 100 END AS change_from_open_pct,
            CASE WHEN hp.price_52w_high = 0 OR hp.price_52w_high IS NULL THEN NULL ELSE ld.close / hp.price_52w_high * 100 END AS price_over_52wk_high,
            CASE WHEN hp.price_52w_low = 0 OR hp.price_52w_low IS NULL THEN NULL ELSE ld.close / hp.price_52w_low * 100 END AS price_over_52wk_low,
            rc.rsi_14 AS rsi,
            cd.dma_200,
            cd.dma_50,
            CASE WHEN cd.dma_50 = 0 OR cd.dma_50 IS NULL THEN NULL ELSE ld.close / cd.dma_50 * 100 END AS price_over_50dma,
            CASE WHEN cd.dma_200 = 0 OR cd.dma_200 IS NULL THEN NULL ELSE ld.close / cd.dma_200 * 100 END AS price_over_200dma,
            CASE 
                WHEN spy.spy_price_1y = 0 OR spy.spy_price_1y IS NULL OR hp.price_1y = 0 OR hp.price_1y IS NULL THEN NULL 
                WHEN spy.spy_ld_close = 0 OR spy.spy_ld_close IS NULL THEN NULL
                ELSE safe_div(safe_div(ld.close - hp.price_1y, hp.price_1y), safe_div(spy.spy_ld_close - spy.spy_price_1y, spy.spy_price_1y))
            END AS beta_1y_vs_spy,
            CASE 
                WHEN spy.spy_price_1m = 0 OR spy.spy_price_1m IS NULL OR hp.price_1m = 0 OR hp.price_1m IS NULL THEN NULL 
                WHEN spy.spy_ld_close = 0 OR spy.spy_ld_close IS NULL THEN NULL
                ELSE safe_div(safe_div(ld.close - hp.price_1m, hp.price_1m), safe_div(spy.spy_ld_close - spy.spy_price_1m, spy.spy_price_1m))
            END AS beta_1m_vs_spy,
            ld.volume,
            av.avg_volume_1m,
            CASE WHEN ld.close IS NULL OR ld.volume IS NULL THEN NULL ELSE ld.close * ld.volume END AS dollar_volume,
            av.avg_dollar_volume_1m,
            cd.pre_market_volume,
            cd.pre_market_dollar_volume,
            CASE WHEN cd.avg_volume_1m_14 = 0 OR cd.avg_volume_1m_14 IS NULL THEN NULL ELSE lm.m_volume / cd.avg_volume_1m_14 END AS relative_volume_14,
            CASE WHEN cd.avg_volume_14d = 0 OR cd.avg_volume_14d IS NULL THEN NULL ELSE cd.pre_market_volume / cd.avg_volume_14d END AS pre_market_vol_over_14d_vol,
            CASE WHEN lm.m_low = 0 OR lm.m_low IS NULL THEN NULL ELSE (lm.m_high - lm.m_low) / lm.m_low * 100 END AS range_1m_pct,
            cd.range_15m_pct,
            cd.range_1h_pct,
            CASE WHEN ld.low = 0 OR ld.low IS NULL THEN NULL ELSE (ld.high - ld.low) / ld.low * 100 END AS day_range_pct,
            cd.volatility_1w_pct,
            cd.volatility_1m_pct,
            CASE WHEN cd.pre_market_low = 0 OR cd.pre_market_low IS NULL THEN NULL ELSE (cd.pre_market_high - cd.pre_market_low) / cd.pre_market_low * 100 END AS pre_market_range_pct
        FROM security_info si
        JOIN latest_daily ld ON ld.ticker = si.ticker
        JOIN latest_minute lm ON lm.ticker = si.ticker
        JOIN cagg_data cd ON cd.ticker = si.ticker
        JOIN rsi_calc rc ON rc.ticker = si.ticker
        JOIN historical_prices hp ON hp.ticker = si.ticker
        JOIN intraday_prices ip ON ip.ticker = si.ticker
        JOIN market_close_data mcd ON mcd.ticker = si.ticker
        JOIN avg_volumes av ON av.ticker = si.ticker
        CROSS JOIN spy_metrics spy
        WHERE ld.close IS NOT NULL AND lm.m_close IS NOT NULL  -- Skip if no data
    ),
    inserted AS (
        INSERT INTO screener (
            ticker, calc_time, security_id, open, high, low, close, wk52_low, wk52_high,
            pre_market_open, pre_market_high, pre_market_low, pre_market_close,
            market_cap, sector, industry,
            pre_market_change, pre_market_change_pct, extended_hours_change, extended_hours_change_pct,
            change_1_pct, change_15_pct, change_1h_pct, change_4h_pct,
            change_1d_pct, change_1w_pct, change_1m_pct, change_3m_pct, change_6m_pct,
            change_ytd_pct, change_1y_pct, change_5y_pct, change_10y_pct, change_all_time_pct,
            change_from_open, change_from_open_pct, price_over_52wk_high, price_over_52wk_low,
            rsi, dma_200, dma_50, price_over_50dma, price_over_200dma,
            beta_1y_vs_spy, beta_1m_vs_spy,
            volume, avg_volume_1m, dollar_volume, avg_dollar_volume_1m,
            pre_market_volume, pre_market_dollar_volume, relative_volume_14, pre_market_vol_over_14d_vol,
            range_1m_pct, range_15m_pct, range_1h_pct, day_range_pct,
            volatility_1w_pct, volatility_1m_pct, pre_market_range_pct
        )
        SELECT *
        FROM computed_metrics
        ON CONFLICT (ticker) DO UPDATE SET
            calc_time = EXCLUDED.calc_time,
            security_id = EXCLUDED.security_id,
            open = EXCLUDED.open,
            high = EXCLUDED.high,
            low = EXCLUDED.low,
            close = EXCLUDED.close,
            wk52_low = EXCLUDED.wk52_low,
            wk52_high = EXCLUDED.wk52_high,
            pre_market_open = EXCLUDED.pre_market_open,
            pre_market_high = EXCLUDED.pre_market_high,
            pre_market_low = EXCLUDED.pre_market_low,
            pre_market_close = EXCLUDED.pre_market_close,
            market_cap = EXCLUDED.market_cap,
            sector = EXCLUDED.sector,
            industry = EXCLUDED.industry,
            pre_market_change = EXCLUDED.pre_market_change,
            pre_market_change_pct = EXCLUDED.pre_market_change_pct,
            extended_hours_change = EXCLUDED.extended_hours_change,
            extended_hours_change_pct = EXCLUDED.extended_hours_change_pct,
            change_1_pct = EXCLUDED.change_1_pct,
            change_15_pct = EXCLUDED.change_15_pct,
            change_1h_pct = EXCLUDED.change_1h_pct,
            change_4h_pct = EXCLUDED.change_4h_pct,
            change_1d_pct = EXCLUDED.change_1d_pct,
            change_1w_pct = EXCLUDED.change_1w_pct,
            change_1m_pct = EXCLUDED.change_1m_pct,
            change_3m_pct = EXCLUDED.change_3m_pct,
            change_6m_pct = EXCLUDED.change_6m_pct,
            change_ytd_pct = EXCLUDED.change_ytd_pct,
            change_1y_pct = EXCLUDED.change_1y_pct,
            change_5y_pct = EXCLUDED.change_5y_pct,
            change_10y_pct = EXCLUDED.change_10y_pct,
            change_all_time_pct = EXCLUDED.change_all_time_pct,
            change_from_open = EXCLUDED.change_from_open,
            change_from_open_pct = EXCLUDED.change_from_open_pct,
            price_over_52wk_high = EXCLUDED.price_over_52wk_high,
            price_over_52wk_low = EXCLUDED.price_over_52wk_low,
            rsi = EXCLUDED.rsi,
            dma_200 = EXCLUDED.dma_200,
            dma_50 = EXCLUDED.dma_50,
            price_over_50dma = EXCLUDED.price_over_50dma,
            price_over_200dma = EXCLUDED.price_over_200dma,
            beta_1y_vs_spy = EXCLUDED.beta_1y_vs_spy,
            beta_1m_vs_spy = EXCLUDED.beta_1m_vs_spy,
            volume = EXCLUDED.volume,
            avg_volume_1m = EXCLUDED.avg_volume_1m,
            dollar_volume = EXCLUDED.dollar_volume,
            avg_dollar_volume_1m = EXCLUDED.avg_dollar_volume_1m,
            pre_market_volume = EXCLUDED.pre_market_volume,
            pre_market_dollar_volume = EXCLUDED.pre_market_dollar_volume,
            relative_volume_14 = EXCLUDED.relative_volume_14,
            pre_market_vol_over_14d_vol = EXCLUDED.pre_market_vol_over_14d_vol,
            range_1m_pct = EXCLUDED.range_1m_pct,
            range_15m_pct = EXCLUDED.range_15m_pct,
            range_1h_pct = EXCLUDED.range_1h_pct,
            day_range_pct = EXCLUDED.day_range_pct,
            volatility_1w_pct = EXCLUDED.volatility_1w_pct,
            volatility_1m_pct = EXCLUDED.volatility_1m_pct,
            pre_market_range_pct = EXCLUDED.pre_market_range_pct
        RETURNING 1
    ),
    counts AS (
        SELECT 
            (SELECT count(*) FROM logged_stale_tickers) as stale_count,
            (SELECT count(*) FROM inserted) as inserted_count
    ),
    -- Mark the processed tickers as fresh within the same transaction
    mark_fresh AS (
        UPDATE screener_stale ss
        SET stale = FALSE,
            last_update_time = v_now
        FROM processed_tickers pt
        WHERE ss.ticker = pt.ticker
        RETURNING 1
    )
    SELECT stale_count, inserted_count
    INTO stale_ticker_count, inserted_rows_count
    FROM counts;

    RAISE NOTICE 'refresh_screener: Found % stale tickers, inserted % rows into screener.', stale_ticker_count, inserted_rows_count;

END;
$$;

INSERT INTO schema_versions (version, description)
VALUES (78, 'Combined screener system with static refs optimization - retired caggs except pre-market/extended-hours')
ON CONFLICT (version) DO UPDATE SET description = EXCLUDED.description;

COMMIT;
