BEGIN;
ALTER TABLE users
ADD COLUMN auth_type VARCHAR(20) DEFAULT 'password';
-- 'password' for password-only auth, 'google' for Google-only auth, 'both' for users who can use either method
COMMIT;