-- Migration: 052_convert_ohlcv_timestamp_to_timestamptz (simplified)
-- Description: Drop old bigint-based OHLCV tables and recreate them with
--              timestamptz timestamp columns. No data is migrated.

BEGIN;

------------------------------------------------------------------
-- 1. Drop old tables                                              --
------------------------------------------------------------------

DROP TABLE IF EXISTS ohlcv_1m CASCADE;
DROP TABLE IF EXISTS ohlcv_1d CASCADE;

------------------------------------------------------------------
-- 2. Recreate tables with timestamptz                             --
------------------------------------------------------------------

CREATE TABLE ohlcv_1m (
    ticker        text         NOT NULL,
    volume        numeric,
    open          numeric,
    close         numeric,
    high          numeric,
    low           numeric,
    "timestamp"  timestamptz   NOT NULL,
    transactions  integer,
    PRIMARY KEY (ticker, "timestamp")
);

SELECT create_hypertable(
    'ohlcv_1m',
    'timestamp',
    'ticker',
    number_partitions => 4,
    chunk_time_interval => INTERVAL '7 days',
    if_not_exists => TRUE
);

CREATE TABLE ohlcv_1d (
    ticker        text         NOT NULL,
    volume        numeric,
    open          numeric,
    close         numeric,
    high          numeric,
    low           numeric,
    "timestamp"  timestamptz   NOT NULL,
    transactions  integer,
    PRIMARY KEY (ticker, "timestamp")
);

SELECT create_hypertable(
    'ohlcv_1d',
    'timestamp',
    'ticker',
    number_partitions => 4,
    chunk_time_interval => INTERVAL '30 days',
    if_not_exists => TRUE
);

------------------------------------------------------------------
-- 3. Secondary indexes & compression                              --
------------------------------------------------------------------

CREATE INDEX IF NOT EXISTS ohlcv_1m_ticker_ts_idx ON ohlcv_1m (ticker, "timestamp" DESC);
CREATE INDEX IF NOT EXISTS ohlcv_1d_ticker_ts_idx ON ohlcv_1d (ticker, "timestamp" DESC);

-- Enable TimescaleDB compression
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

-- Compression policies: 84h for 1m data, 168h (1 week) for 1d data
SELECT add_compression_policy('ohlcv_1m', INTERVAL '84 hours');
SELECT add_compression_policy('ohlcv_1d', INTERVAL '168 hours');

------------------------------------------------------------------
-- 4. Record schema version                                        --
------------------------------------------------------------------

INSERT INTO schema_versions (version, description)
VALUES (
    52,
    'Recreate OHLCV tables with timestamptz columns (no data migration)'
) ON CONFLICT (version) DO NOTHING;

COMMIT; 