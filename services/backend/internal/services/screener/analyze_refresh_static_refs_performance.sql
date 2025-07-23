-- EXPLAIN ANALYZE BUFFERS for all substatements in refresh_static_refs_1m() and refresh_static_refs()
-- This file helps analyze performance bottlenecks in the static reference refresh functions

-- =========================================================
-- ANALYZE refresh_static_refs_1m() SUBSTATEMENTS
-- =========================================================

-- 1. Active securities population for 1m function
EXPLAIN (ANALYZE, BUFFERS, VERBOSE) 
TRUNCATE static_refs_1m_actives_stage;

EXPLAIN (ANALYZE, BUFFERS, VERBOSE) 
INSERT INTO static_refs_1m_actives_stage (ticker)
SELECT DISTINCT s.ticker
FROM securities s
WHERE s.active = TRUE;

-- 2. Stage table truncation for 1m prices
EXPLAIN (ANALYZE, BUFFERS, VERBOSE) 
TRUNCATE static_refs_1m_prices_stage;

-- 3. Complex bulk population query for 1m prices (main bottleneck)
EXPLAIN (ANALYZE, BUFFERS, VERBOSE) 
INSERT INTO static_refs_1m_prices_stage (
    ticker, price_1m, price_15m, price_1h, price_4h,
    range_15m_pct, range_1h_pct, avg_volume_1m_14, avg_dollar_volume_1m_14
)
SELECT 
    s.ticker,
    p1.close,
    p15.close,
    p60.close,
    p240.close,
    ranges.range_15m_pct,
    ranges.range_1h_pct,
    volume_avgs.avg_volume_1m_14,
    volume_avgs.avg_dollar_volume_1m_14
FROM static_refs_1m_actives_stage s
LEFT JOIN LATERAL (
    SELECT close / NULLIF(1000.0, 0) AS close
    FROM ohlcv_1m
    WHERE ticker = s.ticker
      AND "timestamp" >= (now() - INTERVAL '5 days')  -- Broad filter first
    ORDER BY ABS(EXTRACT(EPOCH FROM ("timestamp" - (now() - INTERVAL '1 minute')))) ASC
    LIMIT 1
) p1 ON TRUE
LEFT JOIN LATERAL (
    SELECT close / NULLIF(1000.0, 0) AS close
    FROM ohlcv_1m
    WHERE ticker = s.ticker
      AND "timestamp" >= (now() - INTERVAL '5 days')  -- Broad filter first
    ORDER BY ABS(EXTRACT(EPOCH FROM ("timestamp" - (now() - INTERVAL '15 minutes')))) ASC
    LIMIT 1
) p15 ON TRUE
LEFT JOIN LATERAL (
    SELECT close / NULLIF(1000.0, 0) AS close
    FROM ohlcv_1m
    WHERE ticker = s.ticker
      AND "timestamp" >= (now() - INTERVAL '5 days')  -- Broad filter first
    ORDER BY ABS(EXTRACT(EPOCH FROM ("timestamp" - (now() - INTERVAL '1 hour')))) ASC
    LIMIT 1
) p60 ON TRUE
LEFT JOIN LATERAL (
    SELECT close / NULLIF(1000.0, 0) AS close
    FROM ohlcv_1m
    WHERE ticker = s.ticker
      AND "timestamp" >= (now() - INTERVAL '5 days')  -- Broad filter first
    ORDER BY ABS(EXTRACT(EPOCH FROM ("timestamp" - (now() - INTERVAL '4 hours')))) ASC
    LIMIT 1
) p240 ON TRUE
LEFT JOIN LATERAL (
    -- Range calculations for 15m and 1h
    WITH recent_bars AS (
        SELECT 
            high / 1000.0 AS high,
            low / 1000.0 AS low,
            ROW_NUMBER() OVER (ORDER BY "timestamp" DESC) AS rn
        FROM ohlcv_1m
        WHERE ticker = s.ticker
          AND "timestamp" >= (now() - INTERVAL '2 hours')  -- Broad filter
        ORDER BY "timestamp" DESC
        LIMIT 60  -- Max needed for 1h range
    )
    SELECT
        CASE 
            WHEN COUNT(*) FILTER (WHERE rn <= 15) < 15 THEN NULL
            WHEN MIN(low) FILTER (WHERE rn <= 15) = 0 OR MIN(low) FILTER (WHERE rn <= 15) IS NULL THEN NULL
            ELSE (MAX(high) FILTER (WHERE rn <= 15) / MIN(low) FILTER (WHERE rn <= 15) - 1) * 100
        END AS range_15m_pct,
        CASE 
            WHEN COUNT(*) FILTER (WHERE rn <= 60) < 60 THEN NULL
            WHEN MIN(low) FILTER (WHERE rn <= 60) = 0 OR MIN(low) FILTER (WHERE rn <= 60) IS NULL THEN NULL
            ELSE (MAX(high) FILTER (WHERE rn <= 60) / MIN(low) FILTER (WHERE rn <= 60) - 1) * 100
        END AS range_1h_pct
    FROM recent_bars
) ranges ON TRUE
LEFT JOIN LATERAL (
    -- Volume averages for 14-period
    SELECT
        AVG(volume) AS avg_volume_1m_14,
        AVG(volume * close / 1000.0) AS avg_dollar_volume_1m_14
    FROM (
        SELECT volume, close
        FROM ohlcv_1m
        WHERE ticker = s.ticker
          AND "timestamp" >= (now() - INTERVAL '30 minutes')  -- Broad filter
        ORDER BY "timestamp" DESC
        LIMIT 14
    ) recent_volumes
) volume_avgs ON TRUE;

