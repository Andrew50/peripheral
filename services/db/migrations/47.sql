-- Initialize PostgreSQL extensions for enhanced logging and monitoring
-- This script sets up pg_stat_statements extension

-- Create pg_stat_statements extension for query statistics
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Log the extension setup
DO $$
BEGIN
    RAISE NOTICE 'PostgreSQL extensions initialized successfully';
    RAISE NOTICE 'pg_stat_statements extension created - functions will be available after server restart';
END $$; 