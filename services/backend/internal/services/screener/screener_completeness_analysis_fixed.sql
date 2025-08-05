-- Query to analyze data completeness in the screener table
-- Calculates the percentage of non-empty (non-null) rows for each column
-- Fixed version that avoids GROUP BY issues

WITH total_count AS (
    SELECT COUNT(*) as total_rows
    FROM screener
),
completeness_stats AS (
    SELECT 
        column_name,
        non_null_count,
        ROUND(non_null_count * 100.0 / (SELECT total_rows FROM total_count), 2) as completeness_percentage
    FROM (
        SELECT 'ticker' as column_name, COUNT(CASE WHEN ticker IS NOT NULL AND ticker != '' THEN 1 END) as non_null_count FROM screener
        UNION ALL
        SELECT 'calc_time', COUNT(CASE WHEN calc_time IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'security_id', COUNT(CASE WHEN security_id IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'open', COUNT(CASE WHEN open IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'high', COUNT(CASE WHEN high IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'low', COUNT(CASE WHEN low IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'close', COUNT(CASE WHEN close IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'wk52_low', COUNT(CASE WHEN wk52_low IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'wk52_high', COUNT(CASE WHEN wk52_high IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'pre_market_open', COUNT(CASE WHEN pre_market_open IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'pre_market_high', COUNT(CASE WHEN pre_market_high IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'pre_market_low', COUNT(CASE WHEN pre_market_low IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'pre_market_close', COUNT(CASE WHEN pre_market_close IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'market_cap', COUNT(CASE WHEN market_cap IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'sector', COUNT(CASE WHEN sector IS NOT NULL AND sector != '' THEN 1 END) FROM screener
        UNION ALL
        SELECT 'industry', COUNT(CASE WHEN industry IS NOT NULL AND industry != '' THEN 1 END) FROM screener
        UNION ALL
        SELECT 'pre_market_change', COUNT(CASE WHEN pre_market_change IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'pre_market_change_pct', COUNT(CASE WHEN pre_market_change_pct IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'extended_hours_change', COUNT(CASE WHEN extended_hours_change IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'extended_hours_change_pct', COUNT(CASE WHEN extended_hours_change_pct IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'change_1_pct', COUNT(CASE WHEN change_1_pct IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'change_15_pct', COUNT(CASE WHEN change_15_pct IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'change_1h_pct', COUNT(CASE WHEN change_1h_pct IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'change_4h_pct', COUNT(CASE WHEN change_4h_pct IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'change_1d_pct', COUNT(CASE WHEN change_1d_pct IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'change_1w_pct', COUNT(CASE WHEN change_1w_pct IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'change_1m_pct', COUNT(CASE WHEN change_1m_pct IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'change_3m_pct', COUNT(CASE WHEN change_3m_pct IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'change_6m_pct', COUNT(CASE WHEN change_6m_pct IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'change_ytd_1y_pct', COUNT(CASE WHEN change_ytd_1y_pct IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'change_5y_pct', COUNT(CASE WHEN change_5y_pct IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'change_10y_pct', COUNT(CASE WHEN change_10y_pct IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'change_all_time_pct', COUNT(CASE WHEN change_all_time_pct IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'change_from_open', COUNT(CASE WHEN change_from_open IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'change_from_open_pct', COUNT(CASE WHEN change_from_open_pct IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'price_over_52wk_high', COUNT(CASE WHEN price_over_52wk_high IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'price_over_52wk_low', COUNT(CASE WHEN price_over_52wk_low IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'rsi', COUNT(CASE WHEN rsi IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'dma_200', COUNT(CASE WHEN dma_200 IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'dma_50', COUNT(CASE WHEN dma_50 IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'price_over_50dma', COUNT(CASE WHEN price_over_50dma IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'price_over_200dma', COUNT(CASE WHEN price_over_200dma IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'beta_1y_vs_spy', COUNT(CASE WHEN beta_1y_vs_spy IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'beta_1m_vs_spy', COUNT(CASE WHEN beta_1m_vs_spy IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'volume', COUNT(CASE WHEN volume IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'avg_volume_1m', COUNT(CASE WHEN avg_volume_1m IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'dollar_volume', COUNT(CASE WHEN dollar_volume IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'avg_dollar_volume_1m', COUNT(CASE WHEN avg_dollar_volume_1m IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'pre_market_volume', COUNT(CASE WHEN pre_market_volume IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'pre_market_dollar_volume', COUNT(CASE WHEN pre_market_dollar_volume IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'relative_volume_14', COUNT(CASE WHEN relative_volume_14 IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'pre_market_vol_over_14d_vol', COUNT(CASE WHEN pre_market_vol_over_14d_vol IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'range_1m_pct', COUNT(CASE WHEN range_1m_pct IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'range_15m_pct', COUNT(CASE WHEN range_15m_pct IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'range_1h_pct', COUNT(CASE WHEN range_1h_pct IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'day_range_pct', COUNT(CASE WHEN day_range_pct IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'volatility_1w', COUNT(CASE WHEN volatility_1w IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'volatility_1m', COUNT(CASE WHEN volatility_1m IS NOT NULL THEN 1 END) FROM screener
        UNION ALL
        SELECT 'pre_market_range_pct', COUNT(CASE WHEN pre_market_range_pct IS NOT NULL THEN 1 END) FROM screener
    ) counts
)
SELECT 
    column_name,
    non_null_count,
    (SELECT total_rows FROM total_count) as total_rows,
    completeness_percentage || '%' as completeness_percentage
FROM completeness_stats
ORDER BY completeness_percentage DESC, column_name;

-- Summary statistics
SELECT 
    COUNT(*) as total_columns,
    ROUND(AVG(completeness_percentage), 2) || '%' as avg_completeness_pct,
    ROUND(MIN(completeness_percentage), 2) || '%' as min_completeness_pct,
    ROUND(MAX(completeness_percentage), 2) || '%' as max_completeness_pct,
    COUNT(CASE WHEN completeness_percentage = 100 THEN 1 END) as fully_complete_columns,
    COUNT(CASE WHEN completeness_percentage = 0 THEN 1 END) as completely_empty_columns,
    COUNT(CASE WHEN completeness_percentage > 0 AND completeness_percentage < 50 THEN 1 END) as low_completeness_columns,
    COUNT(CASE WHEN completeness_percentage >= 50 AND completeness_percentage < 90 THEN 1 END) as medium_completeness_columns,
    COUNT(CASE WHEN completeness_percentage >= 90 AND completeness_percentage < 100 THEN 1 END) as high_completeness_columns
FROM completeness_stats; 