-- 4. Bulk upsert to final 1m table
EXPLAIN (ANALYZE, BUFFERS, VERBOSE) 
INSERT INTO static_refs_1m (
    ticker, price_1m, price_15m, price_1h, price_4h,
    range_15m_pct, range_1h_pct, avg_volume_1m_14, avg_dollar_volume_1m_14,
    updated_at
)
SELECT 
    ticker, price_1m, price_15m, price_1h, price_4h,
    range_15m_pct, range_1h_pct, avg_volume_1m_14, avg_dollar_volume_1m_14,
    now()
FROM static_refs_1m_prices_stage
ON CONFLICT (ticker) DO UPDATE SET
    price_1m = EXCLUDED.price_1m,
    price_15m = EXCLUDED.price_15m,
    price_1h = EXCLUDED.price_1h,
    price_4h = EXCLUDED.price_4h,
    range_15m_pct = EXCLUDED.range_15m_pct,
    range_1h_pct = EXCLUDED.range_1h_pct,
    avg_volume_1m_14 = EXCLUDED.avg_volume_1m_14,
    avg_dollar_volume_1m_14 = EXCLUDED.avg_dollar_volume_1m_14,
    updated_at = EXCLUDED.updated_at;

-- 5. Final cleanup for 1m function
EXPLAIN (ANALYZE, BUFFERS, VERBOSE) 
TRUNCATE static_refs_1m_actives_stage;

-- =========================================================
-- ANALYZE refresh_static_refs() DAILY SUBSTATEMENTS
-- =========================================================

-- 6. Active securities population for daily function
EXPLAIN (ANALYZE, BUFFERS, VERBOSE) 
TRUNCATE static_refs_daily_actives_stage;

EXPLAIN (ANALYZE, BUFFERS, VERBOSE) 
INSERT INTO static_refs_daily_actives_stage (ticker)
SELECT DISTINCT s.ticker
FROM securities s
WHERE s.active = TRUE;

-- 7. Stage table truncation for daily prices
EXPLAIN (ANALYZE, BUFFERS, VERBOSE) 
TRUNCATE static_refs_daily_prices_stage;

