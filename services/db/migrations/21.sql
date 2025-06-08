-- Migration: 021_add_python_strategies
-- Description: Add support for Python-based trading strategies with advanced execution capabilities

BEGIN;

-- Create python_strategies table
CREATE TABLE IF NOT EXISTS python_strategies (
    id SERIAL PRIMARY KEY,
    strategy_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    python_code TEXT NOT NULL,
    libraries JSONB DEFAULT '[]'::jsonb, -- Array of required Python libraries
    data_prep_sql TEXT, -- Optional SQL query for data preparation
    execution_mode VARCHAR(20) DEFAULT 'python' CHECK (execution_mode IN ('python', 'hybrid', 'notebook')),
    timeout_seconds INTEGER DEFAULT 300,
    memory_limit_mb INTEGER DEFAULT 512,
    cpu_limit_cores DECIMAL(3,2) DEFAULT 1.0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    version INTEGER DEFAULT 1,
    is_active BOOLEAN DEFAULT true,
    
    -- Foreign key constraints
    FOREIGN KEY (strategy_id) REFERENCES strategies(strategyId) ON DELETE CASCADE,
    UNIQUE(strategy_id) -- One Python strategy per strategy_id
);

-- Create python_executions table to track execution history
CREATE TABLE IF NOT EXISTS python_executions (
    id SERIAL PRIMARY KEY,
    python_strategy_id INTEGER NOT NULL REFERENCES python_strategies(id) ON DELETE CASCADE,
    execution_id UUID DEFAULT gen_random_uuid(),
    user_id INTEGER NOT NULL,
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed', 'timeout', 'cancelled')),
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    execution_time_ms INTEGER,
    memory_used_mb INTEGER,
    cpu_used_percent DECIMAL(5,2),
    input_data JSONB, -- Input parameters/data
    output_data JSONB, -- Execution results
    error_message TEXT,
    logs TEXT,
    worker_node VARCHAR(100), -- K8s node/pod that executed the strategy
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create python_environments table for managing execution environments
CREATE TABLE IF NOT EXISTS python_environments (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    base_image VARCHAR(255) NOT NULL, -- Docker image
    python_version VARCHAR(20) NOT NULL,
    libraries JSONB NOT NULL DEFAULT '[]'::jsonb, -- Pre-installed libraries
    environment_variables JSONB DEFAULT '{}'::jsonb,
    is_default BOOLEAN DEFAULT false,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_python_strategies_user_id ON python_strategies(user_id);
CREATE INDEX IF NOT EXISTS idx_python_strategies_strategy_id ON python_strategies(strategy_id);
CREATE INDEX IF NOT EXISTS idx_python_strategies_active ON python_strategies(is_active) WHERE is_active = true;

CREATE INDEX IF NOT EXISTS idx_python_executions_strategy_id ON python_executions(python_strategy_id);
CREATE INDEX IF NOT EXISTS idx_python_executions_user_id ON python_executions(user_id);
CREATE INDEX IF NOT EXISTS idx_python_executions_status ON python_executions(status);
CREATE INDEX IF NOT EXISTS idx_python_executions_created_at ON python_executions(created_at);

CREATE INDEX IF NOT EXISTS idx_python_environments_active ON python_environments(is_active) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_python_environments_default ON python_environments(is_default) WHERE is_default = true;

-- Insert default Python environment
INSERT INTO python_environments (name, description, base_image, python_version, libraries, is_default, is_active)
VALUES (
    'default-quant',
    'Default quantitative finance environment with common libraries',
    'python:3.11-slim',
    '3.11',
    '["numpy", "pandas", "scipy", "scikit-learn", "matplotlib", "seaborn", "ta-lib", "yfinance", "zipline", "pyfolio", "quantlib", "statsmodels", "arch"]'::jsonb,
    true,
    true
) ON CONFLICT (name) DO NOTHING;

-- Create trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_python_strategies_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION update_python_environments_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_python_strategies_updated_at
    BEFORE UPDATE ON python_strategies
    FOR EACH ROW
    EXECUTE FUNCTION update_python_strategies_updated_at();

CREATE TRIGGER trigger_python_environments_updated_at
    BEFORE UPDATE ON python_environments
    FOR EACH ROW
    EXECUTE FUNCTION update_python_environments_updated_at();

-- Create view for active Python strategies with execution stats
CREATE OR REPLACE VIEW v_python_strategies_with_stats AS
SELECT 
    ps.*,
    s.name as strategy_name,
    COUNT(pe.id) as total_executions,
    COUNT(CASE WHEN pe.status = 'completed' THEN 1 END) as successful_executions,
    COUNT(CASE WHEN pe.status = 'failed' THEN 1 END) as failed_executions,
    AVG(CASE WHEN pe.status = 'completed' THEN pe.execution_time_ms END) as avg_execution_time_ms,
    MAX(pe.completed_at) as last_execution_at
FROM python_strategies ps
LEFT JOIN strategies s ON ps.strategy_id = s.strategyId
LEFT JOIN python_executions pe ON ps.id = pe.python_strategy_id
WHERE ps.is_active = true
GROUP BY ps.id, s.name;

COMMIT;