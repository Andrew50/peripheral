
-- Migration: 051_change_securities_name_to_text
-- Description: Change securities.name column to TEXT to store unlimited length company names

BEGIN;

ALTER TABLE securities
    ALTER COLUMN name TYPE TEXT;

INSERT INTO schema_versions (version, description)
VALUES (
    51,
    'Change securities.name column to TEXT to store unlimited length company names'
) ON CONFLICT (version) DO NOTHING;