-- 8. Complex bulk population query for daily prices (main bottleneck)
EXPLAIN (ANALYZE, BUFFERS, VERBOSE) 
INSERT INTO static_refs_daily_prices_stage (
    ticker, price_prev_close, price_1d, price_1w, price_1m, price_3m, price_6m, price_1y,
    price_5y, price_10y, price_ytd, price_all, price_52w_low, price_52w_high,
    dma_50, dma_200, volatility_1w_pct, volatility_1m_pct, avg_volume_14d, avg_dollar_volume_14d
)
SELECT
    s.ticker,
    prev_close.close,
    d1.close,
    w1.close,
    m1.close,
    m3.close,
    m6.close,
    y1.close,
    y5.close,
    y10.close,
    ytd.ytd_open,
    all_time.all_open,
    extremes.low_52w,
    extremes.high_52w,
    moving_avgs.dma_50,
    moving_avgs.dma_200,
    volatility.volatility_1w_pct,
    volatility.volatility_1m_pct,
    volume_avgs.avg_volume_14d,
    volume_avgs.avg_dollar_volume_14d
FROM static_refs_daily_actives_stage s
LEFT JOIN LATERAL (
    SELECT close / NULLIF(1000.0, 0) AS close
    FROM ohlcv_1d
    WHERE ticker = s.ticker
    ORDER BY "timestamp" DESC
    OFFSET 1 LIMIT 1
) prev_close ON TRUE
LEFT JOIN LATERAL (
    SELECT close / NULLIF(1000.0, 0) AS close
    FROM ohlcv_1d
    WHERE ticker = s.ticker
      AND "timestamp" >= (now() - INTERVAL '2 years')  -- Broad filter for daily data
    ORDER BY ABS(EXTRACT(EPOCH FROM ("timestamp" - (now() - INTERVAL '1 day')))) ASC
    LIMIT 1
) d1 ON TRUE
LEFT JOIN LATERAL (
    SELECT close / NULLIF(1000.0, 0) AS close
    FROM ohlcv_1d
    WHERE ticker = s.ticker
      AND "timestamp" >= (now() - INTERVAL '2 years')  -- Broad filter for daily data
    ORDER BY ABS(EXTRACT(EPOCH FROM ("timestamp" - (now() - INTERVAL '1 week')))) ASC
    LIMIT 1
) w1 ON TRUE
LEFT JOIN LATERAL (
    SELECT close / NULLIF(1000.0, 0) AS close
    FROM ohlcv_1d
    WHERE ticker = s.ticker
      AND "timestamp" >= (now() - INTERVAL '2 years')  -- Broad filter for daily data
    ORDER BY ABS(EXTRACT(EPOCH FROM ("timestamp" - (now() - INTERVAL '1 month')))) ASC
    LIMIT 1
) m1 ON TRUE
LEFT JOIN LATERAL (
    SELECT close / NULLIF(1000.0, 0) AS close
    FROM ohlcv_1d
    WHERE ticker = s.ticker
      AND "timestamp" >= (now() - INTERVAL '2 years')  -- Broad filter for daily data
    ORDER BY ABS(EXTRACT(EPOCH FROM ("timestamp" - (now() - INTERVAL '3 months')))) ASC
    LIMIT 1
) m3 ON TRUE
LEFT JOIN LATERAL (
    SELECT close / NULLIF(1000.0, 0) AS close
    FROM ohlcv_1d
    WHERE ticker = s.ticker
      AND "timestamp" >= (now() - INTERVAL '2 years')  -- Broad filter for daily data
    ORDER BY ABS(EXTRACT(EPOCH FROM ("timestamp" - (now() - INTERVAL '6 months')))) ASC
    LIMIT 1
) m6 ON TRUE
LEFT JOIN LATERAL (
    SELECT close / NULLIF(1000.0, 0) AS close
    FROM ohlcv_1d
    WHERE ticker = s.ticker
      AND "timestamp" >= (now() - INTERVAL '12 years')  -- Broader filter for longer periods
    ORDER BY ABS(EXTRACT(EPOCH FROM ("timestamp" - (now() - INTERVAL '1 year')))) ASC
    LIMIT 1
) y1 ON TRUE
LEFT JOIN LATERAL (
    SELECT close / NULLIF(1000.0, 0) AS close
    FROM ohlcv_1d
    WHERE ticker = s.ticker
      AND "timestamp" >= (now() - INTERVAL '12 years')  -- Broader filter for longer periods
    ORDER BY ABS(EXTRACT(EPOCH FROM ("timestamp" - (now() - INTERVAL '5 years')))) ASC
    LIMIT 1
) y5 ON TRUE
LEFT JOIN LATERAL (
    SELECT close / NULLIF(1000.0, 0) AS close
    FROM ohlcv_1d
    WHERE ticker = s.ticker
      AND "timestamp" >= (now() - INTERVAL '12 years')  -- Broader filter for longer periods
    ORDER BY ABS(EXTRACT(EPOCH FROM ("timestamp" - (now() - INTERVAL '10 years')))) ASC
    LIMIT 1
) y10 ON TRUE
LEFT JOIN LATERAL (
    SELECT open / NULLIF(1000.0, 0) AS ytd_open
    FROM ohlcv_1d
    WHERE ticker = s.ticker AND date_trunc('year', "timestamp" AT TIME ZONE 'America/New_York') = date_trunc('year', now() AT TIME ZONE 'America/New_York')
    ORDER BY "timestamp" ASC LIMIT 1
) ytd ON TRUE
LEFT JOIN LATERAL (
    SELECT open / NULLIF(1000.0, 0) AS all_open
    FROM ohlcv_1d
    WHERE ticker = s.ticker
    -- No time filter - need the very first candle ever
    ORDER BY "timestamp" ASC LIMIT 1
) all_time ON TRUE
LEFT JOIN LATERAL (
    SELECT MAX(high / NULLIF(1000.0, 0)) AS high_52w, MIN(low / NULLIF(1000.0, 0)) AS low_52w
    FROM ohlcv_1d
    WHERE ticker = s.ticker 
    AND "timestamp" >= now() - INTERVAL '52 weeks'  -- Exactly 52 weeks for highs/lows
) extremes ON TRUE
LEFT JOIN LATERAL (
    -- Moving averages using window function for efficiency
    WITH daily_prices AS (
        SELECT close / 1000.0 AS close
        FROM ohlcv_1d
        WHERE ticker = s.ticker
          AND "timestamp" >= (now() - INTERVAL '1 year')  -- Broad filter
        ORDER BY "timestamp" DESC
        LIMIT 200  -- Max needed for 200-DMA
    )
    SELECT
        AVG(close) FILTER (WHERE rownum <= 50) AS dma_50,
        AVG(close) FILTER (WHERE rownum <= 200) AS dma_200
    FROM (
        SELECT close, ROW_NUMBER() OVER () AS rownum
        FROM daily_prices
    ) numbered
) moving_avgs ON TRUE
LEFT JOIN LATERAL (
    -- Volatility calculations
    WITH recent_closes AS (
        SELECT 
            close / 1000.0 AS close,
            ROW_NUMBER() OVER (ORDER BY "timestamp" DESC) AS rn
        FROM ohlcv_1d
        WHERE ticker = s.ticker
          AND "timestamp" >= (now() - INTERVAL '60 days')  -- Broad filter
        ORDER BY "timestamp" DESC
        LIMIT 30  -- Max needed for 30-day volatility
    )
    SELECT
        CASE 
            WHEN COUNT(*) FILTER (WHERE rn <= 7) < 7 THEN NULL
            WHEN AVG(close) FILTER (WHERE rn <= 7) = 0 THEN NULL
            ELSE (STDDEV_SAMP(close) FILTER (WHERE rn <= 7) / AVG(close) FILTER (WHERE rn <= 7)) * 100
        END AS volatility_1w_pct,
        CASE 
            WHEN COUNT(*) FILTER (WHERE rn <= 30) < 30 THEN NULL
            WHEN AVG(close) FILTER (WHERE rn <= 30) = 0 THEN NULL
            ELSE (STDDEV_SAMP(close) FILTER (WHERE rn <= 30) / AVG(close) FILTER (WHERE rn <= 30)) * 100
        END AS volatility_1m_pct
    FROM recent_closes
) volatility ON TRUE
LEFT JOIN LATERAL (
    -- Volume averages
    SELECT
        AVG(volume) AS avg_volume_14d,
        AVG(volume * close / 1000.0) AS avg_dollar_volume_14d
    FROM (
        SELECT volume, close
        FROM ohlcv_1d
        WHERE ticker = s.ticker
          AND "timestamp" >= (now() - INTERVAL '30 days')  -- Broad filter
        ORDER BY "timestamp" DESC
        LIMIT 14
    ) recent_volumes
) volume_avgs ON TRUE;

