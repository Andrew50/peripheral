-- Migration: 027_add_chart_queries_table
-- Description: Add chart_queries table to log chart function calls for analytics

BEGIN;

-- Create chart_queries table to track chart data requests
CREATE TABLE IF NOT EXISTS chart_queries (
    id SERIAL PRIMARY KEY,
    securityid INTEGER NOT NULL,
    timeframe VARCHAR(20) NOT NULL,
    timestamp BIGINT NOT NULL,
    direction VARCHAR(10) NOT NULL,
    bars INTEGER NOT NULL,
    extended_hours BOOLEAN NOT NULL DEFAULT FALSE,
    is_replay BOOLEAN NOT NULL DEFAULT FALSE,
    include_sec_filings BOOLEAN NOT NULL DEFAULT FALSE,
    user_id INTEGER REFERENCES users(userId) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_chart_queries_securityid ON chart_queries(securityid);
CREATE INDEX IF NOT EXISTS idx_chart_queries_created_at ON chart_queries(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_chart_queries_user_id ON chart_queries(user_id);
CREATE INDEX IF NOT EXISTS idx_chart_queries_timeframe ON chart_queries(timeframe);
CREATE INDEX IF NOT EXISTS idx_chart_queries_securityid_timeframe ON chart_queries(securityid, timeframe);

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
        27,
        'Add chart_queries table to log chart function calls'
    ) ON CONFLICT (version) DO NOTHING;

COMMIT; 