-- Migration: 039_add_usage_limits_tracking
-- Description: Add usage tracking and limits to users table for subscription enforcement

BEGIN;

-- Add missing subscription_plan column to users table
ALTER TABLE users 
ADD COLUMN IF NOT EXISTS subscription_plan VARCHAR(50) DEFAULT 'Free';

-- Add usage tracking columns to users table
ALTER TABLE users 
ADD COLUMN IF NOT EXISTS queries_used INTEGER DEFAULT 0,
ADD COLUMN IF NOT EXISTS queries_limit INTEGER DEFAULT 5, -- Free tier default
ADD COLUMN IF NOT EXISTS alerts_used INTEGER DEFAULT 0,
ADD COLUMN IF NOT EXISTS alerts_limit INTEGER DEFAULT 0,
ADD COLUMN IF NOT EXISTS strategy_alerts_used INTEGER DEFAULT 0,
ADD COLUMN IF NOT EXISTS strategy_alerts_limit INTEGER DEFAULT 0,
ADD COLUMN IF NOT EXISTS current_period_start TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
ADD COLUMN IF NOT EXISTS last_limit_reset TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP;

-- Create indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_users_subscription_plan ON users(subscription_plan);
CREATE INDEX IF NOT EXISTS idx_users_queries_usage ON users(queries_used, queries_limit);
CREATE INDEX IF NOT EXISTS idx_users_current_period ON users(current_period_start);
CREATE INDEX IF NOT EXISTS idx_users_last_reset ON users(last_limit_reset);

