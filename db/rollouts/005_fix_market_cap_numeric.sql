-- Convert market_cap from BIGINT to NUMERIC to handle arbitrarily large values with precision
-- Description: Change market_cap to NUMERIC type for arbitrary precision
ALTER TABLE securities
ALTER COLUMN market_cap TYPE NUMERIC USING market_cap::NUMERIC;