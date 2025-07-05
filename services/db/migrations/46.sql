-- Migration: 046_add_subscription_usage_index
-- Description: Add composite index for subscription and usage queries to improve performance

BEGIN;

-- Add composite index for subscription and usage data queries
-- This index covers the most commonly queried columns in GetCombinedSubscriptionAndUsage
CREATE INDEX IF NOT EXISTS idx_users_subscription_usage_composite 
ON users(userId, subscription_status, subscription_plan, subscription_credits_remaining, 
         purchased_credits_remaining, total_credits_remaining, subscription_credits_allocated,
         active_alerts, alerts_limit, active_strategy_alerts, strategy_alerts_limit);

-- Add index for stripe-related queries
CREATE INDEX IF NOT EXISTS idx_users_stripe_composite 
ON users(userId, stripe_customer_id, stripe_subscription_id, current_period_end);

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    46,
    'Add composite indexes for subscription and usage queries to improve performance'
) ON CONFLICT (version) DO NOTHING;

COMMIT; 