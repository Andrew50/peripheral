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
    82,
    'Add ticker to alert_logs table'
) ON CONFLICT (version) DO NOTHING;

COMMIT;