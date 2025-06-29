/* 34.sql ─ Multi-timeframe OHLCV Tables */
BEGIN;

------------------------------------------------------------------
-- 1. Create 1-minute OHLCV table with TimescaleDB hypertable
------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ohlcv_1m (
    securityid      INTEGER      NOT NULL,
    "timestamp"     TIMESTAMPTZ  NOT NULL,
    open            NUMERIC(22,4),
    high            NUMERIC(22,4),
    low             NUMERIC(22,4),
    close           NUMERIC(22,4),
    volume          BIGINT,
    PRIMARY KEY (securityid, "timestamp")
);

-- Convert to hypertable with optimized settings for 1-minute data
DO $$
BEGIN
    -- Check if table is already a hypertable
    IF NOT EXISTS (
        SELECT 1 FROM timescaledb_information.hypertables 
        WHERE hypertable_name = 'ohlcv_1m'
    ) THEN
        PERFORM create_hypertable(
            'ohlcv_1m',
            'timestamp',
            'securityid',
            number_partitions   => 16,        -- Space partitioning by security
            chunk_time_interval => INTERVAL '1 day',  -- Daily chunks for 1-minute data
            if_not_exists => TRUE
        );
    END IF;
END $$;

-- Indexes for 1-minute data
CREATE INDEX IF NOT EXISTS idx_ohlcv_1m_securityid ON ohlcv_1m (securityid);
CREATE INDEX IF NOT EXISTS idx_ohlcv_1m_timestamp ON ohlcv_1m ("timestamp" DESC);
CREATE INDEX IF NOT EXISTS idx_ohlcv_1m_security_time ON ohlcv_1m (securityid, "timestamp" DESC);

------------------------------------------------------------------
-- 2. Create 1-hour OHLCV table with TimescaleDB hypertable  
------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ohlcv_1h (
    securityid      INTEGER      NOT NULL,
    "timestamp"     TIMESTAMPTZ  NOT NULL,
    open            NUMERIC(22,4),
    high            NUMERIC(22,4),
    low             NUMERIC(22,4),
    close           NUMERIC(22,4),
    volume          BIGINT,
    PRIMARY KEY (securityid, "timestamp")
);

-- Convert to hypertable with optimized settings for 1-hour data
DO $$
BEGIN
    -- Check if table is already a hypertable
    IF NOT EXISTS (
        SELECT 1 FROM timescaledb_information.hypertables 
        WHERE hypertable_name = 'ohlcv_1h'
    ) THEN
        PERFORM create_hypertable(
            'ohlcv_1h',
            'timestamp',
            'securityid',
            number_partitions   => 16,        -- Space partitioning by security
            chunk_time_interval => INTERVAL '1 week',  -- Weekly chunks for 1-hour data
            if_not_exists => TRUE
        );
    END IF;
END $$;

-- Indexes for 1-hour data
CREATE INDEX IF NOT EXISTS idx_ohlcv_1h_securityid ON ohlcv_1h (securityid);
CREATE INDEX IF NOT EXISTS idx_ohlcv_1h_timestamp ON ohlcv_1h ("timestamp" DESC);
CREATE INDEX IF NOT EXISTS idx_ohlcv_1h_security_time ON ohlcv_1h (securityid, "timestamp" DESC);

------------------------------------------------------------------
-- 3. Create 1-week OHLCV table with TimescaleDB hypertable
------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ohlcv_1w (
    securityid      INTEGER      NOT NULL,
    "timestamp"     TIMESTAMPTZ  NOT NULL,
    open            NUMERIC(22,4),
    high            NUMERIC(22,4),
    low             NUMERIC(22,4),
    close           NUMERIC(22,4),
    volume          BIGINT,
    PRIMARY KEY (securityid, "timestamp")
);

-- Convert to hypertable with optimized settings for 1-week data
DO $$
BEGIN
    -- Check if table is already a hypertable
    IF NOT EXISTS (
        SELECT 1 FROM timescaledb_information.hypertables 
        WHERE hypertable_name = 'ohlcv_1w'
    ) THEN
        PERFORM create_hypertable(
            'ohlcv_1w',
            'timestamp',
            'securityid',
            number_partitions   => 16,        -- Space partitioning by security
            chunk_time_interval => INTERVAL '1 month',  -- Monthly chunks for 1-week data
            if_not_exists => TRUE
        );
    END IF;
END $$;

-- Indexes for 1-week data
CREATE INDEX IF NOT EXISTS idx_ohlcv_1w_securityid ON ohlcv_1w (securityid);
CREATE INDEX IF NOT EXISTS idx_ohlcv_1w_timestamp ON ohlcv_1w ("timestamp" DESC);
CREATE INDEX IF NOT EXISTS idx_ohlcv_1w_security_time ON ohlcv_1w (securityid, "timestamp" DESC);

------------------------------------------------------------------
-- 4. Enable columnstore before adding compression policies
------------------------------------------------------------------

