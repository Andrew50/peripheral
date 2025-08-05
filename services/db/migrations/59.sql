-- Migration: 054_add_signup_coupon_code
-- Description: Add signup_coupon_code column to users table for tracking promo codes used during signup

BEGIN;

-- Add signup_coupon_code column to users table
ALTER TABLE users 
ADD COLUMN IF NOT EXISTS signup_coupon_code VARCHAR(100);

-- Create index for efficient querying of coupon usage
CREATE INDEX IF NOT EXISTS idx_users_signup_coupon_code ON users(signup_coupon_code) 
WHERE signup_coupon_code IS NOT NULL;

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    59,
    'Add signup_coupon_code column to users table for promo code tracking'
) ON CONFLICT (version) DO NOTHING;

COMMIT;