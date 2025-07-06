-- Initialize PostgreSQL extensions for enhanced logging and monitoring
-- This script sets up pg_stat_statements extension

-- Create pg_stat_statements extension for query statistics
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Log the extension setup
DO $$
BEGIN
    RAISE NOTICE 'PostgreSQL extensions initialized successfully';
    RAISE NOTICE 'pg_stat_statements extension created - functions will be available after server restart';
END $$;

-- Migration: 047_ohlcv_flatfile_schema
-- Description: Create ohlcv_* tables that match Polygon flat-file column order (no securityid).

BEGIN;

-- Drop old tables if they exist (they will be rebuilt from scratch)
DROP TABLE IF EXISTS ohlcv_1m CASCADE;
DROP TABLE IF EXISTS ohlcv_1d CASCADE;
DROP TABLE IF EXISTS ohlcv_1h CASCADE;
DROP TABLE IF EXISTS ohlcv_1w CASCADE;

-- New flat-file schema: mirrors CSV header from Polygon files.
-- Column order must remain identical to the incoming files so COPY works.

CREATE TABLE ohlcv_1m (
    "ticker"        text         NOT NULL,
    "volume"        numeric,
    "open"          numeric,
    "close"         numeric,
    "high"          numeric,
    "low"           numeric,
    "timestamp"     bigint       NOT NULL,
    "transactions"  integer,
    PRIMARY KEY ("ticker", "timestamp")
);

SELECT create_hypertable(
    'ohlcv_1m',
    'timestamp',
    'ticker',
    number_partitions => 4,
    chunk_time_interval => 604800000000000,  -- 1 week in nanoseconds
    if_not_exists => TRUE
);

CREATE TABLE ohlcv_1d (
    "ticker"        text         NOT NULL,
    "volume"        numeric,
    "open"          numeric,
    "close"         numeric,
    "high"          numeric,
    "low"           numeric,
    "timestamp"     bigint       NOT NULL,
    "transactions"  integer,
    PRIMARY KEY ("ticker", "timestamp")
);

SELECT create_hypertable(
    'ohlcv_1d',
    'timestamp',
    'ticker',
    number_partitions => 4,
    chunk_time_interval => 2592000000000000,  -- 30 days in nanoseconds
    if_not_exists => TRUE
);

-- Simple index to speed common queries (ticker + time range)
CREATE INDEX IF NOT EXISTS ohlcv_1m_ticker_ts_idx ON ohlcv_1m (ticker, "timestamp");
CREATE INDEX IF NOT EXISTS ohlcv_1d_ticker_ts_idx ON ohlcv_1d (ticker, "timestamp");

-- Enable compression ---------------------------------------------------------
-- 5. Enable compression on the new flat-file OHLCV tables

ALTER TABLE ohlcv_1m SET (
    timescaledb.compress,
    timescaledb.compress_orderby = '"timestamp" DESC',
    timescaledb.compress_segmentby = 'ticker'
);

ALTER TABLE ohlcv_1d SET (
    timescaledb.compress,
    timescaledb.compress_orderby = '"timestamp" DESC',
    timescaledb.compress_segmentby = 'ticker'
);

-- Add compression policies ---------------------------------------------------
-- Hypertables use BIGINT nanosecond time, so compress_after must be an integer
-- 84 hours  = 84*3600*1e9 = 302 400 000 000 000
-- 168 hours = 168*3600*1e9 = 604 800 000 000 000
SELECT add_compression_policy('ohlcv_1m', 302400000000000);
SELECT add_compression_policy('ohlcv_1d', 604800000000000);

------------------------------------------------------------------
--  NEW: Table to track last successful OHLCV update              --
------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ohlcv_update_state (
    id               boolean PRIMARY KEY DEFAULT TRUE,
    last_loaded_at    date    NOT NULL
);

-- Seed with a starting date of 2008-01-01 if the row does not exist
INSERT INTO ohlcv_update_state (id, last_loaded_at)
VALUES (TRUE, DATE '2008-01-01')
ON CONFLICT (id) DO NOTHING;

-- Record schema version
INSERT INTO schema_versions (version, description)
VALUES (47, 'Create flat-file style OHLCV tables (ohlcv_1m / ohlcv_1d) and update tracker table')
ON CONFLICT (version) DO NOTHING;

COMMIT; 