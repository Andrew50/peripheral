-- Migration 075: Add geolocation columns to splash_screen_logs table

BEGIN;

-- Add geolocation columns to splash_screen_logs
ALTER TABLE splash_screen_logs 
ADD COLUMN IF NOT EXISTS city VARCHAR(100),
ADD COLUMN IF NOT EXISTS region VARCHAR(100),
ADD COLUMN IF NOT EXISTS country VARCHAR(10),
ADD COLUMN IF NOT EXISTS org TEXT;

-- Indexes for performance on geolocation queries
CREATE INDEX IF NOT EXISTS idx_splash_screen_logs_country ON splash_screen_logs(country);
CREATE INDEX IF NOT EXISTS idx_splash_screen_logs_city ON splash_screen_logs(city);

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    75,
    'Add geolocation columns to splash_screen_logs table'
) ON CONFLICT (version) DO NOTHING;

COMMIT; 