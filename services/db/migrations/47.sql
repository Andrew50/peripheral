BEGIN;


-- Create the 1-second OHLCV table
CREATE TABLE IF NOT EXISTS ohlcv_1s (
    timestamp TIMESTAMP NOT NULL,
    securityid INTEGER NOT NULL,
    open DECIMAL(25, 6) NOT NULL,
    high DECIMAL(25, 6) NOT NULL,
    low DECIMAL(25, 6) NOT NULL,
    close DECIMAL(25, 6) NOT NULL,
    volume BIGINT NOT NULL,
    volume_weighted_avg DECIMAL(25, 6),
    trade_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    PRIMARY KEY (securityid, timestamp)
);

-- Convert to TimescaleDB hypertable with partitioning by securityid
SELECT create_hypertable(
    'ohlcv_1s',
    'timestamp',
    'securityid', 
    number_partitions => 32,        -- More partitions for higher volume
    chunk_time_interval => INTERVAL '1 hour',  -- Smaller chunks for 1-second data
    if_not_exists => TRUE
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_ohlcv_1s_securityid ON ohlcv_1s(securityid);
CREATE INDEX IF NOT EXISTS idx_ohlcv_1s_timestamp ON ohlcv_1s(timestamp DESC);

COMMIT; 