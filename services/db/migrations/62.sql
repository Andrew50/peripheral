-- Migration: 062_rename_signup_coupon_code_to_invite_code_used
-- Description: Rename signup_coupon_code column to invite_code_used on users table and update related index

BEGIN;

-- Rename the column if it exists and new column does not already exist
ALTER TABLE users RENAME COLUMN signup_coupon_code TO invite_code_used;

-- Drop old index (if it exists) and create new index on the renamed column
DROP INDEX IF EXISTS idx_users_signup_coupon_code;
CREATE INDEX IF NOT EXISTS idx_users_invite_code_used 
    ON users(invite_code_used) 
    WHERE invite_code_used IS NOT NULL;

-- Record schema version
INSERT INTO schema_versions (version, description)
VALUES (
    62,
    'Rename signup_coupon_code column to invite_code_used on users table'
) ON CONFLICT (version) DO NOTHING;

COMMIT; 