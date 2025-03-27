-- Update the default user's auth_type to 'guest'
UPDATE users
SET auth_type = 'guest'
WHERE userId = 0
    AND username = 'user'
    AND password = 'pass';
-- Add entry to schema_versions table
INSERT INTO schema_versions (version, description)
VALUES (
        '010_update_default_user_auth_type',
        'Update default user auth_type to guest'
    );