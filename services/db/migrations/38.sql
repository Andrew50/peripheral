-- Migration to remove guest account functionality
-- Remove guest users from the system
DELETE FROM users WHERE auth_type = 'guest';

-- If there are any constraints or checks on auth_type that reference 'guest', 
-- they would need to be dropped here. For now, we'll just remove the guest data.

-- Note: If auth_type column has CHECK constraints that include 'guest' as a valid value,
-- those constraints should be recreated without 'guest' option.
-- Example (uncomment and modify if such constraints exist):
-- ALTER TABLE users DROP CONSTRAINT IF EXISTS users_auth_type_check;
-- ALTER TABLE users ADD CONSTRAINT users_auth_type_check CHECK (auth_type IN ('email', 'oauth'));