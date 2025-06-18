-- Migration: 024_add_is_content_to_why_is_it_moving
-- Description: Add is_content column to why_is_it_moving table to distinguish between content types

BEGIN;

-- Add is_content column to why_is_it_moving table
ALTER TABLE why_is_it_moving 
ADD COLUMN IF NOT EXISTS is_content BOOLEAN NOT NULL DEFAULT FALSE;

-- Create index for efficient querying by content type
CREATE INDEX IF NOT EXISTS idx_why_is_it_moving_is_content ON why_is_it_moving(is_content);

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (24, 'Add is_content column to why_is_it_moving table')
ON CONFLICT (version) DO NOTHING;

COMMIT;

