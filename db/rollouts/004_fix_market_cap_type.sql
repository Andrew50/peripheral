-- Convert market_cap from decimal to BIGINT to handle large values
-- Description: Fix market_cap and share_class_shares_outstanding types to handle large values
ALTER TABLE securities
ALTER COLUMN market_cap TYPE BIGINT USING market_cap::BIGINT;
-- Convert share_class_shares_outstanding to BIGINT
ALTER TABLE securities
ALTER COLUMN share_class_shares_outstanding TYPE BIGINT;