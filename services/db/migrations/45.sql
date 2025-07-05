-- Migration: 045_remove_user_limits_trigger_and_function
-- Description: Remove the trigger_update_user_limits trigger and update_user_limits_on_subscription_change function

BEGIN;

-- Drop the trigger first (before dropping the function)
DROP TRIGGER IF EXISTS trigger_update_user_limits ON users;

-- Drop the function
DROP FUNCTION IF EXISTS update_user_limits_on_subscription_change();

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    45,
    'Remove trigger_update_user_limits trigger and update_user_limits_on_subscription_change function'
) ON CONFLICT (version) DO NOTHING;

COMMIT; 