BEGIN;

-- Drop the limit columns from users table since limits are now queried from subscription_products table
-- The users table should only store current usage counts, not limits

-- Drop queries_limit column if it exists
ALTER TABLE users DROP COLUMN IF EXISTS queries_limit;

-- Drop alerts_limit column if it exists  
ALTER TABLE users DROP COLUMN IF EXISTS alerts_limit;

-- Drop strategy_alerts_limit column if it exists
ALTER TABLE users DROP COLUMN IF EXISTS strategy_alerts_limit;

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    88,
    'Drop queries_limit, alerts_limit, and strategy_alerts_limit columns from users table - limits now queried from subscription_products'
) ON CONFLICT (version) DO NOTHING;

COMMIT; 