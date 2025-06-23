-- Migration: 028_consolidate_strategy_tables
-- Description: Consolidate python_strategies into main strategies table and remove redundancy

BEGIN;

-- First, migrate data from python_strategies to strategies table
-- Update existing strategies with python code from python_strategies
UPDATE strategies 
SET 
    pythonCode = ps.python_code,
    description = COALESCE(strategies.description, ps.description),
    createdAt = COALESCE(strategies.createdAt, ps.created_at),
    version = COALESCE(strategies.version, ps.version::VARCHAR)
FROM python_strategies ps 
WHERE strategies.strategyId = ps.strategy_id;

-- Insert new strategies that exist only in python_strategies
INSERT INTO strategies (
    userId, 
    name, 
    description, 
    pythonCode, 
    alertActive,
    createdAt,
    version
)
SELECT 
    ps.user_id,
    ps.name,
    ps.description,
    ps.python_code,
    false, -- default alertActive
    ps.created_at,
    ps.version::VARCHAR
FROM python_strategies ps
WHERE NOT EXISTS (
    SELECT 1 FROM strategies s WHERE s.strategyId = ps.strategy_id
);

-- Add columns from python_strategies that might be useful
ALTER TABLE strategies 
ADD COLUMN IF NOT EXISTS libraries JSONB DEFAULT '[]'::jsonb,
ADD COLUMN IF NOT EXISTS data_prep_sql TEXT,
ADD COLUMN IF NOT EXISTS execution_mode VARCHAR(20) DEFAULT 'python' CHECK (
    execution_mode IN ('python', 'hybrid', 'notebook')
),
ADD COLUMN IF NOT EXISTS timeout_seconds INTEGER DEFAULT 300,
ADD COLUMN IF NOT EXISTS memory_limit_mb INTEGER DEFAULT 512,
ADD COLUMN IF NOT EXISTS cpu_limit_cores DECIMAL(3, 2) DEFAULT 1.0,
ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT true;

-- Create trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_strategies_updated_at() 
RETURNS TRIGGER AS $$ 
BEGIN 
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_strategies_updated_at 
    BEFORE UPDATE ON strategies 
    FOR EACH ROW EXECUTE FUNCTION update_strategies_updated_at();

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_strategies_is_active ON strategies(is_active) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_strategies_execution_mode ON strategies(execution_mode);
CREATE INDEX IF NOT EXISTS idx_strategies_updated_at ON strategies(updated_at DESC);

-- Drop the redundant python_strategies table and related objects
DROP TRIGGER IF EXISTS trigger_python_strategies_updated_at ON python_strategies;
DROP TRIGGER IF EXISTS trigger_python_environments_updated_at ON python_environments;
DROP FUNCTION IF EXISTS update_python_strategies_updated_at();
DROP FUNCTION IF EXISTS update_python_environments_updated_at();
DROP VIEW IF EXISTS v_python_strategies_with_stats;
DROP TABLE IF EXISTS python_executions CASCADE;
DROP TABLE IF EXISTS python_strategies CASCADE;
DROP TABLE IF EXISTS python_environments CASCADE;

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    28,
    'Consolidate python_strategies into main strategies table'
) ON CONFLICT (version) DO NOTHING;

COMMIT; 