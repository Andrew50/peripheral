-- Migration: 031_add_ticker_norm_column
-- Description: Add ticker_norm generated column for improved ticker search performance
BEGIN;
-- Add ticker_norm generated column to securities table ONLY if it doesn't exist
-- This normalizes tickers by uppercasing and removing dots
DO $$ BEGIN -- Check if ticker_norm column already exists
IF NOT EXISTS (
  SELECT 1
  FROM information_schema.columns
  WHERE table_name = 'securities'
    AND column_name = 'ticker_norm'
) THEN
ALTER TABLE securities
ADD COLUMN ticker_norm text GENERATED ALWAYS AS (upper(replace(ticker, '.', ''))) STORED;
END IF;
END $$;
-- Create index for active securities (where maxDate IS NULL)
CREATE INDEX IF NOT EXISTS idx_securities_active ON securities (securityId)
WHERE maxDate IS NULL;
-- Create index on ticker_norm for fast searches
CREATE INDEX IF NOT EXISTS idx_securities_ticker_norm ON securities (ticker_norm);
-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    31,
    'Add ticker_norm generated column for improved ticker search'
  ) ON CONFLICT (version) DO NOTHING;
COMMIT;