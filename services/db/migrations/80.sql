BEGIN;

-- Drop legacy and unused columns from the alerts table.
-- The table will now exclusively store price alerts.
ALTER TABLE alerts DROP COLUMN IF EXISTS setupId;
ALTER TABLE alerts DROP COLUMN IF EXISTS algoId;
ALTER TABLE alerts DROP COLUMN IF EXISTS alertType;
ALTER TABLE alerts DROP COLUMN IF EXISTS triggeredTimestamp;

-- Drop the old alertLogs table if it exists (was already dropped in migration 14, but ensuring cleanup)
DROP TABLE IF EXISTS alertLogs CASCADE;

-- Create the new unified alert_logs table
CREATE TABLE IF NOT EXISTS alert_logs (
    log_id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(userId) ON DELETE CASCADE,
    alert_type VARCHAR(20) NOT NULL CHECK (alert_type IN ('price', 'strategy')),
    related_id INTEGER NOT NULL, -- alertId for price alerts, strategyId for strategy alerts
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    message TEXT NOT NULL,
    payload JSONB DEFAULT '{}'::jsonb -- Store additional data like securityId, ticker, etc.
);

-- Create indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_alert_logs_user_id ON alert_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_alert_logs_timestamp ON alert_logs(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_alert_logs_alert_type ON alert_logs(alert_type);
CREATE INDEX IF NOT EXISTS idx_alert_logs_related_id ON alert_logs(related_id);
CREATE INDEX IF NOT EXISTS idx_alert_logs_user_type_time ON alert_logs(user_id, alert_type, timestamp DESC);
-- Add alert configuration columns to strategies table
ALTER TABLE strategies ADD COLUMN IF NOT EXISTS alert_threshold NUMERIC DEFAULT NULL;
ALTER TABLE strategies ADD COLUMN IF NOT EXISTS alert_universe TEXT[] DEFAULT NULL;

-- Create index for efficient querying of strategy alerts
CREATE INDEX IF NOT EXISTS idx_strategies_alert_active ON strategies(isalertactive) WHERE isalertactive = true;
INSERT INTO schema_versions (version, description)
VALUES (
    80,
    'Create unified alert_logs table for both price and strategy alerts'
) ON CONFLICT (version) DO NOTHING;

COMMIT; 