-- Migration: 087_email_verification
-- Description: Adds columns for email verification of users

BEGIN;

ALTER TABLE users
ADD COLUMN verified BOOLEAN NOT NULL DEFAULT FALSE,
ADD COLUMN otp_code VARCHAR(10), 
ADD COLUMN otp_code INTEGER,
ADD COLUMN otp_expires_at TIMESTAMP;

COMMIT;