-- Enable columnstore on each hypertable (required for compression policies)
DO $$
BEGIN
    -- Enable columnstore on ohlcv_1m if not already enabled
    BEGIN
        ALTER TABLE ohlcv_1m SET (timescaledb.compress, timescaledb.compress_orderby = 'timestamp DESC', timescaledb.compress_segmentby = 'securityid');
    EXCEPTION
        WHEN duplicate_object THEN
            NULL; -- Table already has compression enabled
        WHEN others THEN
            RAISE NOTICE 'Could not enable compression on ohlcv_1m: %', SQLERRM;
    END;
    
    -- Enable columnstore on ohlcv_1h if not already enabled
    BEGIN
        ALTER TABLE ohlcv_1h SET (timescaledb.compress, timescaledb.compress_orderby = 'timestamp DESC', timescaledb.compress_segmentby = 'securityid');
    EXCEPTION
        WHEN duplicate_object THEN
            NULL; -- Table already has compression enabled
        WHEN others THEN
            RAISE NOTICE 'Could not enable compression on ohlcv_1h: %', SQLERRM;
    END;
    
    -- Enable columnstore on ohlcv_1w if not already enabled
    BEGIN
        ALTER TABLE ohlcv_1w SET (timescaledb.compress, timescaledb.compress_orderby = 'timestamp DESC', timescaledb.compress_segmentby = 'securityid');
    EXCEPTION
        WHEN duplicate_object THEN
            NULL; -- Table already has compression enabled
        WHEN others THEN
            RAISE NOTICE 'Could not enable compression on ohlcv_1w: %', SQLERRM;
    END;
END $$;

------------------------------------------------------------------
-- 5. Add compression policies for optimal storage
------------------------------------------------------------------

-- Enable compression on older chunks for space efficiency
-- 1-minute data: compress after 7 days
DO $$
BEGIN
    BEGIN
        PERFORM add_compression_policy('ohlcv_1m', INTERVAL '7 days');
    EXCEPTION
        WHEN duplicate_object THEN
            NULL; -- Policy already exists
        WHEN others THEN
            RAISE NOTICE 'Could not add compression policy for ohlcv_1m: %', SQLERRM;
    END;
END $$;

-- 1-hour data: compress after 30 days  
DO $$
BEGIN
    BEGIN
        PERFORM add_compression_policy('ohlcv_1h', INTERVAL '30 days');
    EXCEPTION
        WHEN duplicate_object THEN
            NULL; -- Policy already exists
        WHEN others THEN
            RAISE NOTICE 'Could not add compression policy for ohlcv_1h: %', SQLERRM;
    END;
END $$;

-- 1-week data: compress after 90 days
DO $$
BEGIN
    BEGIN
        PERFORM add_compression_policy('ohlcv_1w', INTERVAL '90 days');
    EXCEPTION
        WHEN duplicate_object THEN
            NULL; -- Policy already exists
        WHEN others THEN
            RAISE NOTICE 'Could not add compression policy for ohlcv_1w: %', SQLERRM;
    END;
END $$;

------------------------------------------------------------------
-- 6. Add retention policies for data lifecycle management
------------------------------------------------------------------

-- 1-minute data: retain for 6 months (high volume, shorter retention)
DO $$
BEGIN
    BEGIN
        PERFORM add_retention_policy('ohlcv_1m', INTERVAL '6 months');
    EXCEPTION
        WHEN duplicate_object THEN
            NULL; -- Policy already exists
        WHEN others THEN
            RAISE NOTICE 'Could not add retention policy for ohlcv_1m: %', SQLERRM;
    END;
END $$;

-- 1-hour data: retain for 5 years
DO $$
BEGIN
    BEGIN
        PERFORM add_retention_policy('ohlcv_1h', INTERVAL '5 years');
    EXCEPTION
        WHEN duplicate_object THEN
            NULL; -- Policy already exists
        WHEN others THEN
            RAISE NOTICE 'Could not add retention policy for ohlcv_1h: %', SQLERRM;
    END;
END $$;

-- 1-week data: retain for 20 years (low volume, long retention)
DO $$
BEGIN
    BEGIN
        PERFORM add_retention_policy('ohlcv_1w', INTERVAL '20 years');
    EXCEPTION
        WHEN duplicate_object THEN
            NULL; -- Policy already exists
        WHEN others THEN
            RAISE NOTICE 'Could not add retention policy for ohlcv_1w: %', SQLERRM;
    END;
END $$;

COMMIT;

/*
Migration Summary:
- Created ohlcv_1m table with 1-day chunks and 6-month retention
- Created ohlcv_1h table with 1-week chunks and 5-year retention  
- Created ohlcv_1w table with 1-month chunks and 20-year retention
- All tables have space partitioning (16 partitions) for optimal query performance
- Enabled columnstore compression with proper segment and order by settings
- Compression policies reduce storage costs for older data
- Retention policies automatically manage data lifecycle
- Optimized indexes for common query patterns (security, time, security+time)
- Added proper error handling for idempotent migration execution
*/