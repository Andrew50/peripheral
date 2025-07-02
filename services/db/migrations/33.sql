-- Migration 033: Ensure strategies table compatibility with strategy_generator.py
-- Description: Add missing columns and ensure naming compatibility

BEGIN;

-- Add any missing columns that strategy_generator.py expects
-- These may already exist from previous migrations, so use IF NOT EXISTS

ALTER TABLE strategies
ADD COLUMN IF NOT EXISTS description TEXT,
ADD COLUMN IF NOT EXISTS prompt TEXT,
ADD COLUMN IF NOT EXISTS pythoncode TEXT,
ADD COLUMN IF NOT EXISTS score INTEGER DEFAULT 0,
ADD COLUMN IF NOT EXISTS version VARCHAR(20) DEFAULT '1.0',
ADD COLUMN IF NOT EXISTS createdat TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
ADD COLUMN IF NOT EXISTS isalertactive BOOLEAN DEFAULT FALSE;

-- Copy data from camelCase columns to lowercase columns if they exist
-- This ensures compatibility with strategy_generator.py which uses lowercase

-- Copy pythonCode to pythoncode if pythonCode exists
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name = 'strategies' AND column_name = 'pythonCode') THEN
        UPDATE strategies 
        SET pythoncode = COALESCE(pythoncode, "pythonCode")
        WHERE "pythonCode" IS NOT NULL AND pythoncode IS NULL;
    END IF;
END $$;

-- Copy createdAt to createdat if createdAt exists
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name = 'strategies' AND column_name = 'createdAt') THEN
        UPDATE strategies 
        SET createdat = COALESCE(createdat, "createdAt")
        WHERE "createdAt" IS NOT NULL AND createdat IS NULL;
    END IF;
END $$;

-- Copy isAlertActive to isalertactive if isAlertActive exists
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name = 'strategies' AND column_name = 'isAlertActive') THEN
        UPDATE strategies 
        SET isalertactive = COALESCE(isalertactive, "isAlertActive")
        WHERE isalertactive IS NULL;
    END IF;
END $$;

-- Also sync alertActive with isalertactive for backward compatibility
UPDATE strategies 
SET isalertactive = COALESCE(isalertactive, alertActive)
WHERE isalertactive IS NULL;

-- Create trigger to update updated_at timestamp if it doesn't exist
CREATE OR REPLACE FUNCTION update_strategies_updated_at() RETURNS TRIGGER AS $$ 
BEGIN 
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Drop existing trigger if it exists and recreate
DROP TRIGGER IF EXISTS trigger_strategies_updated_at ON strategies;
CREATE TRIGGER trigger_strategies_updated_at 
    BEFORE UPDATE ON strategies 
    FOR EACH ROW EXECUTE FUNCTION update_strategies_updated_at();

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_strategies_createdat ON strategies(createdat DESC);
CREATE INDEX IF NOT EXISTS idx_strategies_updated_at ON strategies(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_strategies_isalertactive ON strategies(isalertactive) WHERE isalertactive = true;
CREATE INDEX IF NOT EXISTS idx_strategies_user_createdat ON strategies(userId, createdat DESC);
CREATE INDEX IF NOT EXISTS idx_strategies_score ON strategies(score DESC);
CREATE INDEX IF NOT EXISTS idx_strategies_version ON strategies(version);

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    33,
    'Ensure strategies table compatibility with strategy_generator.py'
) ON CONFLICT (version) DO NOTHING;

COMMIT; 