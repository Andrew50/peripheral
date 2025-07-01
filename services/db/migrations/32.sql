-- Migration: 032_placeholder_migration
-- Description: Placeholder migration to maintain sequential numbering
BEGIN;
-- This is a placeholder migration to maintain sequential numbering
-- No actual schema changes needed for this version
-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
        32,
        'Placeholder migration for sequential numbering'
    ) ON CONFLICT (version) DO NOTHING;
COMMIT;