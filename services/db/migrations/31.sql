-- Migration: 031_add_ticker_norm_column
-- Description: Add ticker_norm generated column for improved ticker search performance

BEGIN;

-- Add ticker_norm generated column to securities table
-- This normalizes tickers by uppercasing and removing dots
ALTER TABLE securities
  ADD COLUMN ticker_norm text
        GENERATED ALWAYS AS (upper(replace(ticker, '.', ''))) STORED;

-- Create index for active securities (where maxDate IS NULL)
CREATE INDEX IF NOT EXISTS idx_securities_active ON securities (securityId) WHERE maxDate IS NULL;

-- Create index on ticker_norm for fast searches
CREATE INDEX IF NOT EXISTS idx_securities_ticker_norm ON securities (ticker_norm);

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (31, 'Add ticker_norm generated column for improved ticker search')
ON CONFLICT (version) DO NOTHING;

COMMIT;