-- Migration: 036_add_stripe_billing_columns
-- Description: Add Stripe-related columns to users table for subscription management

BEGIN;

-- Add Stripe-related columns to users table
ALTER TABLE users 
ADD COLUMN IF NOT EXISTS stripe_customer_id VARCHAR(255),
ADD COLUMN IF NOT EXISTS stripe_subscription_id VARCHAR(255),
ADD COLUMN IF NOT EXISTS subscription_status VARCHAR(50) DEFAULT 'inactive',
ADD COLUMN IF NOT EXISTS current_period_end TIMESTAMP WITH TIME ZONE,
ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP;

-- Create indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_users_stripe_customer_id ON users(stripe_customer_id);
CREATE INDEX IF NOT EXISTS idx_users_stripe_subscription_id ON users(stripe_subscription_id);
CREATE INDEX IF NOT EXISTS idx_users_subscription_status ON users(subscription_status);

-- Create trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_users_updated_at() RETURNS TRIGGER AS $$ 
BEGIN 
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Drop existing trigger if it exists and recreate
DROP TRIGGER IF EXISTS trigger_users_updated_at ON users;
CREATE TRIGGER trigger_users_updated_at 
    BEFORE UPDATE ON users 
    FOR EACH ROW EXECUTE FUNCTION update_users_updated_at();

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    36,
    'Add Stripe-related columns to users table for subscription management'
) ON CONFLICT (version) DO NOTHING;

COMMIT;
