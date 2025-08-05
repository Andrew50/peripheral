-- Migration 55: Restructure pricing tables
-- Remove display fields from credit_products, rename plan_limits to subscription_products,
-- remove subscription_plans table, and add prices table

BEGIN;

-- Create new prices table
CREATE TABLE prices (
    id SERIAL PRIMARY KEY,
    price_cents INTEGER NOT NULL,
    stripe_price_id_live VARCHAR(255),
    stripe_price_id_test VARCHAR(255),
    product_id INTEGER NOT NULL,
    billing_period VARCHAR(20) NOT NULL CHECK (billing_period IN ('monthly', 'yearly', 'single')),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Add indexes for performance
CREATE INDEX idx_prices_stripe_price_id_live ON prices(stripe_price_id_live);
CREATE INDEX idx_prices_stripe_price_id_test ON prices(stripe_price_id_test);
CREATE INDEX idx_prices_product_id ON prices(product_id);
CREATE INDEX idx_prices_billing_period ON prices(billing_period);

-- Migrate data from subscription_plans to prices table
INSERT INTO prices (price_cents, stripe_price_id_live, stripe_price_id_test, product_id, billing_period)
SELECT 
    sp.price_cents,
    sp.stripe_price_id_live,
    sp.stripe_price_id_test,
    pl.id as product_id,
    CASE 
        WHEN sp.billing_period = 'month' THEN 'monthly'
        WHEN sp.billing_period = 'year' THEN 'yearly'
        ELSE sp.billing_period
    END as billing_period
FROM subscription_plans sp
JOIN plan_limits pl ON sp.plan_name = pl.plan_name
WHERE sp.is_active = true;

-- Migrate credit product prices to prices table
INSERT INTO prices (price_cents, stripe_price_id_live, stripe_price_id_test, product_id, billing_period)
SELECT 
    cp.price_cents,
    cp.stripe_price_id_live,
    cp.stripe_price_id_test,
    cp.id as product_id,
    'single' as billing_period
FROM credit_products cp
WHERE cp.is_active = true;

-- Rename plan_limits table to subscription_products and update columns
ALTER TABLE plan_limits RENAME TO subscription_products;
ALTER TABLE subscription_products RENAME COLUMN plan_name TO product_key;
ALTER TABLE subscription_products RENAME COLUMN credits_per_billing_period TO credits_per_month;

-- Remove columns from credit_products
ALTER TABLE credit_products DROP COLUMN display_name;
ALTER TABLE credit_products DROP COLUMN description;
ALTER TABLE credit_products DROP COLUMN price_cents;
ALTER TABLE credit_products DROP COLUMN is_popular;
ALTER TABLE credit_products DROP COLUMN sort_order;

-- Drop subscription_plans table
DROP TABLE subscription_plans;

COMMIT; 