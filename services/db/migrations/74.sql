-- Migration 074: Add cloudflare_ipv6 column to splash_screen_logs table

BEGIN;

-- Add cloudflare_ipv6 column to store Cloudflare's CF-Connecting-IP header
ALTER TABLE splash_screen_logs 
ADD COLUMN IF NOT EXISTS cloudflare_ipv6 INET;

-- Add index for the new column for performance
CREATE INDEX IF NOT EXISTS idx_splash_screen_logs_cloudflare_ipv6 ON splash_screen_logs(cloudflare_ipv6);

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    74,
    'Add cloudflare_ipv6 column to splash_screen_logs table'
) ON CONFLICT (version) DO NOTHING;

COMMIT;
