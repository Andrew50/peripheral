-- Migration: 064_add_ticker_to_screener
-- Description: Add ticker column to screener table for direct filtering and display
BEGIN;

-- Add ticker column to screener table
ALTER TABLE screener 
ADD COLUMN IF NOT EXISTS ticker VARCHAR(10);

-- Create index on ticker for fast filtering
CREATE INDEX IF NOT EXISTS idx_screener_ticker ON screener (ticker);

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    64,
    'Add ticker column to screener table for direct filtering and display'
) ON CONFLICT (version) DO NOTHING;

COMMIT; 