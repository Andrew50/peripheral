BEGIN;
CREATE TABLE IF NOT EXISTS backtest_jobs (
    job_id      SERIAL PRIMARY KEY,
    user_id     INTEGER NOT NULL,
    strategy_id INTEGER NOT NULL,
    rows_total  INTEGER,
    rows_done   INTEGER DEFAULT 0,
    created_at  TIMESTAMP DEFAULT now(),
    updated_at  TIMESTAMP DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_backtest_jobs_user ON backtest_jobs(user_id);
CREATE INDEX IF NOT EXISTS idx_backtest_jobs_strategy ON backtest_jobs(strategy_id);

ALTER TABLE IF EXISTS ohlcv_1d SET (timescaledb.compress, timescaledb.compress_segmentby='securityid');
SELECT add_compression_policy('ohlcv_1d', INTERVAL '30 days');
ALTER TABLE IF EXISTS ohlcv_1d SET (timescaledb.columnar);
COMMIT;

