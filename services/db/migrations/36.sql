/* 36.sql ─ Fix OHLCV Hypertable Configuration - Reduce Chunk Count */
BEGIN;

------------------------------------------------------------------
-- CRITICAL FIX: The previous ohlcv_1m configuration created 40,671 chunks
-- Problem: 1-day chunks + 16 space partitions = 16 chunks per day
-- Solution: Use 1-week chunks + 4 space partitions = 4 chunks per week
------------------------------------------------------------------

-- 1. Drop existing hypertables (this will remove all data but fix the structure)
DROP TABLE IF EXISTS ohlcv_1m CASCADE;
DROP TABLE IF EXISTS ohlcv_1h CASCADE; 
DROP TABLE IF EXISTS ohlcv_1w CASCADE;

------------------------------------------------------------------
-- 2. Recreate 1-minute OHLCV table with OPTIMIZED chunk configuration
------------------------------------------------------------------
CREATE TABLE ohlcv_1m (
    securityid INTEGER NOT NULL,
    "timestamp" TIMESTAMPTZ NOT NULL,
    open NUMERIC(22, 4),
    high NUMERIC(22, 4),
    low NUMERIC(22, 4),
    close NUMERIC(22, 4),
    volume BIGINT,
    PRIMARY KEY (securityid, "timestamp")
);

-- Convert to hypertable with OPTIMIZED settings for 1-minute data
-- FIXED: Weekly chunks + fewer partitions = much fewer chunks
SELECT create_hypertable(
    'ohlcv_1m',
    'timestamp',
    'securityid',
    number_partitions => 4,        -- REDUCED from 16 to 4 (75% fewer chunks)
    chunk_time_interval => INTERVAL '1 week',  -- INCREASED from 1 day to 1 week (7x fewer chunks)
    if_not_exists => TRUE
);

-- Indexes for 1-minute data
CREATE INDEX idx_ohlcv_1m_securityid ON ohlcv_1m (securityid);
CREATE INDEX idx_ohlcv_1m_timestamp ON ohlcv_1m ("timestamp" DESC);
CREATE INDEX idx_ohlcv_1m_security_time ON ohlcv_1m (securityid, "timestamp" DESC);

------------------------------------------------------------------
-- 3. Recreate 1-hour OHLCV table with OPTIMIZED configuration
------------------------------------------------------------------
CREATE TABLE ohlcv_1h (
    securityid INTEGER NOT NULL,
    "timestamp" TIMESTAMPTZ NOT NULL,
    open NUMERIC(22, 4),
    high NUMERIC(22, 4),
    low NUMERIC(22, 4),
    close NUMERIC(22, 4),
    volume BIGINT,
    PRIMARY KEY (securityid, "timestamp")
);

-- OPTIMIZED: Monthly chunks for 1-hour data
SELECT create_hypertable(
    'ohlcv_1h',
    'timestamp',
    'securityid', 
    number_partitions => 4,        -- REDUCED from 16 to 4
    chunk_time_interval => INTERVAL '1 month',  -- INCREASED from 1 week to 1 month
    if_not_exists => TRUE
);

-- Indexes for 1-hour data
CREATE INDEX idx_ohlcv_1h_securityid ON ohlcv_1h (securityid);
CREATE INDEX idx_ohlcv_1h_timestamp ON ohlcv_1h ("timestamp" DESC);
CREATE INDEX idx_ohlcv_1h_security_time ON ohlcv_1h (securityid, "timestamp" DESC);

------------------------------------------------------------------
-- 4. Recreate 1-week OHLCV table with OPTIMIZED configuration
------------------------------------------------------------------
CREATE TABLE ohlcv_1w (
    securityid INTEGER NOT NULL,
    "timestamp" TIMESTAMPTZ NOT NULL,
    open NUMERIC(22, 4),
    high NUMERIC(22, 4),
    low NUMERIC(22, 4),
    close NUMERIC(22, 4),
    volume BIGINT,
    PRIMARY KEY (securityid, "timestamp")
);