-- Create usage_logs table for detailed tracking and analytics
CREATE TABLE IF NOT EXISTS usage_logs (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(userId) ON DELETE CASCADE,
    usage_type VARCHAR(50) NOT NULL, -- 'query', 'chart_request', 'alert', 'strategy_alert'
    resource_consumed INTEGER DEFAULT 1, -- How many units of the resource were used
    plan_name VARCHAR(50), -- Track which plan the user was on when they used this
    metadata JSONB DEFAULT '{}', -- Store additional context (query type, chart timeframe, etc.)
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for usage_logs
CREATE INDEX IF NOT EXISTS idx_usage_logs_user_id ON usage_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_usage_logs_created_at ON usage_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_usage_logs_usage_type ON usage_logs(usage_type);
CREATE INDEX IF NOT EXISTS idx_usage_logs_user_type_date ON usage_logs(user_id, usage_type, created_at DESC);

-- Create plan_limits table for centralized limit configuration
CREATE TABLE IF NOT EXISTS plan_limits (
    id SERIAL PRIMARY KEY,
    plan_name VARCHAR(50) NOT NULL UNIQUE,
    queries_limit INTEGER NOT NULL DEFAULT 0,
    alerts_limit INTEGER NOT NULL DEFAULT 0,
    strategy_alerts_limit INTEGER NOT NULL DEFAULT 0,
    realtime_charts BOOLEAN DEFAULT FALSE,
    sub_minute_charts BOOLEAN DEFAULT FALSE,
    multi_chart BOOLEAN DEFAULT FALSE,
    multi_strategy_screening BOOLEAN DEFAULT FALSE,
    watchlist_alerts BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Insert default plan limits based on pricing config
INSERT INTO plan_limits (plan_name, queries_limit, alerts_limit, strategy_alerts_limit, realtime_charts, sub_minute_charts, multi_chart, multi_strategy_screening, watchlist_alerts) VALUES
('Free', 5, 0, 0, FALSE, FALSE, FALSE, FALSE, FALSE),
('Plus', 250, 100, 5, TRUE, FALSE, FALSE, FALSE, FALSE),
('Pro', 1000, 400, 20, TRUE, TRUE, TRUE, TRUE, TRUE)
ON CONFLICT (plan_name) DO NOTHING;

-- Create function to reset user limits based on their billing cycle
CREATE OR REPLACE FUNCTION reset_user_limits() RETURNS VOID AS $$
DECLARE
    user_record RECORD;
BEGIN
    -- Reset limits for users whose billing period has passed
    FOR user_record IN 
        SELECT userId, subscription_status, subscription_plan, current_period_end, current_period_start
        FROM users 
        WHERE subscription_status IN ('active', 'inactive') 
        AND (
            -- For active subscriptions, check if billing period ended
            (subscription_status = 'active' AND current_period_end IS NOT NULL AND current_period_end <= CURRENT_TIMESTAMP)
            OR
            -- For free users, reset monthly (30 days since last reset)
            (subscription_status = 'inactive' AND last_limit_reset <= CURRENT_TIMESTAMP - INTERVAL '30 days')
        )
    LOOP
        -- Update usage counts and period dates
        UPDATE users SET 
            queries_used = 0,
            alerts_used = 0,
            strategy_alerts_used = 0,
            current_period_start = CURRENT_TIMESTAMP,
            last_limit_reset = CURRENT_TIMESTAMP
        WHERE userId = user_record.userId;
        
        -- Log the reset action
        INSERT INTO usage_logs (user_id, usage_type, resource_consumed, plan_name, metadata)
        VALUES (
            user_record.userId, 
            'limit_reset', 
            0, 
            COALESCE(user_record.subscription_plan, 'Free'),
            jsonb_build_object('reset_reason', 'billing_cycle', 'previous_period_end', user_record.current_period_end)
        );
    END LOOP;
END;
$$ LANGUAGE plpgsql;

-- Create function to update user limits when subscription changes
CREATE OR REPLACE FUNCTION update_user_limits_on_subscription_change() RETURNS TRIGGER AS $$
DECLARE
    plan_record RECORD;
BEGIN
    -- Get limits for the new plan
    SELECT * INTO plan_record FROM plan_limits WHERE plan_name = COALESCE(NEW.subscription_plan, 'Free');
    
    IF plan_record IS NOT NULL THEN
        -- Update user limits based on new plan
        NEW.queries_limit := plan_record.queries_limit;
        NEW.alerts_limit := plan_record.alerts_limit;
        NEW.strategy_alerts_limit := plan_record.strategy_alerts_limit;
        
        -- If upgrading to a paid plan, reset usage to give immediate access
        IF OLD.subscription_plan IS DISTINCT FROM NEW.subscription_plan AND NEW.subscription_status = 'active' THEN
            NEW.queries_used := 0;
            NEW.alerts_used := 0;
            NEW.strategy_alerts_used := 0;
            NEW.current_period_start := CURRENT_TIMESTAMP;
            NEW.last_limit_reset := CURRENT_TIMESTAMP;
        END IF;
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger to automatically update limits when subscription changes
DROP TRIGGER IF EXISTS trigger_update_user_limits ON users;
CREATE TRIGGER trigger_update_user_limits
    BEFORE UPDATE ON users
    FOR EACH ROW
    WHEN (OLD.subscription_plan IS DISTINCT FROM NEW.subscription_plan OR OLD.subscription_status IS DISTINCT FROM NEW.subscription_status)
    EXECUTE FUNCTION update_user_limits_on_subscription_change();

-- Update existing users with proper limits based on their current plan
UPDATE users SET 
    queries_limit = CASE 
        WHEN subscription_plan = 'Plus' THEN 250
        WHEN subscription_plan = 'Pro' THEN 1000
        ELSE 5
    END,
    alerts_limit = CASE 
        WHEN subscription_plan = 'Plus' THEN 100
        WHEN subscription_plan = 'Pro' THEN 400
        ELSE 0
    END,
    strategy_alerts_limit = CASE 
        WHEN subscription_plan = 'Plus' THEN 5
        WHEN subscription_plan = 'Pro' THEN 20
        ELSE 0
    END
WHERE queries_limit IS NULL OR queries_limit = 5; -- Only update if not already set

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    39,
    'Add usage tracking and limits to users table for subscription enforcement'
) ON CONFLICT (version) DO NOTHING;

COMMIT;