-- Migration: 021_add_why_is_it_moving_table
-- Description: Add why_is_it_moving table to track news/reasons for stock movements

BEGIN;

-- Create why_is_it_moving table to store movement explanations for securities
CREATE TABLE IF NOT EXISTS why_is_it_moving (
    id SERIAL PRIMARY KEY,
    securityid int,
    ticker VARCHAR(10) NOT NULL,
    content TEXT NOT NULL,
    source VARCHAR(100), -- Optional: track the source of the information
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_why_is_it_moving_ticker ON why_is_it_moving(ticker);
CREATE INDEX IF NOT EXISTS idx_why_is_it_moving_created_at ON why_is_it_moving(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_why_is_it_moving_ticker_date ON why_is_it_moving(ticker, created_at DESC);

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (21, 'Add why_is_it_moving table for tracking stock movement explanations');

COMMIT; 