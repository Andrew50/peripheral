-- Migration: 004_create_ohlcv_table
-- Description: Creates a hypertable for storing daily OHLCV data for securities

-- Create the daily OHLCV table
CREATE TABLE IF NOT EXISTS daily_ohlcv (
    timestamp TIMESTAMP NOT NULL,
    security_id INTEGER NOT NULL,
    ticker VARCHAR(10) NOT NULL,
    open DECIMAL(25, 6) NOT NULL,
    high DECIMAL(25, 6) NOT NULL,
    low DECIMAL(25, 6) NOT NULL,
    close DECIMAL(25, 6) NOT NULL,
    volume BIGINT NOT NULL,
    vwap DECIMAL(25, 6),
    transactions INTEGER,
    CONSTRAINT unique_security_date
        UNIQUE (securityid, timestamp)
);

-- Convert to TimescaleDB hypertable
SELECT create_hypertable('daily_ohlcv', 'timestamp');

-- Create indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_daily_ohlcv_security_id ON daily_ohlcv(security_id);
CREATE INDEX IF NOT EXISTS idx_daily_ohlcv_ticker ON daily_ohlcv(ticker);
CREATE INDEX IF NOT EXISTS idx_daily_ohlcv_timestamp_desc ON daily_ohlcv(timestamp DESC);

-- Insert record in schema_versions table
INSERT INTO schema_versions (version, description)
VALUES ('004_create_ohlcv_table', 'Creates hypertable for daily OHLCV data') 
ON CONFLICT (version) DO NOTHING; 