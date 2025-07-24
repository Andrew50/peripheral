-- Migration 081: Reset strategies.version to INTEGER
-- Description: Drop and recreate version column as INTEGER with default value of 1

BEGIN;

-- Drop the existing index on version column
DROP INDEX IF EXISTS idx_strategies_version;

-- Drop the version column entirely
ALTER TABLE strategies DROP COLUMN IF EXISTS version;

-- Add version column back as INTEGER with default 1
ALTER TABLE strategies ADD COLUMN version INTEGER NOT NULL DEFAULT 1;

-- Recreate the index on the version column
CREATE INDEX IF NOT EXISTS idx_strategies_version ON strategies(version);

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    81,
    'Reset strategies.version to INTEGER with default value of 1'
) ON CONFLICT (version) DO NOTHING;

COMMIT; 