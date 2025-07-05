-- Migration: 041_rename_alerts_to_active_alerts
-- Description: Rename alerts_used and strategy_alerts_used to active_alerts and active_strategy_alerts, and remove free tier reset behavior
-- Note: Usage types are now 'credits', 'alert', 'strategy_alert' (removed 'query' and 'chart_request')

BEGIN;

-- Rename alerts_used to active_alerts
ALTER TABLE users RENAME COLUMN alerts_used TO active_alerts;

-- Rename strategy_alerts_used to active_strategy_alerts  
ALTER TABLE users RENAME COLUMN strategy_alerts_used TO active_strategy_alerts;

-- Update indexes to match new column names
DROP INDEX IF EXISTS idx_users_queries_usage;
CREATE INDEX IF NOT EXISTS idx_users_active_alerts ON users(active_alerts, alerts_limit);
CREATE INDEX IF NOT EXISTS idx_users_active_strategy_alerts ON users(active_strategy_alerts, strategy_alerts_limit);

-- Remove the reset_user_limits function from migration 39 as it's no longer needed
DROP FUNCTION IF EXISTS reset_user_limits();

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    41,
    'Rename alerts_used and strategy_alerts_used to active_alerts and active_strategy_alerts, remove free tier reset behavior'
) ON CONFLICT (version) DO NOTHING;

COMMIT; 