-- 9. Bulk upsert to final daily table
EXPLAIN (ANALYZE, BUFFERS, VERBOSE) 
INSERT INTO static_refs_daily (
    ticker, price_prev_close, price_1d, price_1w, price_1m, price_3m, price_6m, price_1y,
    price_5y, price_10y, price_ytd, price_all, price_52w_low, price_52w_high,
    dma_50, dma_200, volatility_1w_pct, volatility_1m_pct, avg_volume_14d, avg_dollar_volume_14d,
    updated_at
)
SELECT 
    ticker, price_prev_close, price_1d, price_1w, price_1m, price_3m, price_6m, price_1y,
    price_5y, price_10y, price_ytd, price_all, price_52w_low, price_52w_high,
    dma_50, dma_200, volatility_1w_pct, volatility_1m_pct, avg_volume_14d, avg_dollar_volume_14d,
    now()
FROM static_refs_daily_prices_stage
ON CONFLICT (ticker) DO UPDATE SET
    price_prev_close = EXCLUDED.price_prev_close,
    price_1d = EXCLUDED.price_1d,
    price_1w = EXCLUDED.price_1w,
    price_1m = EXCLUDED.price_1m,
    price_3m = EXCLUDED.price_3m,
    price_6m = EXCLUDED.price_6m,
    price_1y = EXCLUDED.price_1y,
    price_5y = EXCLUDED.price_5y,
    price_10y = EXCLUDED.price_10y,
    price_ytd = EXCLUDED.price_ytd,
    price_all = EXCLUDED.price_all,
    price_52w_low = EXCLUDED.price_52w_low,
    price_52w_high = EXCLUDED.price_52w_high,
    dma_50 = EXCLUDED.dma_50,
    dma_200 = EXCLUDED.dma_200,
    volatility_1w_pct = EXCLUDED.volatility_1w_pct,
    volatility_1m_pct = EXCLUDED.volatility_1m_pct,
    avg_volume_14d = EXCLUDED.avg_volume_14d,
    avg_dollar_volume_14d = EXCLUDED.avg_dollar_volume_14d,
    updated_at = EXCLUDED.updated_at;

