-- Migration 073: Add splash_screen_logs table for website splash analytics

BEGIN;

CREATE TABLE IF NOT EXISTS splash_screen_logs (
    id SERIAL PRIMARY KEY,
    ip_address INET NOT NULL,
    user_agent TEXT,
    referrer TEXT,
    path VARCHAR(255) NOT NULL,
    timestamp TIMESTAMPTZ DEFAULT NOW(),
    userId INTEGER REFERENCES users(userId), -- Optional: if user is authenticated
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_splash_screen_logs_timestamp ON splash_screen_logs(timestamp);
CREATE INDEX IF NOT EXISTS idx_splash_screen_logs_path ON splash_screen_logs(path);
CREATE INDEX IF NOT EXISTS idx_splash_screen_logs_ip ON splash_screen_logs(ip_address);

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    73,
    'Add splash_screen_logs table for website splash analytics'
) ON CONFLICT (version) DO NOTHING;

COMMIT;
