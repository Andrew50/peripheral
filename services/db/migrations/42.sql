-- Migration: 042_create_pricing_configuration_tables
-- Description: Create database tables for storing subscription plans and credit products configuration

BEGIN;

-- Create subscription_plans table to store subscription plan configurations
CREATE TABLE IF NOT EXISTS subscription_plans (
    id SERIAL PRIMARY KEY,
    plan_name VARCHAR(50) NOT NULL UNIQUE,
    stripe_price_id_test VARCHAR(255), -- Test/development mode price ID
    stripe_price_id_live VARCHAR(255), -- Production mode price ID
    display_name VARCHAR(100) NOT NULL,
    description TEXT,
    price_cents INTEGER NOT NULL, -- Price in cents
    billing_period VARCHAR(20) NOT NULL DEFAULT 'month', -- 'month', 'year', etc.
    credits_per_billing_period INTEGER NOT NULL DEFAULT 0,
    alerts_limit INTEGER NOT NULL DEFAULT 0,
    strategy_alerts_limit INTEGER NOT NULL DEFAULT 0,
    features JSONB DEFAULT '[]'::jsonb, -- Array of feature strings
    is_active BOOLEAN DEFAULT TRUE,
    is_popular BOOLEAN DEFAULT FALSE,
    sort_order INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create credit_products table to store credit purchase configurations
CREATE TABLE IF NOT EXISTS credit_products (
    id SERIAL PRIMARY KEY,
    product_key VARCHAR(50) NOT NULL UNIQUE,
    stripe_price_id_test VARCHAR(255), -- Test/development mode price ID
    stripe_price_id_live VARCHAR(255), -- Production mode price ID
    display_name VARCHAR(100) NOT NULL,
    description TEXT,
    credit_amount INTEGER NOT NULL,
    price_cents INTEGER NOT NULL, -- Price in cents
    is_active BOOLEAN DEFAULT TRUE,
    is_popular BOOLEAN DEFAULT FALSE,
    sort_order INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_subscription_plans_plan_name ON subscription_plans(plan_name);
CREATE INDEX IF NOT EXISTS idx_subscription_plans_stripe_price_test ON subscription_plans(stripe_price_id_test);
CREATE INDEX IF NOT EXISTS idx_subscription_plans_stripe_price_live ON subscription_plans(stripe_price_id_live);
CREATE INDEX IF NOT EXISTS idx_subscription_plans_active ON subscription_plans(is_active);
CREATE INDEX IF NOT EXISTS idx_subscription_plans_sort_order ON subscription_plans(sort_order);

CREATE INDEX IF NOT EXISTS idx_credit_products_product_key ON credit_products(product_key);
CREATE INDEX IF NOT EXISTS idx_credit_products_stripe_price_test ON credit_products(stripe_price_id_test);
CREATE INDEX IF NOT EXISTS idx_credit_products_stripe_price_live ON credit_products(stripe_price_id_live);
CREATE INDEX IF NOT EXISTS idx_credit_products_active ON credit_products(is_active);
CREATE INDEX IF NOT EXISTS idx_credit_products_sort_order ON credit_products(sort_order);

-- Create triggers to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_subscription_plans_updated_at() RETURNS TRIGGER AS $$ 
BEGIN 
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION update_credit_products_updated_at() RETURNS TRIGGER AS $$ 
BEGIN 
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_subscription_plans_updated_at ON subscription_plans;
CREATE TRIGGER trigger_subscription_plans_updated_at 
    BEFORE UPDATE ON subscription_plans 
    FOR EACH ROW EXECUTE FUNCTION update_subscription_plans_updated_at();

DROP TRIGGER IF EXISTS trigger_credit_products_updated_at ON credit_products;
CREATE TRIGGER trigger_credit_products_updated_at 
    BEFORE UPDATE ON credit_products 
    FOR EACH ROW EXECUTE FUNCTION update_credit_products_updated_at();

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    42,
    'Create database tables for storing subscription plans and credit products configuration'
) ON CONFLICT (version) DO NOTHING;

COMMIT; 