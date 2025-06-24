-- Migration: 029_add_system_user
-- Description: Add system user with ID 0 for public chart access

BEGIN;

-- Insert a system user with ID 0 for public/unauthenticated access
-- This prevents foreign key constraint violations when logging public chart queries
INSERT INTO users (userId, username, password, email, auth_type, settings)
VALUES (
    0, 
    'public@atlantis.trading', 
    'NO_PASSWORD', 
    'public@atlantis.trading',
    'system',
    '{"description": "System user for public chart access and analytics"}'::json
)
ON CONFLICT (userId) DO NOTHING;

-- Ensure the sequence doesn't conflict with the manually inserted ID 0
-- Reset the sequence to start from 1 if it's currently at 0
SELECT CASE 
    WHEN last_value = 0 
    THEN setval('users_userid_seq', 1, false)
    ELSE setval('users_userid_seq', greatest(last_value, 1), true)
END
FROM users_userid_seq;

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (29, 'Add system user with ID 0 for public chart access')
ON CONFLICT (version) DO NOTHING;

COMMIT; 