CREATE TABLE IF NOT EXISTS horizontal_lines (
    id SERIAL PRIMARY KEY,
    userId INT REFERENCES users(userId) ON DELETE CASCADE,
    securityId INT,
    price FLOAT NOT NULL,
    UNIQUE (userId, securityId, price)
);
CREATE INDEX IF NOT EXISTS idxUserIdSecurityIdPrice ON horizontal_lines(userId, securityId, price);