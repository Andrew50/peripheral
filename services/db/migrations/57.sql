-- Migration 57: Remove duplicate credits_per_month column
-- The credits_per_month column is a duplicate of queries_limit
-- This migration removes the duplicate column

BEGIN;

-- Drop the credits_per_month column from subscription_products
ALTER TABLE subscription_products DROP COLUMN IF EXISTS credits_per_month;

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (57, 'Remove duplicate credits_per_month column from subscription_products table')
ON CONFLICT (version) DO NOTHING;

COMMIT; 