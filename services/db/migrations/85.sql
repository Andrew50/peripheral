BEGIN;


ALTER TABLE users
ADD COLUMN verified BOOLEAN NOT NULL DEFAULT FALSE,
ADD COLUMN otp_code VARCHAR(10),
ADD COLUMN otp_expires_at TIMESTAMP;

-- make sure to not deactivate current accounts
UPDATE users SET verified = TRUE; 

COMMIT;
