-- Migration: 049_insert_yearly_pricing_tiers
-- Description: Add yearly Plus and Pro subscription plans with two months free (annual billing)

BEGIN;

-- Insert yearly subscription plans
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
-- Plus Yearly (2 months free)
(
    'Plus Yearly',
    'price_1RhmwGGCLCMUwjFly2UYM7wo', -- Test mode price ID
    'price_1RhmtGGCLCMUwjFlzcETQoMj', -- Live price ID
    'Plus (Yearly)',
    'Perfect for active traders – save 2 months with annual billing',
    99900, -- $999.00 in cents (10 × $99.90)
    'year',
    250,  -- 250 credits × 12 months
    100,  -- 100 alerts × 12 months
    5,    -- 5 strategy alerts × 12 months
    '["Realtime charting", "3000 credits / yr", "5 strategy alerts", "Single strategy screening", "100 news or price alerts", "2 months free"]'::jsonb,
    TRUE,
    FALSE,
    4
),
-- Pro Yearly (2 months free)
(
    'Pro Yearly',
    'price_1RhmvlGCLCMUwjFlMML9PGH6', -- Test mode price ID
    'price_1RhmsCGCLCMUwjFlxUrjt27k', -- Live price ID
    'Pro (Yearly)',
    'Advanced features for professional traders – save 2 months with annual billing',
    199900, -- $1,999.00 in cents (10 × $199.90)
    'year',
    1000, -- 1000 credits × 12 months
    400,  -- 400 alerts × 12 months
    20,   -- 20 strategy alerts × 12 months
    '["Sub 1 minute charting", "Multi chart", "12000 credits / yr", "20 strategy alerts", "Multi strategy screening", "400 alerts", "Watchlist alerts", "2 months free"]'::jsonb,
    TRUE,
    TRUE,
    5
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

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    49,
    'Insert yearly Plus and Pro subscription plans'
) ON CONFLICT (version) DO NOTHING;

COMMIT; 