-- Migration: 011_change_version_to_numeric
-- Description: Changes schema_versions.version from VARCHAR to a numeric type
-- First create a temporary table with the new structure
CREATE TABLE schema_versions_new (
    version NUMERIC PRIMARY KEY,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    description TEXT
);
-- Copy data from the old table, converting version to numeric
INSERT INTO schema_versions_new (version, applied_at, description)
SELECT CASE
        WHEN version ~ '^[0-9]+$' THEN version::NUMERIC
        ELSE 0 -- Handle any non-numeric versions (shouldn't exist, but just in case)
    END,
    applied_at,
    description
FROM schema_versions;
-- Drop the old table
DROP TABLE schema_versions;
-- Rename the new table to schema_versions
ALTER TABLE schema_versions_new
    RENAME TO schema_versions;
-- Update the hardcoded entry for init.sql if it exists
UPDATE schema_versions
SET version = 10
WHERE version = '10'::NUMERIC
    OR description LIKE '%all rollouts up to 10 included%';