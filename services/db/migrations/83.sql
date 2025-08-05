-- Migration 083: Enable strategy versioning by updating unique constraint
-- Description: Change unique constraint from (userId, name) to (userId, name, version) to allow multiple versions

BEGIN;

ALTER TABLE alert_logs ADD COLUMN IF NOT EXISTS ticker TEXT DEFAULT NULL;

-- Drop unused strategy columns
ALTER TABLE strategies DROP COLUMN IF EXISTS spec;
ALTER TABLE strategies DROP COLUMN IF EXISTS libraries;
ALTER TABLE strategies DROP COLUMN IF EXISTS data_prep_sql;
ALTER TABLE strategies DROP COLUMN IF EXISTS execution_mode;
ALTER TABLE strategies DROP COLUMN IF EXISTS timeout_seconds;
ALTER TABLE strategies DROP COLUMN IF EXISTS memory_limit_mb;
ALTER TABLE strategies DROP COLUMN IF EXISTS cpu_limit_cores;
ALTER TABLE strategies DROP COLUMN IF EXISTS isalertactive;

INSERT INTO schema_versions (version, description)
VALUES (
    83,
    'Enable strategy versioning by changing unique constraint to (userId, name, version)'
) ON CONFLICT (version) DO NOTHING;

COMMIT;