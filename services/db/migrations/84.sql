<<<<<<< HEAD

BEGIN;

-- Drop the existing unique constraint on (userId, name)
ALTER TABLE strategies DROP CONSTRAINT IF EXISTS strategies_userid_name_key;

-- Add new unique constraint on (userId, name, version) - only if it doesn't exist
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints 
        WHERE constraint_name = 'strategies_userid_name_version_key' 
        AND table_name = 'strategies'
    ) THEN
        ALTER TABLE strategies ADD CONSTRAINT strategies_userid_name_version_key 
            UNIQUE (userId, name, version);
    END IF;
END $$;

-- Create index for efficient querying of strategy versions
CREATE INDEX IF NOT EXISTS idx_strategies_user_name_version ON strategies(userId, name, version DESC);
=======
-- Migration 084: Update queries limit for Free and Plus subscription products
-- Description: Increase Free plan queries_limit from 5 to 15 and Plus plan queries_limit from 250 to 300

BEGIN;

-- Update Free plan (id=1) queries_limit from 5 to 15
UPDATE subscription_products 
SET queries_limit = 15, updated_at = CURRENT_TIMESTAMP
WHERE id = 1 AND product_key = 'Free';

-- Update Plus plan (id=2) queries_limit from 250 to 300
UPDATE subscription_products 
SET queries_limit = 300, updated_at = CURRENT_TIMESTAMP
WHERE id = 2 AND product_key = 'Plus';
>>>>>>> 859a6a0cc9c732c8cb778b5ccb7aa9ee4d484906

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    84,
<<<<<<< HEAD
    'Add alert_threshold and alert_universe columns to strategies table'
=======
    'Update queries_limit for Free (5->15) and Plus (250->300) subscription products'
>>>>>>> 859a6a0cc9c732c8cb778b5ccb7aa9ee4d484906
) ON CONFLICT (version) DO NOTHING;

COMMIT; 