-- Migration to add intraday and weekly OHLCV tables
BEGIN;
CREATE TABLE IF NOT EXISTS ohlcv_1s (
    timestamp TIMESTAMP NOT NULL,
    securityid INTEGER NOT NULL,
    open DECIMAL(25, 6) NOT NULL,
    high DECIMAL(25, 6) NOT NULL,
    low DECIMAL(25, 6) NOT NULL,
    close DECIMAL(25, 6) NOT NULL,
    volume BIGINT NOT NULL,
    PRIMARY KEY (securityid, timestamp)
);
SELECT create_hypertable('ohlcv_1s', 'timestamp',
                         'securityid', number_partitions => 16,
                         chunk_time_interval => INTERVAL '1 day',
                         if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS idx_ohlcv_1s_securityid ON ohlcv_1s(securityid);
CREATE INDEX IF NOT EXISTS idx_ohlcv_1s_timestamp ON ohlcv_1s(timestamp DESC);

CREATE TABLE IF NOT EXISTS ohlcv_1 (
    timestamp TIMESTAMP NOT NULL,
    securityid INTEGER NOT NULL,
    open DECIMAL(25, 6) NOT NULL,
    high DECIMAL(25, 6) NOT NULL,
    low DECIMAL(25, 6) NOT NULL,
    close DECIMAL(25, 6) NOT NULL,
    volume BIGINT NOT NULL,
    PRIMARY KEY (securityid, timestamp)
);
SELECT create_hypertable('ohlcv_1', 'timestamp',
                         'securityid', number_partitions => 16,
                         chunk_time_interval => INTERVAL '1 day',
                         if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS idx_ohlcv_1_securityid ON ohlcv_1(securityid);
CREATE INDEX IF NOT EXISTS idx_ohlcv_1_timestamp ON ohlcv_1(timestamp DESC);

CREATE TABLE IF NOT EXISTS ohlcv_1h (
    timestamp TIMESTAMP NOT NULL,
    securityid INTEGER NOT NULL,
    open DECIMAL(25, 6) NOT NULL,
    high DECIMAL(25, 6) NOT NULL,
    low DECIMAL(25, 6) NOT NULL,
    close DECIMAL(25, 6) NOT NULL,
    volume BIGINT NOT NULL,
    PRIMARY KEY (securityid, timestamp)
);
SELECT create_hypertable('ohlcv_1h', 'timestamp',
                         'securityid', number_partitions => 16,
                         chunk_time_interval => INTERVAL '1 month',
                         if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS idx_ohlcv_1h_securityid ON ohlcv_1h(securityid);
CREATE INDEX IF NOT EXISTS idx_ohlcv_1h_timestamp ON ohlcv_1h(timestamp DESC);

CREATE TABLE IF NOT EXISTS ohlcv_1w (
    timestamp TIMESTAMP NOT NULL,
    securityid INTEGER NOT NULL,
    open DECIMAL(25, 6) NOT NULL,
    high DECIMAL(25, 6) NOT NULL,
    low DECIMAL(25, 6) NOT NULL,
    close DECIMAL(25, 6) NOT NULL,
    volume BIGINT NOT NULL,
    PRIMARY KEY (securityid, timestamp)
);
SELECT create_hypertable('ohlcv_1w', 'timestamp',
                         'securityid', number_partitions => 16,
                         chunk_time_interval => INTERVAL '1 month',
                         if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS idx_ohlcv_1w_securityid ON ohlcv_1w(securityid);
CREATE INDEX IF NOT EXISTS idx_ohlcv_1w_timestamp ON ohlcv_1w(timestamp DESC);
COMMIT;
