-- Migration to set default auth_type for existing users
UPDATE users SET auth_type = 'guest' WHERE auth_type IS NULL; 