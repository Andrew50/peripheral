BEGIN;









ALTER TABLE screener ADD COLUMN IF NOT EXISTS change_1y_pct NUMERIC DEFAULT NULL;
INSERT INTO schema_versions (version, description)
VALUES (
    81,
    'Add ticker to alert_logs table'
) ON CONFLICT (version) DO NOTHING;

COMMIT;