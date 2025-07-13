-- Migration: 060_drop_username_column
-- Description: Drop username column from users table

BEGIN;

-- Drop the username column
ALTER TABLE users DROP COLUMN IF EXISTS username;

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    60,
    'Drop username column from users table'
) ON CONFLICT (version) DO NOTHING;

COMMIT; 