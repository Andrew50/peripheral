-- Migration 076: Test no downtime migration deployment

BEGIN;

-- add table
CREATE TABLE IF NOT EXISTS test_migration_table (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);
-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    76,
    'Migration Test'
) ON CONFLICT (version) DO NOTHING;

COMMIT; 