-- 10. Final cleanup for daily function
EXPLAIN (ANALYZE, BUFFERS, VERBOSE) 
TRUNCATE static_refs_daily_actives_stage;

-- =========================================================
-- INDIVIDUAL LATERAL JOIN ANALYSIS (for detailed breakdown)
-- =========================================================

-- Test individual LATERAL joins from refresh_static_refs_1m to identify bottlenecks

-- 1-minute price lookup (most recent)
EXPLAIN (ANALYZE, BUFFERS, VERBOSE)
SELECT 
    s.ticker,
    p1.close
FROM static_refs_1m_actives_stage s
LEFT JOIN LATERAL (
    SELECT close / NULLIF(1000.0, 0) AS close
    FROM ohlcv_1m
    WHERE ticker = s.ticker
      AND "timestamp" >= (now() - INTERVAL '5 days')
    ORDER BY ABS(EXTRACT(EPOCH FROM ("timestamp" - (now() - INTERVAL '1 minute')))) ASC
    LIMIT 1
) p1 ON TRUE
LIMIT 10;

-- 15-minute price lookup
EXPLAIN (ANALYZE, BUFFERS, VERBOSE)
SELECT 
    s.ticker,
    p15.close
FROM static_refs_1m_actives_stage s
LEFT JOIN LATERAL (
    SELECT close / NULLIF(1000.0, 0) AS close
    FROM ohlcv_1m
    WHERE ticker = s.ticker
      AND "timestamp" >= (now() - INTERVAL '5 days')
    ORDER BY ABS(EXTRACT(EPOCH FROM ("timestamp" - (now() - INTERVAL '15 minutes')))) ASC
    LIMIT 1
) p15 ON TRUE
LIMIT 10;

