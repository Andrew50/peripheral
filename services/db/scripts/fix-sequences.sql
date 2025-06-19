-- Fix sequence values to prevent duplicate key errors
-- This script synchronizes all SERIAL sequences with existing data
-- Run this whenever the database starts up or after bulk data imports

-- Securities table
SELECT setval('securities_securityid_seq', COALESCE((SELECT MAX(securityid) FROM securities), 1), true);

-- Users table  
SELECT setval('users_userid_seq', COALESCE((SELECT MAX(userid) FROM users), 1), true);

-- Watchlists table
SELECT setval('watchlists_watchlistid_seq', COALESCE((SELECT MAX(watchlistid) FROM watchlists), 1), true);

-- Watchlist items table
SELECT setval('watchlistitems_watchlistitemid_seq', COALESCE((SELECT MAX(watchlistitemid) FROM watchlistitems), 1), true);

-- Strategies table
SELECT setval('strategies_strategyid_seq', COALESCE((SELECT MAX(strategyid) FROM strategies), 1), true);

-- Studies table
SELECT setval('studies_studyid_seq', COALESCE((SELECT MAX(studyid) FROM studies), 1), true);

-- Alerts table
SELECT setval('alerts_alertid_seq', COALESCE((SELECT MAX(alertid) FROM alerts), 1), true);

-- Horizontal lines table
SELECT setval('horizontal_lines_id_seq', COALESCE((SELECT MAX(id) FROM horizontal_lines), 1), true);

-- Trades table
SELECT setval('trades_tradeid_seq', COALESCE((SELECT MAX(tradeid) FROM trades), 1), true);

-- Trade executions table
SELECT setval('trade_executions_executionid_seq', COALESCE((SELECT MAX(executionid) FROM trade_executions), 1), true);

-- Query logs table
SELECT setval('query_logs_log_id_seq', COALESCE((SELECT MAX(log_id) FROM query_logs), 1), true);

-- Why is it moving table
SELECT setval('why_is_it_moving_id_seq', COALESCE((SELECT MAX(id) FROM why_is_it_moving), 1), true);

-- Print confirmation
SELECT 'All sequences have been synchronized with current data' AS status; 