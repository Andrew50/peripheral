-- Migration 58: Fix prices table corruption from migration 56
-- The previous migration 56 left the prices table in an inconsistent state
-- This migration safely recreates the table with the correct structure

BEGIN;

-- Drop the potentially corrupted prices table
DROP TABLE IF EXISTS prices CASCADE;

-- Recreate the prices table with the correct structure from migration 56
CREATE TABLE prices (
    id SERIAL PRIMARY KEY,
    price_cents INTEGER NOT NULL,
    stripe_price_id_live VARCHAR(255),
    stripe_price_id_test VARCHAR(255),
    product_key VARCHAR(50) NOT NULL,
    billing_period VARCHAR(20) NOT NULL CHECK (billing_period IN ('monthly', 'yearly', 'single')),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for performance
CREATE INDEX idx_prices_stripe_price_id_live ON prices(stripe_price_id_live);
CREATE INDEX idx_prices_stripe_price_id_test ON prices(stripe_price_id_test);
CREATE INDEX idx_prices_product_key ON prices(product_key);
CREATE INDEX idx_prices_billing_period ON prices(billing_period);

-- Insert the correct pricing data (from migration 56)
INSERT INTO prices (price_cents, stripe_price_id_live, stripe_price_id_test, product_key, billing_period) VALUES
-- Credit products
(1000, 'price_1RhJ3bGCLCMUwjFlUrFgdSrd', 'price_1RhJ6eGCLCMUwjFlE66c7kD0', 'credits100', 'single'),
(2000, 'price_1RhJ3xGCLCMUwjFlH3sTP1vk', 'price_1RhJ6uGCLCMUwjFlLvvMcyDb', 'credits250', 'single'),
(5000, 'price_1RhJ4HGCLCMUwjFlFxij7ktv', 'price_1RhJ7AGCLCMUwjFlxhkchyJM', 'credits1000', 'single'),
-- Subscription products
(19900, 'price_1RgsiRGCLCMUwjFljLLknvSu', 'price_1RhAucGCLCMUwjFli0rmWtIe', 'Pro', 'monthly'),
(199900, 'price_1RhmsCGCLCMUwjFlxUrjt27k', 'price_1RhmvlGCLCMUwjFlMML9PGH6', 'Pro', 'yearly'),
(9900, 'price_1Rgsj1GCLCMUwjFlf4U6jMRt', 'price_1RhAuSGCLCMUwjFlh3wPqWyo', 'Plus', 'monthly'),
(99900, 'price_1RhmtGGCLCMUwjFlzcETQoMj', 'price_1RhmwGGCLCMUwjFly2UYM7wo', 'Plus', 'yearly');

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (58, 'Fix prices table corruption from migration 56 by recreating table with correct structure')
ON CONFLICT (version) DO NOTHING;

COMMIT;