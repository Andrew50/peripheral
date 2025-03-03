-- Migration: 000_create_schema_versions
-- Description: Creates a table to track applied database migrations
CREATE TABLE IF NOT EXISTS schema_versions (
    version VARCHAR(50) PRIMARY KEY,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    description TEXT
);
-- Insert a record for this migration
INSERT INTO schema_versions (version, description)
VALUES (
        '000_create_schema_versions',
        'Creates schema_versions table'
    ) ON CONFLICT (version) DO NOTHING;