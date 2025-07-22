-- Migration 77: Standardize user_id column names to userId
-- Fix inconsistent column naming across tables

-- ===================================================================
-- 1. Fix query_logs table
-- ===================================================================

-- Drop existing index
DROP INDEX IF EXISTS idx_query_logs_user_id;

-- Rename column from user_id to userId (if it exists)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name = 'query_logs' AND column_name = 'user_id') THEN
        ALTER TABLE query_logs RENAME COLUMN user_id TO userId;
    END IF;
END $$;

-- Recreate index with new column name
CREATE INDEX IF NOT EXISTS idx_query_logs_userId ON query_logs(userId);

-- ===================================================================
-- 2. Fix conversations table 
-- ===================================================================

-- Drop existing indexes
DROP INDEX IF EXISTS idx_conversations_user_id;

-- Rename column from user_id to userId (if it exists)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name = 'conversations' AND column_name = 'user_id') THEN
        ALTER TABLE conversations RENAME COLUMN user_id TO userId;
    END IF;
END $$;

-- Fix the foreign key constraint (drop old, create new with correct reference)
ALTER TABLE conversations DROP CONSTRAINT IF EXISTS conversations_user_id_fkey;
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints 
                   WHERE table_name = 'conversations' AND constraint_name = 'conversations_userId_fkey') THEN
        ALTER TABLE conversations ADD CONSTRAINT conversations_userId_fkey 
            FOREIGN KEY (userId) REFERENCES users(userId) ON DELETE CASCADE;
    END IF;
END $$;

-- Recreate index with new column name
CREATE INDEX IF NOT EXISTS idx_conversations_userId ON conversations(userId);

-- ===================================================================
-- 3. Fix chart_queries table
-- ===================================================================

-- Drop existing index
DROP INDEX IF EXISTS idx_chart_queries_user_id;

-- Rename column from user_id to userId (if it exists)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name = 'chart_queries' AND column_name = 'user_id') THEN
        ALTER TABLE chart_queries RENAME COLUMN user_id TO userId;
    END IF;
END $$;

-- Recreate index with new column name
CREATE INDEX IF NOT EXISTS idx_chart_queries_userId ON chart_queries(userId);

-- ===================================================================
-- 4. Fix usage_logs table (if it exists)
-- ===================================================================

-- Drop existing indexes
DROP INDEX IF EXISTS idx_usage_logs_user_id;
DROP INDEX IF EXISTS idx_usage_logs_user_type_date;

-- Rename column from user_id to userId (if table and column exist)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables 
               WHERE table_name = 'usage_logs') AND
       EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name = 'usage_logs' AND column_name = 'user_id') THEN
        ALTER TABLE usage_logs RENAME COLUMN user_id TO userId;
    END IF;
END $$;

-- Recreate indexes with new column name
CREATE INDEX IF NOT EXISTS idx_usage_logs_userId ON usage_logs(userId);
CREATE INDEX IF NOT EXISTS idx_usage_logs_user_type_date ON usage_logs(userId, usage_type, created_at DESC);

-- ===================================================================
-- Update schema version
-- ===================================================================
INSERT INTO schema_versions (version, description)
VALUES (77, 'Standardize user_id column names to userId across all tables')
ON CONFLICT (version) DO NOTHING; 