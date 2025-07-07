-- Migration: 051_update_yearly_plan_prices
-- Description: Adjust yearly Plus and Pro subscription plan pricing (Plus → $1,020, Pro → $1,920)

BEGIN;

-- Update Pro Yearly pricing
UPDATE subscription_plans
SET price_cents = 192000,          -- $1,920.00 in cents
    updated_at  = CURRENT_TIMESTAMP
WHERE plan_name = 'Pro Yearly';

-- Update Plus Yearly pricing
UPDATE subscription_plans
SET price_cents = 102000,          -- $1,020.00 in cents
    updated_at  = CURRENT_TIMESTAMP
WHERE plan_name = 'Plus Yearly';

-- Record schema version
INSERT INTO schema_versions (version, description)
VALUES (51, 'Update yearly Plus and Pro plan prices to 1020 and 1920 USD')
ON CONFLICT (version) DO NOTHING;

COMMIT; 