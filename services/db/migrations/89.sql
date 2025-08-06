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

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    89,
    'Add unique constraint on (userId, name, version) to strategies table'
) ON CONFLICT (version) DO NOTHING;

COMMIT; 