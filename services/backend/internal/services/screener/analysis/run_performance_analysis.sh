#!/bin/bash

# Performance Analysis Script for refresh_static_refs functions
# Executes each query with clear delimiters for debugging

OUTPUT_FILE="performance_analysis_results.txt"
SQL_FILE="analyze_refresh_static_refs_performance.sql"

# Clear the output file
> "$OUTPUT_FILE"

echo "Starting performance analysis at $(date)" | tee -a "$OUTPUT_FILE"
echo "=============================================================" | tee -a "$OUTPUT_FILE"

# Function to execute a query with delimiter
execute_query() {
    local query_description="$1"
    local query="$2"
    
    echo "" | tee -a "$OUTPUT_FILE"
    echo "EXECUTING: $query_description" | tee -a "$OUTPUT_FILE"
    echo "=============================================================" | tee -a "$OUTPUT_FILE"
    echo "QUERY:" | tee -a "$OUTPUT_FILE"
    echo "$query" | tee -a "$OUTPUT_FILE"
    echo "-------------------------------------------------------------" | tee -a "$OUTPUT_FILE"
    echo "RESULTS:" | tee -a "$OUTPUT_FILE"
    
    # Execute the query and capture both stdout and stderr
    echo "$query" | docker exec -i dev-db-1 psql -U postgres -d postgres 2>&1 | tee -a "$OUTPUT_FILE"
    
    echo "=============================================================" | tee -a "$OUTPUT_FILE"
}

# Extract and execute queries from the SQL file
echo "Extracting queries from $SQL_FILE..." | tee -a "$OUTPUT_FILE"

# 1. Active securities population for 1m function - TRUNCATE
execute_query "1. TRUNCATE static_refs_1m_actives_stage" "EXPLAIN (ANALYZE, BUFFERS, VERBOSE) TRUNCATE static_refs_1m_actives_stage;"

# 2. Active securities population for 1m function - INSERT
execute_query "2. INSERT INTO static_refs_1m_actives_stage" "EXPLAIN (ANALYZE, BUFFERS, VERBOSE) 
INSERT INTO static_refs_1m_actives_stage (ticker)
SELECT DISTINCT s.ticker
FROM securities s
WHERE s.active = TRUE;"

# 3. Stage table truncation for 1m prices
execute_query "3. TRUNCATE static_refs_1m_prices_stage" "EXPLAIN (ANALYZE, BUFFERS, VERBOSE) TRUNCATE static_refs_1m_prices_stage;"

# 4. Complex bulk population query for 1m prices (main bottleneck) - First part only due to complexity
execute_query "4. Complex 1m prices population (simplified test)" "EXPLAIN (ANALYZE, BUFFERS, VERBOSE) 
SELECT 
    s.ticker,
    p1.close
FROM static_refs_1m_actives_stage s
LEFT JOIN LATERAL (
    SELECT close / NULLIF(1000.0, 0) AS close
    FROM ohlcv_1m
    WHERE ticker = s.ticker
      AND \"timestamp\" >= (now() - INTERVAL '5 days')
    ORDER BY ABS(EXTRACT(EPOCH FROM (\"timestamp\" - (now() - INTERVAL '1 minute')))) ASC
    LIMIT 1
) p1 ON TRUE
LIMIT 10;"

# 5. Test the full function
execute_query "5. Full refresh_static_refs_1m() function" "EXPLAIN (ANALYZE, BUFFERS, VERBOSE) SELECT refresh_static_refs_1m();"

# 6. Daily function - TRUNCATE
execute_query "6. TRUNCATE static_refs_daily_actives_stage" "EXPLAIN (ANALYZE, BUFFERS, VERBOSE) TRUNCATE static_refs_daily_actives_stage;"

# 7. Daily function - INSERT actives
execute_query "7. INSERT INTO static_refs_daily_actives_stage" "EXPLAIN (ANALYZE, BUFFERS, VERBOSE) 
INSERT INTO static_refs_daily_actives_stage (ticker)
SELECT DISTINCT s.ticker
FROM securities s
WHERE s.active = TRUE;"

# 8. Test daily function
execute_query "8. Full refresh_static_refs() function" "EXPLAIN (ANALYZE, BUFFERS, VERBOSE) SELECT refresh_static_refs();"

# 9. Index analysis
execute_query "9. Index analysis on critical tables" "SELECT 
    schemaname,
    tablename,
    indexname,
    indexdef
FROM pg_indexes 
WHERE tablename IN ('ohlcv_1m', 'ohlcv_1d', 'static_refs_1m', 'static_refs_daily', 'securities')
ORDER BY tablename, indexname;"

# 10. Table statistics
execute_query "10. Table statistics for data volume analysis" "SELECT 
    schemaname,
    tablename,
    n_tup_ins as inserts,
    n_tup_upd as updates,
    n_tup_del as deletes,
    n_live_tup as live_rows,
    n_dead_tup as dead_rows,
    last_vacuum,
    last_autovacuum,
    last_analyze,
    last_autoanalyze
FROM pg_stat_user_tables 
WHERE tablename IN ('ohlcv_1m', 'ohlcv_1d', 'static_refs_1m', 'static_refs_daily', 'securities',
                    'static_refs_1m_actives_stage', 'static_refs_1m_prices_stage', 
                    'static_refs_daily_actives_stage', 'static_refs_daily_prices_stage')
ORDER BY tablename;"

echo "" | tee -a "$OUTPUT_FILE"
echo "Performance analysis completed at $(date)" | tee -a "$OUTPUT_FILE"
echo "Results saved to: $OUTPUT_FILE" | tee -a "$OUTPUT_FILE" 