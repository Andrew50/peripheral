-- Migration: 021_add_why_is_it_moving_table
-- Description: Add why_is_it_moving table to track news/reasons for stock movements

BEGIN;

-- Create why_is_it_moving table to store movement explanations for securities
CREATE TABLE IF NOT EXISTS why_is_it_moving (
    id SERIAL PRIMARY KEY,
    securityid INTEGER NOT NULL REFERENCES securities(securityid) ON DELETE CASCADE,
    ticker VARCHAR(10) NOT NULL,
    date DATE NOT NULL, -- When the movement occurred (business date)
    content TEXT NOT NULL,
    source VARCHAR(100), -- Optional: track the source of the information
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Ensure one entry per security per movement date
    UNIQUE (securityid, created_at)
);

-- Create indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_why_is_it_moving_securityid ON why_is_it_moving(securityid);
CREATE INDEX IF NOT EXISTS idx_why_is_it_moving_ticker ON why_is_it_moving(ticker);
CREATE INDEX IF NOT EXISTS idx_why_is_it_moving_date ON why_is_it_moving(date DESC);
CREATE INDEX IF NOT EXISTS idx_why_is_it_moving_created_at ON why_is_it_moving(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_why_is_it_moving_security_date ON why_is_it_moving(securityid, date DESC);

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (21, 'Add why_is_it_moving table for tracking stock movement explanations');

COMMIT; 