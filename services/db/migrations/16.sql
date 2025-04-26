/* 16.sql ─ rebuild daily_ohlcv → ohlcv_1d */

BEGIN;

------------------------------------------------------------------
-- 1. Take the existing hypertable out of service but keep the data
------------------------------------------------------------------
ALTER TABLE IF EXISTS daily_ohlcv
    RENAME TO daily_ohlcv_old;
-- (It is still a hypertable; its chunks are now named _hyper_*_daily_ohlcv_old.)

------------------------------------------------------------------
-- 2. Create the new heap table with the target schema
------------------------------------------------------------------
CREATE TABLE ohlcv_1d (
    securityid      INTEGER      NOT NULL,
    "timestamp"     TIMESTAMP    NOT NULL,
    open            NUMERIC(12,4),
    high            NUMERIC(12,4),
    low             NUMERIC(12,4),
    close           NUMERIC(12,4),
    volume          BIGINT,
    -- add/keep any additional columns you need here
    PRIMARY KEY (securityid, "timestamp")
);

------------------------------------------------------------------
-- 3. Promote it to a hypertable (time + space partitioning)
------------------------------------------------------------------
SELECT create_hypertable(
           'ohlcv_1d',
           'timestamp',
           'securityid',
           number_partitions   => 16,
           chunk_time_interval => INTERVAL '1 month'
       );

------------------------------------------------------------------
-- 4. Bulk-migrate the rows from the old table
------------------------------------------------------------------
INSERT INTO ohlcv_1d (securityid,
                      "timestamp",
                      open,
                      high,
                      low,
                      close,
                      volume)
SELECT securityid::INTEGER,      -- cast if the original type wasn’t integer
       "timestamp",
       open,
       high,
       low,
       close,
       volume
FROM   daily_ohlcv_old;

------------------------------------------------------------------
-- 5. Secondary indexes for your typical filter/scan patterns
------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_ohlcv_1d_securityid ON ohlcv_1d (securityid);
CREATE INDEX IF NOT EXISTS idx_ohlcv_1d_timestamp   ON ohlcv_1d ("timestamp" DESC);

------------------------------------------------------------------
-- 6. Retire the old table once the new one is verified
------------------------------------------------------------------
DROP TABLE daily_ohlcv_old;

COMMIT;



/* ---------------------------------------------------------------------------
   Ancillary schema bits preserved from the original migration
--------------------------------------------------------------------------- */

-- Fundamentals -------------------------------------------------------------
CREATE TABLE IF NOT EXISTS fundamentals (
    security_id          VARCHAR(20)  REFERENCES securities(security_id),
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
ALTER TABLE strategies
    RENAME COLUMN criteria TO spec;

