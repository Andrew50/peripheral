
CREATE TABLE users (
    user_id SERIAL PRIMARY KEY,
    username VARCHAR(100) UNIQUE NOT NULL,
    password VARCHAR(100) NOT NULL,
    settings JSONB DEFAULT '{}'
);
CREATE INDEX idx_users ON users (username, password);
