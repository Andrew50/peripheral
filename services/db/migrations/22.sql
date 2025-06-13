-- Migration: 022_create_query_logs_table
-- Description: Create query_logs table to track user queries and LLM responses
BEGIN;
-- Create query_logs table
CREATE TABLE IF NOT EXISTS query_logs (
    log_id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(userId),
    query_text TEXT NOT NULL,
    context JSONB,
    -- Store the provided context items
    response_type VARCHAR(50),
    -- e.g., 'text', 'mixed_content', 'function_calls', 'error'
    response_summary TEXT,
    -- Store a summary or error message
    llm_thinking_response TEXT,
    -- Raw response from the thinking model
    llm_final_response TEXT,
    -- Raw response from the final response model
    requested_functions JSONB,
    -- Store JSON array of function calls requested by LLM
    executed_functions JSONB,
    -- Store JSON array of function names called, if any
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- Create indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_query_logs_user_id ON query_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_query_logs_timestamp ON query_logs(timestamp);
COMMIT;
-- Migration: 022_add_name_index
-- Description: Add trigram index on securities name column for better search performance
BEGIN;
-- Create trigram index on name column for fuzzy search optimization
CREATE INDEX IF NOT EXISTS trgm_idx_securities_name ON securities USING gin (name gin_trgm_ops);
-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
        22,
        'Add trigram index on securities name column for search optimization'
    );
COMMIT;