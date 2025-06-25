-- Migration: 030_add_securities_enhancements
-- Description: Add ticker_norm generated column and idx_securities_active index

BEGIN;

-- Add ticker_norm generated column to securities table
-- This normalizes tickers by uppercasing and removing dots
ALTER TABLE securities
  ADD COLUMN ticker_norm text
        GENERATED ALWAYS AS (upper(replace(ticker, '.', ''))) STORED;

-- Create index for active securities (where maxDate IS NULL)
CREATE INDEX idx_securities_active ON securities (securityId) WHERE maxDate IS NULL;

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (30, 'Add ticker_norm generated column and idx_securities_active index')
ON CONFLICT (version) DO NOTHING;

COMMIT; 