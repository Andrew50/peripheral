-- Migration: 091_add_email_verification_columns
-- Description: Adds columns for email verification of users

BEGIN;

ALTER TABLE users
ADD COLUMN verified BOOLEAN NOT NULL DEFAULT FALSE,
ADD COLUMN otp_code VARCHAR(10), 
ADD COLUMN otp_code INTEGER,
ADD COLUMN otp_expires_at TIMESTAMP;

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    91,
    'Add email verification columns'
) ON CONFLICT (version) DO NOTHING;

COMMIT;