-- Migration 56: Update pricing structure
-- Drop stripe_price_id columns from credit_products, 
-- replace product_id with product_key in prices table,
-- and populate with new pricing data

BEGIN;

-- Drop stripe price columns from credit_products
ALTER TABLE credit_products DROP COLUMN IF EXISTS stripe_price_id_test;
ALTER TABLE credit_products DROP COLUMN IF EXISTS stripe_price_id_live;
ALTER TABLE credit_products DROP COLUMN IF EXISTS is_active;

-- Truncate the prices table first to avoid NULL constraint issues
TRUNCATE TABLE prices;

-- Replace product_id with product_key in prices table
-- First, add the new column
ALTER TABLE prices ADD COLUMN product_key VARCHAR(50);

-- Drop the old column and constraints
ALTER TABLE prices DROP COLUMN product_id;

-- Make product_key NOT NULL (safe now since table is empty)
ALTER TABLE prices ALTER COLUMN product_key SET NOT NULL;

-- Drop old indexes
DROP INDEX IF EXISTS idx_prices_product_id;

-- Create new index for product_key
CREATE INDEX idx_prices_product_key ON prices(product_key);

-- Insert the new pricing data
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
VALUES (56, 'Update pricing structure: drop stripe columns from credit_products, replace product_id with product_key in prices, populate new pricing data')
ON CONFLICT (version) DO NOTHING;

COMMIT; 