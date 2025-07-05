-- Migration: 044_remove_consume_user_credits_function
-- Description: Remove the consume_user_credits function as credit consumption is now handled in Go code

BEGIN;

-- Drop the consume_user_credits function
DROP FUNCTION IF EXISTS consume_user_credits(INTEGER, INTEGER);

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    44,
    'Remove consume_user_credits function as credit consumption is now handled in Go code'
) ON CONFLICT (version) DO NOTHING;

COMMIT; 