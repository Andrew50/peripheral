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



INSERT INTO schema_versions (version, description)
VALUES (
    70,
    'Add materialized view for 52-week extremes'
) ON CONFLICT (version) DO NOTHING;
COMMIT; 