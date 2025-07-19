BEGIN;

-- Insert schema version
INSERT INTO schema_versions (version, description)
VALUES (
    74,
    'Update TimescaleDB extension'
) ON CONFLICT (version) DO NOTHING;

ALTER EXTENSION timescaledb UPDATE;

COMMIT;