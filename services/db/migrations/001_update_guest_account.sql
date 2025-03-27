-- Migration script to update the guest account
-- This script will delete any existing 'guest' account and update user ID 0 to be a proper guest account
-- First, add a migration record
INSERT INTO schema_versions (version, description)
VALUES (
        '001_update_guest_account',
        'Updates the guest account to use proper credentials'
    ) ON CONFLICT (version) DO NOTHING;
-- Delete any existing accounts with guest auth_type (except user ID 0)
DELETE FROM users
WHERE auth_type = 'guest'
    AND userId != 0;
-- Update user ID 0 to have the proper guest credentials
UPDATE users
SET username = 'Guest',
    password = 'guest-password',
    email = 'guest@atlantis.local',
    auth_type = 'guest'
WHERE userId = 0;
-- If no user with ID 0 exists, create it
INSERT INTO users (userId, username, password, email, auth_type)
SELECT 0,
    'Guest',
    'guest-password',
    'guest@atlantis.local',
    'guest'
WHERE NOT EXISTS (
        SELECT 1
        FROM users
        WHERE userId = 0
    );
-- Reset the user sequence if needed to account for manually inserted ID 0
SELECT setval(
        'users_userid_seq',
        (
            SELECT COALESCE(MAX(userId), 0)
            FROM users
        ),
        true
    );