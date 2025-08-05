-- Migration: 040_replace_queries_with_credits_system
-- Description: Replace queries_used/queries_limit with credits system that tracks subscription and purchased credits separately

BEGIN;

-- Add new credit tracking columns
ALTER TABLE users 
ADD COLUMN IF NOT EXISTS subscription_credits_remaining INTEGER DEFAULT 0,
ADD COLUMN IF NOT EXISTS purchased_credits_remaining INTEGER DEFAULT 0,
ADD COLUMN IF NOT EXISTS total_credits_remaining INTEGER GENERATED ALWAYS AS (subscription_credits_remaining + purchased_credits_remaining) STORED;

-- Add column to track subscription credits allocated per billing period
ALTER TABLE users 
ADD COLUMN IF NOT EXISTS subscription_credits_allocated INTEGER DEFAULT 0;

-- Update existing users with subscription credits based on their current plan
UPDATE users SET 
    subscription_credits_allocated = CASE 
        WHEN subscription_plan = 'Plus' THEN 250
        WHEN subscription_plan = 'Pro' THEN 1000
        ELSE 5
    END,
    subscription_credits_remaining = CASE 
        WHEN subscription_plan = 'Plus' THEN GREATEST(0, 250 - COALESCE(queries_used, 0))
        WHEN subscription_plan = 'Pro' THEN GREATEST(0, 1000 - COALESCE(queries_used, 0))
        ELSE GREATEST(0, 5 - COALESCE(queries_used, 0))
    END,
    purchased_credits_remaining = 0;

-- Update plan_limits table to use credits instead of queries
ALTER TABLE plan_limits 
ADD COLUMN IF NOT EXISTS credits_per_billing_period INTEGER DEFAULT 0;

-- Update plan limits with credit allocations
UPDATE plan_limits SET 
    credits_per_billing_period = queries_limit;

-- Create indexes for efficient querying of credits
CREATE INDEX IF NOT EXISTS idx_users_subscription_credits ON users(subscription_credits_remaining);
CREATE INDEX IF NOT EXISTS idx_users_purchased_credits ON users(purchased_credits_remaining);
CREATE INDEX IF NOT EXISTS idx_users_total_credits ON users(total_credits_remaining);

-- Update usage_logs to track credit consumption instead of queries
ALTER TABLE usage_logs 
ADD COLUMN IF NOT EXISTS credits_consumed INTEGER DEFAULT 1,
ADD COLUMN IF NOT EXISTS credits_source VARCHAR(20) DEFAULT 'subscription'; -- 'subscription' or 'purchased'

-- Create function to consume credits (subscription first, then purchased)
CREATE OR REPLACE FUNCTION consume_user_credits(user_id_param INTEGER, credits_to_consume INTEGER) 
RETURNS TABLE(success BOOLEAN, remaining_credits INTEGER, source_used VARCHAR(20)) AS $$
DECLARE
    current_subscription_credits INTEGER;
    current_purchased_credits INTEGER;
    credits_from_subscription INTEGER := 0;
    credits_from_purchased INTEGER := 0;
BEGIN
    -- Get current credit balances
    SELECT subscription_credits_remaining, purchased_credits_remaining 
    INTO current_subscription_credits, current_purchased_credits
    FROM users WHERE userId = user_id_param;
    
    -- Check if user has enough total credits
    IF (current_subscription_credits + current_purchased_credits) < credits_to_consume THEN
        RETURN QUERY SELECT FALSE, (current_subscription_credits + current_purchased_credits), 'insufficient'::VARCHAR(20);
        RETURN;
    END IF;
    
    -- Consume from subscription credits first
    IF current_subscription_credits >= credits_to_consume THEN
        credits_from_subscription := credits_to_consume;
        credits_from_purchased := 0;
    ELSE
        credits_from_subscription := current_subscription_credits;
        credits_from_purchased := credits_to_consume - current_subscription_credits;
    END IF;
    
    -- Update user credits
    UPDATE users SET 
        subscription_credits_remaining = subscription_credits_remaining - credits_from_subscription,
        purchased_credits_remaining = purchased_credits_remaining - credits_from_purchased
    WHERE userId = user_id_param;
    
    -- Return success and remaining credits
    RETURN QUERY SELECT TRUE, 
                       (current_subscription_credits + current_purchased_credits - credits_to_consume),
                       CASE WHEN credits_from_purchased > 0 THEN 'both' ELSE 'subscription' END::VARCHAR(20);
END;
$$ LANGUAGE plpgsql;

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    40,
    'Replace queries system with credits system that tracks subscription and purchased credits separately'
) ON CONFLICT (version) DO NOTHING;

COMMIT; 