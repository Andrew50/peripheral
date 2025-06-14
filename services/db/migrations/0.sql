-- Migration: 000_create_schema_versions
-- Description: Creates a table to track applied database migrations
CREATE TABLE IF NOT EXISTS schema_versions (
    version NUMERIC PRIMARY KEY,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    description TEXT
);