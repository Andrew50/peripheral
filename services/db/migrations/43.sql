-- Migration: 043_insert_pricing_configuration_data
-- Description: Insert subscription plans and credit products configuration data

BEGIN;

-- Insert subscription plans
INSERT INTO subscription_plans (
    plan_name, 
    stripe_price_id_test, 
    stripe_price_id_live, 
    display_name, 
    description, 
    price_cents, 
    billing_period, 
    credits_per_billing_period, 
    alerts_limit, 
    strategy_alerts_limit, 
    features, 
    is_active, 
    is_popular, 
    sort_order
) VALUES 
(
    'Free',
    NULL,
    NULL,
    'Free',
    'Basic access to get started',
    0,
    'month',
    5,
    0,
    0,
    '["Delayed charting", "5 credits", "Watchlists"]'::jsonb,
    TRUE,
    FALSE,
    1
),
(
    'Plus',
    'price_1RhAuSGCLCMUwjFlh3wPqWyo',
    'price_1Rgsj1GCLCMUwjFlf4U6jMRt',
    'Plus',
    'Perfect for active traders',
    9900, -- $99.00 in cents
    'month',
    250,
    100,
    5,
    '["Realtime charting", "250 credits", "5 strategy alerts", "Single strategy screening", "100 news or price alerts"]'::jsonb,
    TRUE,
    FALSE,
    2
),
(
    'Pro',
    'price_1RhAucGCLCMUwjFli0rmWtIe',
    'price_1RgsiRGCLCMUwjFljLLknvSu',
    'Pro',
    'Advanced features for professional traders',
    19900, -- $199.00 in cents
    'month',
    1000,
    400,
    20,
    '["Sub 1 minute charting", "Multi chart", "1000 credits", "20 strategy alerts", "Multi strategy screening", "400 alerts", "Watchlist alerts"]'::jsonb,
    TRUE,
    TRUE,
    3
)
ON CONFLICT (plan_name) DO UPDATE SET
    stripe_price_id_test = EXCLUDED.stripe_price_id_test,
    stripe_price_id_live = EXCLUDED.stripe_price_id_live,
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description,
    price_cents = EXCLUDED.price_cents,
    billing_period = EXCLUDED.billing_period,
    credits_per_billing_period = EXCLUDED.credits_per_billing_period,
    alerts_limit = EXCLUDED.alerts_limit,
    strategy_alerts_limit = EXCLUDED.strategy_alerts_limit,
    features = EXCLUDED.features,
    is_active = EXCLUDED.is_active,
    is_popular = EXCLUDED.is_popular,
    sort_order = EXCLUDED.sort_order,
    updated_at = CURRENT_TIMESTAMP;

-- Insert credit products
INSERT INTO credit_products (
    product_key,
    stripe_price_id_test,
    stripe_price_id_live,
    display_name,
    description,
    credit_amount,
    price_cents,
    is_active,
    is_popular,
    sort_order
) VALUES 
(
    'credits100',
    'price_1RhJ6eGCLCMUwjFlE66c7kD0',
    'price_1RhJ3bGCLCMUwjFlUrFgdSrd',
    '100 Credits',
    'Perfect for light usage',
    100,
    1000, -- $10.00 in cents
    TRUE,
    FALSE,
    1
),
(
    'credits250',
    'price_1RhJ6uGCLCMUwjFlLvvMcyDb',
    'price_1RhJ3xGCLCMUwjFlH3sTP1vk',
    '250 Credits',
    'Great value for regular users',
    250,
    2500, -- $25.00 in cents
    TRUE,
    TRUE,
    2
),
(
    'credits1000',
    'price_1RhJ7AGCLCMUwjFlxhkchyJM',
    'price_1RhJ4HGCLCMUwjFlFxij7ktv',
    '1000 Credits',
    'Maximum value for power users',
    1000,
    5000, -- $50.00 in cents
    TRUE,
    FALSE,
    3
)
ON CONFLICT (product_key) DO UPDATE SET
    stripe_price_id_test = EXCLUDED.stripe_price_id_test,
    stripe_price_id_live = EXCLUDED.stripe_price_id_live,
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description,
    credit_amount = EXCLUDED.credit_amount,
    price_cents = EXCLUDED.price_cents,
    is_active = EXCLUDED.is_active,
    is_popular = EXCLUDED.is_popular,
    sort_order = EXCLUDED.sort_order,
    updated_at = CURRENT_TIMESTAMP;

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    43,
    'Insert subscription plans and credit products configuration data'
) ON CONFLICT (version) DO NOTHING;

COMMIT; 