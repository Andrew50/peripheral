-- Update the default user's auth_type to 'guest'
UPDATE users
SET auth_type = 'guest'
WHERE userId = 0
    AND username = 'user'
    AND password = 'pass';