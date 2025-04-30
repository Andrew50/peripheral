-- Add market_cap and share_class_shares_outstanding to daily_ohlcv

BEGIN;

ALTER TABLE IF EXISTS daily_ohlcv
ADD COLUMN IF NOT EXISTS market_cap DECIMAL(25, 6),
ADD COLUMN IF NOT EXISTS share_class_shares_outstanding BIGINT;

COMMIT; 