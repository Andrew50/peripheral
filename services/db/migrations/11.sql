-- Description: Fix schema_versions table to ensure version is numeric
-- This migration corrects the data type of the version column in schema_versions table
-- Check if there are any non-numeric versions in the table
DO $$
DECLARE version_count INT;
BEGIN -- Check if we need to migrate data (using exception handling)
BEGIN -- If this query succeeds, the column is already numeric
PERFORM version::numeric
FROM schema_versions
LIMIT 1;
RAISE NOTICE 'Version column already has correct numeric format.';
EXCEPTION
WHEN others THEN -- We need to migrate the data
RAISE NOTICE 'Migrating schema_versions table to use numeric version format...';
-- Create a temporary table to hold existing data
CREATE TEMP TABLE schema_versions_backup AS
SELECT *
FROM schema_versions;
-- Drop the existing table
DROP TABLE schema_versions;
-- Recreate the table with the correct column type
CREATE TABLE schema_versions (
    version NUMERIC PRIMARY KEY,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    description TEXT
);
-- Migrate data with special handling for version
INSERT INTO schema_versions (version, applied_at, description)
SELECT -- Extract numeric part from version strings like '001_create_table'
    CASE
        WHEN version ~ '^[0-9]+' THEN (regexp_matches(version, '^0*([0-9]+)')) [1]::numeric
        ELSE 11 -- Default to 11 for any unparseable versions
    END AS version,
    applied_at,
    description
FROM schema_versions_backup ON CONFLICT (version) DO
UPDATE
SET description = EXCLUDED.description,
    applied_at = EXCLUDED.applied_at;
-- Count how many versions we migrated
GET DIAGNOSTICS version_count = ROW_COUNT;
RAISE NOTICE 'Migrated % version records',
version_count;
-- Drop the temp table
DROP TABLE schema_versions_backup;
END;
END;
$$;