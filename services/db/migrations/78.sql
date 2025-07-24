-- Migration: 071_ohlcv_bigint_timestamptz_schema
-- Description: Recreate OHLCV tables with bigint price columns (4 decimal places offset) and timestamptz timestamps

/*
Performance tuning notes:
- worker_mem 1gb, total mem 8gb, 2gb chunk size target
- 6400 days of ohlcv_1d is 2700mb
- 2300 days of ohlcv_1m is 40gb
- num_partitions is 4 to match cpu count
- 4 month per 1m chunk
- No time chunking for ohlcv_1d (single chunk for all data)

Price storage format:
- OHLCV prices (open, high, low, close) are stored as BIGINT with 4 decimal places offset
- Example: $123.4567 is stored as 1234567 (multiply by 10000)
- Volume is stored as BIGINT with no decimal offset (normal integer)
*/

/*

worke rmem 1gb total mem 8gb, 2gb chunk size target,
6400 days of ohlcv_1d is 2700mb
2300 days of ohlcv_1m is 40gb

num partitions is 4 to match cpu count

2300 * (2000 / 40000) = 115 days per 1m chunk

#16 yeras per 1d chunk (increase to fit all data which is like 20 yeras, just dont chunk by time

6400 * (2000 / 2700) = 4741 days per 1d chunk
#4 month per 1m chunk

work mem should be 1gb so that we can fit one chunks in memoery for each worker (2gb chunk / 4 partitions = 512mb per partition)
3  Other knobs worth double-checking
Parameter       Why it matters    Quick rule of thumb
shared_buffers  Postgres page cache; 25–40 % RAM is safe
Timescale       2 GiB is fine on 8 GiB
maintenance_work_mem   VACUUM / index build; keep higher than work_mem
Timescale       512 MiB–1 GiB
timescaledb.max_background_workers  One per CPU plus compression jobs     8 matches your config
max_locks_per_transaction    Must exceed 2 × max chunks you touch in one txn
Timescale

*/

ALTER EXTENSION timescaledb UPDATE;

BEGIN;

-- force-disconnect all other sessions on this database
DO $$
BEGIN
  PERFORM pg_terminate_backend(pid)
  FROM pg_stat_activity
  WHERE pid <> pg_backend_pid()
    AND datname = current_database();
END
$$;

-- Create temporary backup tables to preserve existing data
/*CREATE TABLE IF NOT EXISTS ohlcv_1m_backup AS 
SELECT * FROM ohlcv_1m LIMIT 0;*/

-- Drop existing tables and recreate with new schema
SELECT remove_compression_policy('ohlcv_1m');
SELECT remove_compression_policy('ohlcv_1d');
DROP TABLE IF EXISTS ohlcv_1m CASCADE;
DROP TABLE IF EXISTS ohlcv_1d CASCADE;
DROP TABLE IF EXISTS ohlcv_update_state CASCADE;

-- Create new ohlcv_1m table with bigint prices and timestamptz
CREATE TABLE ohlcv_1m (
    "ticker"        text         NOT NULL,
    "volume"        bigint,                    -- Volume as normal integer (no decimal offset)
    "open"          bigint,                    -- Price * 10000 (4 decimal places offset)
    "close"         bigint,                    -- Price * 10000 (4 decimal places offset)
    "high"          bigint,                    -- Price * 10000 (4 decimal places offset)
    "low"           bigint,                    -- Price * 10000 (4 decimal places offset)
    "timestamp"     timestamptz  NOT NULL,
    "transactions"  integer,
    PRIMARY KEY ("ticker", "timestamp")
);
-- Add column comments for decimal offset storage on ohlcv_1m
COMMENT ON COLUMN ohlcv_1m.open IS 'Price * 10000 (4 decimal places offset)';
COMMENT ON COLUMN ohlcv_1m.high IS 'Price * 10000 (4 decimal places offset)';
COMMENT ON COLUMN ohlcv_1m.low IS 'Price * 10000 (4 decimal places offset)';
COMMENT ON COLUMN ohlcv_1m.close IS 'Price * 10000 (4 decimal places offset)';
COMMENT ON COLUMN ohlcv_1m.volume IS 'Volume as normal integer (no decimal offset)';

-- Create hypertable with 4-month chunks and ticker partitioning
SELECT create_hypertable(
    'ohlcv_1m',
    'timestamp',
    'ticker',
    number_partitions => 8,
    chunk_time_interval => INTERVAL '4 months',
    if_not_exists => TRUE
);

