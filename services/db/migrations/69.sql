-- Migration: 069_add_python_agent_execs_table
-- Description: Add python_agent_execs table for tracking Python agent execution history

BEGIN;

-- Create python_agent_execs table
CREATE TABLE IF NOT EXISTS python_agent_execs (
    id SERIAL PRIMARY KEY,
    userid INTEGER NOT NULL REFERENCES users(userid) ON DELETE CASCADE,
    prompt TEXT NOT NULL,
    python_code TEXT,
    execution_id VARCHAR(255) NOT NULL,
    result JSONB,
    prints TEXT DEFAULT '',
    plots JSONB DEFAULT '[]'::jsonb,
    response_images JSONB DEFAULT '[]'::jsonb,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_python_agent_execs_userid ON python_agent_execs(userid);
CREATE INDEX IF NOT EXISTS idx_python_agent_execs_execution_id ON python_agent_execs(execution_id);
CREATE INDEX IF NOT EXISTS idx_python_agent_execs_created_at ON python_agent_execs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_python_agent_execs_userid_created_at ON python_agent_execs(userid, created_at DESC);

-- GIN indexes for JSONB columns that might be searched
CREATE INDEX IF NOT EXISTS idx_python_agent_execs_plots ON python_agent_execs USING GIN(plots);
CREATE INDEX IF NOT EXISTS idx_python_agent_execs_response_images ON python_agent_execs USING GIN(response_images);

-- Record schema version
INSERT INTO schema_versions (version, description)
VALUES (
    69,
    'Add python_agent_execs table for tracking Python agent execution history'
) ON CONFLICT (version) DO NOTHING;

COMMIT; 