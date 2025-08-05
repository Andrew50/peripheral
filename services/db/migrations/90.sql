BEGIN;

-- Drop the limit columns from users table since limits are now queried from subscription_products table
-- The users table should only store current usage counts, not limits

-- Drop queries_limit column if it exists
ALTER TABLE users DROP COLUMN IF EXISTS queries_limit;

-- Drop alerts_limit column if it exists  
ALTER TABLE users DROP COLUMN IF EXISTS alerts_limit;

-- Drop strategy_alerts_limit column if it exists
ALTER TABLE users DROP COLUMN IF EXISTS strategy_alerts_limit;

-- Add per-strategy alert metadata
ALTER TABLE strategies
    ADD COLUMN IF NOT EXISTS min_timeframe          TEXT,
    ADD COLUMN IF NOT EXISTS alert_last_trigger_at  TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS alert_universe_full    TEXT[];

-- Add constraint to ensure array elements are non-empty strings
ALTER TABLE strategies
    ADD CONSTRAINT alert_universe_full_nonempty
    CHECK (
        alert_universe_full IS NULL
        OR NOT ('' = ANY(alert_universe_full))
    );

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    90,
    'Drop queries_limit, alerts_limit, and strategy_alerts_limit columns from users table - limits now queried from subscription_products. Add min_timeframe, alert_last_trigger_at, and alert_universe_full to strategies'
) ON CONFLICT (version) DO NOTHING;

COMMIT; 