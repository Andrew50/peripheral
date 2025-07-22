-- Migration 76: Standardize user_id column names to userId
-- Fix inconsistent column naming across tables

-- ===================================================================
-- 1. Fix query_logs table
-- ===================================================================

-- Drop existing index
DROP INDEX IF EXISTS idx_query_logs_user_id;

-- Rename column from user_id to userId
ALTER TABLE query_logs RENAME COLUMN user_id TO userId;

-- Recreate index with new column name
CREATE INDEX idx_query_logs_userId ON query_logs(userId);

-- ===================================================================
-- 2. Fix conversations table 
-- ===================================================================

-- Drop existing indexes
DROP INDEX IF EXISTS idx_conversations_user_id;

-- Rename column from user_id to userId
ALTER TABLE conversations RENAME COLUMN user_id TO userId;

-- Fix the foreign key constraint (drop old, create new with correct reference)
ALTER TABLE conversations DROP CONSTRAINT IF EXISTS conversations_user_id_fkey;
ALTER TABLE conversations ADD CONSTRAINT conversations_userId_fkey 
    FOREIGN KEY (userId) REFERENCES users(userId) ON DELETE CASCADE;

-- Recreate index with new column name
CREATE INDEX idx_conversations_userId ON conversations(userId);

-- ===================================================================
-- 3. Fix chart_queries table
-- ===================================================================

-- Drop existing index
DROP INDEX IF EXISTS idx_chart_queries_user_id;

-- Rename column from user_id to userId
ALTER TABLE chart_queries RENAME COLUMN user_id TO userId;

-- Recreate index with new column name
CREATE INDEX idx_chart_queries_userId ON chart_queries(userId);

-- ===================================================================
-- 4. Fix usage_logs table (if it exists)
-- ===================================================================

-- Drop existing indexes
DROP INDEX IF EXISTS idx_usage_logs_user_id;
DROP INDEX IF EXISTS idx_usage_logs_user_type_date;

-- Rename column from user_id to userId
ALTER TABLE usage_logs RENAME COLUMN user_id TO userId;

-- Recreate indexes with new column name
CREATE INDEX idx_usage_logs_userId ON usage_logs(userId);
CREATE INDEX idx_usage_logs_user_type_date ON usage_logs(userId, usage_type, created_at DESC);

-- ===================================================================
-- Update schema version
-- ===================================================================
INSERT INTO schema_versions (version, description)
VALUES (76, 'Standardize user_id column names to userId across all tables')
ON CONFLICT (version) DO NOTHING; 