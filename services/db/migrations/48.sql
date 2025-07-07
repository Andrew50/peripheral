-- Migration 48: track failed flat files during OHLCV bulk load

CREATE TABLE IF NOT EXISTS ohlcv_failed_files (
    day DATE NOT NULL,
    timeframe TEXT NOT NULL,
    reason TEXT NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (day, timeframe, reason)
); 