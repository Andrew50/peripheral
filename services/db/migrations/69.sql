-- Migration: 69_change_ohlcv1d_compression.sql
-- Description: (1) Change compression policy for ohlcv_1d to keep last 400 days uncompressed
--              (2) Create a materialized view to store 52-week high/low per ticker so the screener
--                  query no longer scans a full year of ohlcv_1d every refresh.

BEGIN;

------------------------------------------------------------------
-- 1.  Update compression policy (remove old → add new)          --
------------------------------------------------------------------

-- Remove existing compression policy if it exists
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM timescaledb_information.jobs
        WHERE hypertable_name = 'ohlcv_1d'
          AND proc_name       = 'policy_compression'
    ) THEN
        PERFORM remove_compression_policy('ohlcv_1d');
    END IF;
END$$;

-- Keep ~400 days of daily bars uncompressed, compress anything older
SELECT add_compression_policy('ohlcv_1d', INTERVAL '400 days');

------------------------------------------------------------------
-- 2.  Materialized view for 52-week extremes                    --
------------------------------------------------------------------
-- Stores the rolling 52-week high & low per ticker.  We refresh it once a day
-- after the new daily bar arrives.
CREATE MATERIALIZED VIEW IF NOT EXISTS ohlcv_52w_extremes
AS
SELECT ticker,
       MAX(high) AS wk52_high,
       MIN(low)  AS wk52_low
FROM   ohlcv_1d
WHERE  "timestamp" >= now() - INTERVAL '52 weeks'
GROUP  BY ticker
WITH NO DATA;

-- Index so JOINs are instant
CREATE UNIQUE INDEX IF NOT EXISTS ohlcv_52w_extremes_ticker_idx
    ON ohlcv_52w_extremes (ticker);

INSERT INTO schema_versions (version, description)
VALUES (
    69,
    'Change compression policy for ohlcv_1d to keep last 400 days uncompressed'
) ON CONFLICT (version) DO NOTHING;
COMMIT; 