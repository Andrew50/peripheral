/* 16.sql ─ rebuild daily_ohlcv → ohlcv_1d */
BEGIN;
------------------------------------------------------------------
-- 1. Drop the existing hypertable (completely removing the data)
------------------------------------------------------------------
DROP TABLE IF EXISTS daily_ohlcv;
------------------------------------------------------------------
-- 2. Create the new heap table with the target schema
------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ohlcv_1d (
    securityid      INTEGER      NOT NULL,
    "timestamp"     TIMESTAMP    NOT NULL,
    open            NUMERIC(22,4),
    high            NUMERIC(22,4),
    low             NUMERIC(22,4),
    close           NUMERIC(22,4),
    volume          BIGINT,
    -- add/keep any additional columns you need here
    PRIMARY KEY (securityid, "timestamp")
);
------------------------------------------------------------------
-- 3. Promote it to a hypertable (time + space partitioning)
--   only if it's not already a hypertable
------------------------------------------------------------------
DO $$
BEGIN
    -- Check if table is already a hypertable
    IF NOT EXISTS (
        SELECT 1 FROM timescaledb_information.hypertables 
        WHERE hypertable_name = 'ohlcv_1d'
    ) THEN
        PERFORM create_hypertable(
            'ohlcv_1d',
            'timestamp',
            'securityid',
            number_partitions   => 16,
            chunk_time_interval => INTERVAL '1 month',
            if_not_exists => TRUE
        );
    END IF;
END $$;
------------------------------------------------------------------
-- 4. Secondary indexes for your typical filter/scan patterns
------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_ohlcv_1d_securityid ON ohlcv_1d (securityid);
CREATE INDEX IF NOT EXISTS idx_ohlcv_1d_timestamp  ON ohlcv_1d ("timestamp" DESC);
COMMIT;
/* ---------------------------------------------------------------------------
   Ancillary schema bits preserved from the original migration
--------------------------------------------------------------------------- */
-- Fundamentals -------------------------------------------------------------
CREATE TABLE IF NOT EXISTS fundamentals (
    security_id          INT,
    "timestamp"          TIMESTAMP,
    market_cap           DECIMAL(22,2),
    shares_outstanding   BIGINT,
    eps                  DECIMAL(12,4),
    revenue              DECIMAL(22,2),
    dividend             DECIMAL(12,4),
    social_sentiment     DECIMAL(10,4),
    fear_greed           DECIMAL(10,4),
    short_interest       DECIMAL(10,4),
    borrow_fee           DECIMAL(10,4),
    PRIMARY KEY (security_id, "timestamp")
);
CREATE INDEX IF NOT EXISTS idx_fundamentals_security  ON fundamentals (security_id);
CREATE INDEX IF NOT EXISTS idx_fundamentals_timestamp ON fundamentals ("timestamp");
-- Strategies ----------------------------------------------------------------
DO $$
BEGIN
    -- Check if the 'criteria' column exists in the 'strategies' table
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema() -- Use current_schema() for safety
          AND table_name = 'strategies'
          AND column_name = 'criteria'
    ) THEN
        -- If it exists, rename it to 'spec'
        ALTER TABLE strategies RENAME COLUMN criteria TO spec;
    END IF;
END $$;
