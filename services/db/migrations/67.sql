-- Migration: 067_replace_screener_table_with_materialized_view
-- Description: Drop the old screener table and rename screener_ca materialized view to screener
BEGIN;

-- Drop the old screener table (hypertable)
DROP TABLE IF EXISTS screener CASCADE;

-- Rename the materialized view to screener
ALTER MATERIALIZED VIEW screener_ca RENAME TO screener;

-- Recreate the index with the new name
CREATE INDEX IF NOT EXISTS screener_latest_idx
    ON screener (ticker, calc_time DESC);

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    67,
    'Replace screener table with materialized view - drop old table and rename screener_ca to screener'
) ON CONFLICT (version) DO NOTHING;

COMMIT;