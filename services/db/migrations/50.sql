-- Migration: 050_add_timeframe_to_ohlcv_update_state
-- Description: Recreate ohlcv_update_state with per-timeframe tracking and seed default rows.

BEGIN;

-- Drop existing state table (removes any legacy boolean-id records)
DROP TABLE IF EXISTS ohlcv_update_state;

-- Recreate with new schema keyed by timeframe
CREATE TABLE ohlcv_update_state (
    timeframe       text PRIMARY KEY,
    last_loaded_at  date NOT NULL
);

-- Seed initial rows for known timeframes
INSERT INTO ohlcv_update_state (timeframe, last_loaded_at) VALUES
    ('1-minute', DATE '2008-01-01'),
    ('1-day',    DATE '2008-01-01');

-- Record schema version
INSERT INTO schema_versions (version, description)
VALUES (50, 'Recreate ohlcv_update_state with timeframe key')
ON CONFLICT (version) DO NOTHING;

COMMIT; 