-- Range calculations (complex CTE)
EXPLAIN (ANALYZE, BUFFERS, VERBOSE)
SELECT 
    s.ticker,
    ranges.range_15m_pct,
    ranges.range_1h_pct
FROM static_refs_1m_actives_stage s
LEFT JOIN LATERAL (
    WITH recent_bars AS (
        SELECT 
            high / 1000.0 AS high,
            low / 1000.0 AS low,
            ROW_NUMBER() OVER (ORDER BY "timestamp" DESC) AS rn
        FROM ohlcv_1m
        WHERE ticker = s.ticker
          AND "timestamp" >= (now() - INTERVAL '2 hours')
        ORDER BY "timestamp" DESC
        LIMIT 60
    )
    SELECT
        CASE 
            WHEN COUNT(*) FILTER (WHERE rn <= 15) < 15 THEN NULL
            WHEN MIN(low) FILTER (WHERE rn <= 15) = 0 OR MIN(low) FILTER (WHERE rn <= 15) IS NULL THEN NULL
            ELSE (MAX(high) FILTER (WHERE rn <= 15) / MIN(low) FILTER (WHERE rn <= 15) - 1) * 100
        END AS range_15m_pct,
        CASE 
            WHEN COUNT(*) FILTER (WHERE rn <= 60) < 60 THEN NULL
            WHEN MIN(low) FILTER (WHERE rn <= 60) = 0 OR MIN(low) FILTER (WHERE rn <= 60) IS NULL THEN NULL
            ELSE (MAX(high) FILTER (WHERE rn <= 60) / MIN(low) FILTER (WHERE rn <= 60) - 1) * 100
        END AS range_1h_pct
    FROM recent_bars
) ranges ON TRUE
LIMIT 10;

-- Volume averages
EXPLAIN (ANALYZE, BUFFERS, VERBOSE)
SELECT 
    s.ticker,
    volume_avgs.avg_volume_1m_14,
    volume_avgs.avg_dollar_volume_1m_14
FROM static_refs_1m_actives_stage s
LEFT JOIN LATERAL (
    SELECT
        AVG(volume) AS avg_volume_1m_14,
        AVG(volume * close / 1000.0) AS avg_dollar_volume_1m_14
    FROM (
        SELECT volume, close
        FROM ohlcv_1m
        WHERE ticker = s.ticker
          AND "timestamp" >= (now() - INTERVAL '30 minutes')
        ORDER BY "timestamp" DESC
        LIMIT 14
    ) recent_volumes
) volume_avgs ON TRUE
LIMIT 10;

-- =========================================================
-- INDEX ANALYSIS
-- =========================================================

-- Check existing indexes on critical tables
SELECT 
    schemaname,
    tablename,
    indexname,
    indexdef
FROM pg_indexes 
WHERE tablename IN ('ohlcv_1m', 'ohlcv_1d', 'static_refs_1m', 'static_refs_daily', 'securities')
ORDER BY tablename, indexname;

-- Table statistics for understanding data volume
SELECT 
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
ORDER BY tablename;

-- =========================================================
-- FULL FUNCTION ANALYSIS (for comparison)
-- =========================================================

-- Test the complete functions for end-to-end performance
EXPLAIN (ANALYZE, BUFFERS, VERBOSE) SELECT refresh_static_refs_1m();
EXPLAIN (ANALYZE, BUFFERS, VERBOSE) SELECT refresh_static_refs(); 