-- Create new ohlcv_1d table with bigint prices and timestamptz
CREATE TABLE ohlcv_1d (
    "ticker"        text         NOT NULL,
    "volume"        bigint,                    -- Volume as normal integer (no decimal offset)
    "open"          bigint,                    -- Price * 10000 (4 decimal places offset)
    "close"         bigint,                    -- Price * 10000 (4 decimal places offset)
    "high"          bigint,                    -- Price * 10000 (4 decimal places offset)
    "low"           bigint,                    -- Price * 10000 (4 decimal places offset)
    "timestamp"     timestamptz  NOT NULL,
    "transactions"  integer,
    PRIMARY KEY ("ticker", "timestamp")
);
-- Add column comments for decimal offset storage on ohlcv_1d
COMMENT ON COLUMN ohlcv_1d.open IS 'Price * 10000 (4 decimal places offset)';
COMMENT ON COLUMN ohlcv_1d.high IS 'Price * 10000 (4 decimal places offset)';
COMMENT ON COLUMN ohlcv_1d.low IS 'Price * 10000 (4 decimal places offset)';
COMMENT ON COLUMN ohlcv_1d.close IS 'Price * 10000 (4 decimal places offset)';
COMMENT ON COLUMN ohlcv_1d.volume IS 'Volume as normal integer (no decimal offset)';

-- Create hypertable with no time chunking (single chunk) and ticker partitioning
SELECT create_hypertable(
    'ohlcv_1d',
    'timestamp',
    'ticker',
    number_partitions => 8,
    chunk_time_interval => INTERVAL '8 years',  -- Large interval for single chunk
    if_not_exists => TRUE
);

-- Migrate data from backup tables with proper type conversion
/*INSERT INTO ohlcv_1m (ticker, volume, open, close, high, low, timestamp, transactions)
SELECT 
    ticker,
    ROUND(volume)::bigint,                                    -- Volume as normal integer
    ROUND(open * 10000)::bigint,                             -- Price * 10000 for 4 decimal places
    ROUND(close * 10000)::bigint,                            -- Price * 10000 for 4 decimal places
    ROUND(high * 10000)::bigint,                             -- Price * 10000 for 4 decimal places
    ROUND(low * 10000)::bigint,                              -- Price * 10000 for 4 decimal places
    to_timestamp(timestamp / 1000000000.0),                  -- Convert nanoseconds to timestamptz
    transactions
FROM ohlcv_1m_backup;
*/
/*
INSERT INTO ohlcv_1d (ticker, volume, open, close, high, low, timestamp, transactions)
SELECT 
    ticker,
    ROUND(volume)::bigint,                                    -- Volume as normal integer
    ROUND(open * 10000)::bigint,                             -- Price * 10000 for 4 decimal places
    ROUND(close * 10000)::bigint,                            -- Price * 10000 for 4 decimal places
    ROUND(high * 10000)::bigint,                             -- Price * 10000 for 4 decimal places
    ROUND(low * 10000)::bigint,                              -- Price * 10000 for 4 decimal places
    "timestamp",                                              -- Timestamp is already timestamptz, no conversion needed
    transactions
FROM ohlcv_1d_backup;
*/

-- Create optimized indexes for fast queries
CREATE INDEX IF NOT EXISTS ohlcv_1m_ticker_ts_desc_inc 
    ON ohlcv_1m (ticker, "timestamp" DESC)
    INCLUDE (open, high, low, close, volume);

CREATE INDEX IF NOT EXISTS ohlcv_1d_ticker_ts_desc_inc 
    ON ohlcv_1d (ticker, "timestamp" DESC)
    INCLUDE (open, high, low, close, volume);





-- Enable compression on both tables
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
/*
-- DO NOT ENABLE COMPRESSION HERE AS IT WILL BE HANDLED BY OHLCV LOADER FOR MAXIMUM PERFORMANCE

-- Add compression policies
-- ohlcv_1m: compress data older than 2 weeks (14 days)
SELECT add_compression_policy('ohlcv_1m', INTERVAL '14 days');

-- ohlcv_1d: compress data older than 4 months (120 days)
SELECT add_compression_policy('ohlcv_1d', INTERVAL '120 days');
*/

-- Drop backup tables after successful migration
/*DROP TABLE IF EXISTS ohlcv_1m_backup;*/
DROP TABLE IF EXISTS ohlcv_update_state;

CREATE TABLE ohlcv_update_state (
    timeframe text PRIMARY KEY,
    earliest_loaded_at date,
    latest_loaded_at date
);

-- Record schema version
INSERT INTO schema_versions (version, description)
VALUES (78, 'Recreate OHLCV tables with bigint price columns (4 decimal offset) and timestamptz timestamps')
ON CONFLICT (version) DO UPDATE SET description = EXCLUDED.description;

COMMIT;