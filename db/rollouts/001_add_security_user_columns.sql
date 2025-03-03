-- Migration: 001_add_security_user_columns
-- Description: Adds additional columns to securities and users tables
ALTER TABLE securities
ADD COLUMN IF NOT EXISTS name varchar(200),
    ADD COLUMN IF NOT EXISTS market varchar(50),
    ADD COLUMN IF NOT EXISTS locale varchar(50),
    ADD COLUMN IF NOT EXISTS primary_exchange varchar(50),
    ADD COLUMN IF NOT EXISTS active boolean DEFAULT true,
    ADD COLUMN IF NOT EXISTS market_cap decimal(20, 2),
    ADD COLUMN IF NOT EXISTS description text,
    ADD COLUMN IF NOT EXISTS logo text,
    ADD COLUMN IF NOT EXISTS icon text,
    ADD COLUMN IF NOT EXISTS share_class_shares_outstanding bigint,
    ADD COLUMN IF NOT EXISTS total_shares BIGINT;
ALTER TABLE users
ADD COLUMN IF NOT EXISTS profile_picture TEXT;