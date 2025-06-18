-- Migration: 026_increase_securities_name_length
-- Description: Increase name column length in securities table to handle longer company names
BEGIN;
-- Increase the length of the name column from VARCHAR(200) to VARCHAR(500)
-- to accommodate longer company names from external APIs
ALTER TABLE securities
ALTER COLUMN name TYPE VARCHAR(500);
-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
        26,
        'Increase securities name column length to VARCHAR(500)'
    ) ON CONFLICT (version) DO NOTHING;
COMMIT;