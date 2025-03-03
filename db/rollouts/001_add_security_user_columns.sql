-- Migration: 001_add_security_user_columns
-- Description: Adds additional columns to securities and users tables
-- Use DO blocks with exception handling for more resilient migrations
DO $$ BEGIN -- Add columns to securities table if they don't exist
BEGIN
ALTER TABLE securities
ADD COLUMN IF NOT EXISTS name varchar(200);
EXCEPTION
WHEN duplicate_column THEN -- Column already exists, do nothing
END;
BEGIN
ALTER TABLE securities
ADD COLUMN IF NOT EXISTS market varchar(50);
EXCEPTION
WHEN duplicate_column THEN -- Column already exists, do nothing
END;
BEGIN
ALTER TABLE securities
ADD COLUMN IF NOT EXISTS locale varchar(50);
EXCEPTION
WHEN duplicate_column THEN -- Column already exists, do nothing
END;
BEGIN
ALTER TABLE securities
ADD COLUMN IF NOT EXISTS primary_exchange varchar(50);
EXCEPTION
WHEN duplicate_column THEN -- Column already exists, do nothing
END;
BEGIN
ALTER TABLE securities
ADD COLUMN IF NOT EXISTS active boolean DEFAULT true;
EXCEPTION
WHEN duplicate_column THEN -- Column already exists, do nothing
END;
BEGIN
ALTER TABLE securities
ADD COLUMN IF NOT EXISTS market_cap decimal(20, 2);
EXCEPTION
WHEN duplicate_column THEN -- Column already exists, do nothing
END;
BEGIN
ALTER TABLE securities
ADD COLUMN IF NOT EXISTS description text;
EXCEPTION
WHEN duplicate_column THEN -- Column already exists, do nothing
END;
BEGIN
ALTER TABLE securities
ADD COLUMN IF NOT EXISTS logo text;
EXCEPTION
WHEN duplicate_column THEN -- Column already exists, do nothing
END;
BEGIN
ALTER TABLE securities
ADD COLUMN IF NOT EXISTS icon text;
EXCEPTION
WHEN duplicate_column THEN -- Column already exists, do nothing
END;
BEGIN
ALTER TABLE securities
ADD COLUMN IF NOT EXISTS share_class_shares_outstanding bigint;
EXCEPTION
WHEN duplicate_column THEN -- Column already exists, do nothing
END;
BEGIN
ALTER TABLE securities
ADD COLUMN IF NOT EXISTS total_shares BIGINT;
EXCEPTION
WHEN duplicate_column THEN -- Column already exists, do nothing
END;
-- Add column to users table if it doesn't exist
BEGIN
ALTER TABLE users
ADD COLUMN IF NOT EXISTS profile_picture TEXT;
EXCEPTION
WHEN duplicate_column THEN -- Column already exists, do nothing
END;
END $$;