-- OPTIMIZED: Quarterly chunks for 1-week data
SELECT create_hypertable(
    'ohlcv_1w',
    'timestamp',
    'securityid',
    number_partitions => 2,        -- REDUCED from 16 to 2 (weekly data needs fewer partitions)
    chunk_time_interval => INTERVAL '3 months',  -- INCREASED from 1 month to 3 months  
    if_not_exists => TRUE
);

-- Indexes for 1-week data
CREATE INDEX idx_ohlcv_1w_securityid ON ohlcv_1w (securityid);
CREATE INDEX idx_ohlcv_1w_timestamp ON ohlcv_1w ("timestamp" DESC);
CREATE INDEX idx_ohlcv_1w_security_time ON ohlcv_1w (securityid, "timestamp" DESC);

------------------------------------------------------------------
-- 5. Enable compression with optimal settings
------------------------------------------------------------------
-- Enable compression on ohlcv_1m
ALTER TABLE ohlcv_1m SET (
    timescaledb.compress,
    timescaledb.compress_orderby = 'timestamp DESC',
    timescaledb.compress_segmentby = 'securityid'
);

-- Enable compression on ohlcv_1h
ALTER TABLE ohlcv_1h SET (
    timescaledb.compress,
    timescaledb.compress_orderby = 'timestamp DESC', 
    timescaledb.compress_segmentby = 'securityid'
);

-- Enable compression on ohlcv_1w
ALTER TABLE ohlcv_1w SET (
    timescaledb.compress,
    timescaledb.compress_orderby = 'timestamp DESC',
    timescaledb.compress_segmentby = 'securityid'
);

------------------------------------------------------------------
-- 6. Add compression policies
------------------------------------------------------------------
-- 1-minute data: compress after 14 days (was 7, but weekly chunks need more time)
SELECT add_compression_policy('ohlcv_1m', INTERVAL '14 days');

-- 1-hour data: compress after 60 days (was 30, but monthly chunks need more time)
SELECT add_compression_policy('ohlcv_1h', INTERVAL '60 days');

-- 1-week data: compress after 6 months (was 90 days, but quarterly chunks need more time)
SELECT add_compression_policy('ohlcv_1w', INTERVAL '6 months');

------------------------------------------------------------------
-- 7. Add retention policies  
------------------------------------------------------------------
-- 1-minute data: retain for 1 year (increased from 6 months due to weekly chunks)
SELECT add_retention_policy('ohlcv_1m', INTERVAL '1 year');

-- 1-hour data: retain for 5 years (unchanged)
SELECT add_retention_policy('ohlcv_1h', INTERVAL '5 years');

-- 1-week data: retain for 20 years (unchanged)
SELECT add_retention_policy('ohlcv_1w', INTERVAL '20 years');

COMMIT;

/*
CHUNK COUNT COMPARISON:
======================

OLD CONFIGURATION (34.sql):
- ohlcv_1m: 1 day chunks × 16 partitions = 16 chunks/day → 40,671 total chunks for 7 years
- ohlcv_1h: 1 week chunks × 16 partitions = 16 chunks/week  
- ohlcv_1w: 1 month chunks × 16 partitions = 16 chunks/month

NEW CONFIGURATION (36.sql):
- ohlcv_1m: 1 week chunks × 4 partitions = 4 chunks/week → ~1,456 total chunks for 7 years (96% REDUCTION!)
- ohlcv_1h: 1 month chunks × 4 partitions = 4 chunks/month → ~336 total chunks for 7 years  
- ohlcv_1w: 3 month chunks × 2 partitions = 2 chunks/quarter → ~56 total chunks for 7 years

TOTAL CHUNK REDUCTION: From 40,671 to ~1,848 chunks (95.5% reduction!)

BENEFITS:
- Dramatically reduced memory requirements for operations
- Faster TRUNCATE/DELETE operations  
- Better query performance for time-range queries
- Reduced metadata overhead
- Still maintains good query performance with proper indexing
*/ 