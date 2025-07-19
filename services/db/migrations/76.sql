-- =========================================================
-- Migration: 076_add_rsi_cagg
-- Description: Add real-time RSI-14 continuous aggregate
-- =========================================================

BEGIN;

-- Insert schema version
INSERT INTO schema_versions (version, description)
VALUES (
    76,
    'Add real-time RSI-14 continuous aggregate'
) ON CONFLICT (version) DO NOTHING;

-- ❶ Enable timescaledb_toolkit if not already available
CREATE EXTENSION IF NOT EXISTS timescaledb_toolkit;

-- ❷ Enable window functions in continuous aggregates
SET timescaledb.enable_cagg_window_functions = true;

-- ❸ RSI-14 continuous aggregate using window functions
-- This requires timescaledb.enable_cagg_window_functions = true
-- SET timescaledb.enable_cagg_window_functions = true;

-- SET timescaledb.enable_cagg_window_functions = true;
-- -- Enable experimental window‑function support in CAggs (Timescale ≥ 2.20)
-- SET timescaledb.enable_cagg_window_functions = true;

-- CREATE MATERIALIZED VIEW IF NOT EXISTS cagg_rsi_14_day
-- WITH (timescaledb.continuous) AS

-- /* ----------------------------------------------------------
--  * Step 1:  Per‑row gain / loss and one‑row‑per‑day bucket tag
--  * ---------------------------------------------------------- */
-- WITH base AS (
--     SELECT
--         "timestamp",
--         time_bucket('1 day', "timestamp", 'America/New_York') AS bucket,
--         ticker,
--         GREATEST(close/1000.0
--                  - lag(close/1000.0)
--                      OVER (PARTITION BY ticker ORDER BY "timestamp"), 0)       AS gain,
--         ABS(LEAST(close/1000.0
--                  - lag(close/1000.0)
--                      OVER (PARTITION BY ticker ORDER BY "timestamp"), 0))      AS loss
--     FROM ohlcv_1m
-- ),

-- /* ----------------------------------------------------------
--  * Step 2:  14‑period smoothed gain & loss using a window
--  * ---------------------------------------------------------- */
-- smooth AS (
--     SELECT
--         "timestamp",
--         bucket,
--         ticker,
--         avg(gain) OVER w14   AS avg_gain_14,
--         avg(loss) OVER w14   AS avg_loss_14
--     FROM base
--     WINDOW w14 AS (
--         PARTITION BY ticker
--         ORDER BY bucket
--         ROWS BETWEEN 13 PRECEDING AND CURRENT ROW
--     )
-- )

-- /* ----------------------------------------------------------
--  * Step 3:  Pick the last smoothed value per (bucket,ticker)
--  *          and turn it into RSI
--  * ---------------------------------------------------------- */
-- SELECT
--     bucket,
--     ticker,
--     100 - 100 / (
--           1 + last(avg_gain_14, "timestamp")
--                 / NULLIF(last(avg_loss_14, "timestamp"), 0)
--     ) AS rsi_14
-- FROM smooth
-- GROUP BY bucket, ticker
-- WITH NO DATA;      -- create now, back‑fill later
-- ❄ Handy index for "latest-row-per-ticker" lookup
-- CREATE INDEX IF NOT EXISTS cagg_rsi_14_day_latest_idx
--     ON cagg_rsi_14_day (ticker, bucket DESC);

-- ❻ Updated refresh_screener function to use the RSI continuous aggregate
CREATE OR REPLACE FUNCTION refresh_screener(p_limit integer)
RETURNS VOID
LANGUAGE plpgsql AS $$
DECLARE
    v_now timestamptz := now();
BEGIN
    -- Bulk refresh in a single statement
    WITH stale_tickers AS (
        SELECT ticker
        FROM screener_stale
        WHERE stale = TRUE
        ORDER BY last_update_time ASC
        LIMIT p_limit
    ),
    latest_daily AS (
        SELECT DISTINCT ON (ticker)
            ticker,
            open,
            high,
            low,
            close,
            volume
        FROM ohlcv_1d
        WHERE ticker IN (SELECT ticker FROM stale_tickers)
        ORDER BY ticker, "timestamp" DESC
    ),
    latest_minute AS (
        SELECT DISTINCT ON (ticker)
            ticker,
            open AS m_open,
            high AS m_high,
            low AS m_low,
            close AS m_close,
            volume AS m_volume
        FROM ohlcv_1m
        WHERE ticker IN (SELECT ticker FROM stale_tickers)
        ORDER BY ticker, "timestamp" DESC
    ),
    security_info AS (
        SELECT DISTINCT ON (st.ticker)
            st.ticker,
            s.securityid,
            s.market_cap,
            s.sector,
            s.industry
        FROM stale_tickers st
        LEFT JOIN securities s ON s.ticker = st.ticker AND s.maxDate IS NULL
        ORDER BY st.ticker, s.securityid DESC
    ),
    cagg_data AS (
        SELECT
            st.ticker,
            c1440.pre_market_open,
            c1440.pre_market_high,
            c1440.pre_market_low,
            c1440.pre_market_close,
            c1440.pre_market_volume,
            c1440.pre_market_dollar_volume,
            c15.range_15m_pct,
            c60.range_1h_pct,
            c4h.close AS price_4h_ago,
            c7d.volatility_1w,
            c30.volatility_1m,
            c50.dma_50,
            c200.dma_200,
            c14d.avg_volume_14d,
            c14m.avg_volume_1m_14,
            0 AS rsi_14  -- Default RSI value (RSI continuous aggregate commented out)
        FROM stale_tickers st
        LEFT JOIN LATERAL (
            SELECT *
            FROM cagg_1440_minute
            WHERE ticker = st.ticker
            ORDER BY bucket DESC
            LIMIT 1
        ) c1440 ON TRUE
        LEFT JOIN LATERAL (
            SELECT *
            FROM cagg_15_minute
            WHERE ticker = st.ticker
            ORDER BY bucket DESC
            LIMIT 1
        ) c15 ON TRUE
        LEFT JOIN LATERAL (
            SELECT *
            FROM cagg_60_minute
            WHERE ticker = st.ticker
            ORDER BY bucket DESC
            LIMIT 1
        ) c60 ON TRUE
        LEFT JOIN LATERAL (
            SELECT *
            FROM cagg_4_hour
            WHERE ticker = st.ticker
            ORDER BY bucket DESC
            LIMIT 1
        ) c4h ON TRUE
        LEFT JOIN LATERAL (
            SELECT *
            FROM cagg_7_day
            WHERE ticker = st.ticker
            ORDER BY bucket DESC
            LIMIT 1
        ) c7d ON TRUE
        LEFT JOIN LATERAL (
            SELECT *
            FROM cagg_30_day
            WHERE ticker = st.ticker
            ORDER BY bucket DESC
            LIMIT 1
        ) c30 ON TRUE
        LEFT JOIN LATERAL (
            SELECT *
            FROM cagg_50_day
            WHERE ticker = st.ticker
            ORDER BY bucket DESC
            LIMIT 1
        ) c50 ON TRUE
        LEFT JOIN LATERAL (
            SELECT *
            FROM cagg_200_day
            WHERE ticker = st.ticker
            ORDER BY bucket DESC
            LIMIT 1
        ) c200 ON TRUE
        LEFT JOIN LATERAL (
            SELECT *
            FROM cagg_14_day
            WHERE ticker = st.ticker
            ORDER BY bucket DESC
            LIMIT 1
        ) c14d ON TRUE
        LEFT JOIN LATERAL (
            SELECT *
            FROM cagg_14_minute
            WHERE ticker = st.ticker
            ORDER BY bucket DESC
            LIMIT 1
        ) c14m ON TRUE
        -- LEFT JOIN LATERAL (
        --     SELECT rsi_14
        --     FROM cagg_rsi_14_day
        --     WHERE ticker = st.ticker
        --     ORDER BY bucket DESC
        --     LIMIT 1
        -- ) rsi ON TRUE
    ),
    historical_prices AS (
        SELECT
            st.ticker,
            prev_close.close AS price_prev_close,
            d1.close AS price_1d,
            w1.close AS price_1w,
            m1.close AS price_1m,
            m3.close AS price_3m,
            m6.close AS price_6m,
            y1.close AS price_1y,
            y5.close AS price_5y,
            y10.close AS price_10y,
            ytd.open AS price_ytd,
            all_time.open AS price_all,
            extremes.min_low AS price_52w_low,
            extremes.max_high AS price_52w_high
        FROM stale_tickers st
        LEFT JOIN LATERAL (
            SELECT close / 1000.0 AS close
            FROM ohlcv_1d
            WHERE ticker = st.ticker
            ORDER BY "timestamp" DESC
            OFFSET 1 LIMIT 1
        ) prev_close ON TRUE
        LEFT JOIN LATERAL (
            SELECT close / 1000.0 AS close
            FROM static_refs_daily
            WHERE ticker = st.ticker
        ) d1 ON d1.close IS NULL
        LEFT JOIN LATERAL (
            SELECT price_1w AS close FROM static_refs_daily WHERE ticker = st.ticker
        ) w1 ON TRUE
        LEFT JOIN LATERAL (
            SELECT price_1m AS close FROM static_refs_daily WHERE ticker = st.ticker
        ) m1 ON TRUE
        LEFT JOIN LATERAL (
            SELECT price_3m AS close FROM static_refs_daily WHERE ticker = st.ticker
        ) m3 ON TRUE
        LEFT JOIN LATERAL (
            SELECT price_6m AS close FROM static_refs_daily WHERE ticker = st.ticker
        ) m6 ON TRUE
        LEFT JOIN LATERAL (
            SELECT price_1y AS close FROM static_refs_daily WHERE ticker = st.ticker
        ) y1 ON TRUE
        LEFT JOIN LATERAL (
            SELECT price_5y AS close FROM static_refs_daily WHERE ticker = st.ticker
        ) y5 ON TRUE
        LEFT JOIN LATERAL (
            SELECT price_10y AS close FROM static_refs_daily WHERE ticker = st.ticker
        ) y10 ON TRUE
        LEFT JOIN LATERAL (
            SELECT price_ytd AS open FROM static_refs_daily WHERE ticker = st.ticker
        ) ytd ON TRUE
        LEFT JOIN LATERAL (
            SELECT price_all AS open FROM static_refs_daily WHERE ticker = st.ticker
        ) all_time ON TRUE
        LEFT JOIN LATERAL (
            SELECT price_52w_low AS min_low, price_52w_high AS max_high
            FROM static_refs_daily
            WHERE ticker = st.ticker
        ) extremes ON TRUE
    ),
    intraday_prices AS (
        SELECT
            st.ticker,
            p1.close AS price_1m,
            p15.close AS price_15m,
            p60.close AS price_1h,
            p240.close AS price_4h
        FROM stale_tickers st
        LEFT JOIN LATERAL (
            SELECT price_1m AS close FROM static_refs_1m WHERE ticker = st.ticker
        ) p1 ON TRUE
        LEFT JOIN LATERAL (
            SELECT price_15m AS close FROM static_refs_1m WHERE ticker = st.ticker
        ) p15 ON TRUE
        LEFT JOIN LATERAL (
            SELECT price_1h AS close FROM static_refs_1m WHERE ticker = st.ticker
        ) p60 ON TRUE
        LEFT JOIN LATERAL (
            SELECT price_4h AS close FROM static_refs_1m WHERE ticker = st.ticker
        ) p240 ON TRUE
    ),
    market_close_data AS (
        SELECT
            st.ticker,
            NULL::numeric AS change_1d_pct,  -- TODO: Calculate 1-day change from historical_prices
            cd.pre_market_open,
            cd.pre_market_high,
            cd.pre_market_low,
            cd.pre_market_close,
            CASE WHEN hp.price_prev_close = 0 OR hp.price_prev_close IS NULL THEN NULL ELSE (cd.pre_market_close - hp.price_prev_close) END AS pre_market_change,
            CASE WHEN hp.price_prev_close = 0 OR hp.price_prev_close IS NULL THEN NULL ELSE (cd.pre_market_close - hp.price_prev_close) / hp.price_prev_close * 100 END AS pre_market_change_pct,
            CASE WHEN cd.pre_market_close = 0 OR cd.pre_market_close IS NULL THEN NULL ELSE (ld.close - cd.pre_market_close) END AS extended_hours_change,
            CASE WHEN cd.pre_market_close = 0 OR cd.pre_market_close IS NULL THEN NULL ELSE (ld.close - cd.pre_market_close) / cd.pre_market_close * 100 END AS extended_hours_change_pct
        FROM stale_tickers st
        JOIN latest_daily ld ON ld.ticker = st.ticker
        JOIN cagg_data cd ON cd.ticker = st.ticker
        JOIN historical_prices hp ON hp.ticker = st.ticker
    ),
    avg_volumes AS (
        SELECT
            st.ticker,
            cd.avg_volume_1m_14 AS avg_volume_1m,
            CASE WHEN ld.close IS NULL OR cd.avg_volume_1m_14 IS NULL THEN NULL ELSE ld.close * cd.avg_volume_1m_14 END AS avg_dollar_volume_1m,
            cd.avg_volume_14d AS avg_daily_volume_14d
        FROM stale_tickers st
        JOIN latest_daily ld ON ld.ticker = st.ticker
        JOIN cagg_data cd ON cd.ticker = st.ticker
    ),
    spy_metrics AS (
        SELECT
            ld_spy.close AS spy_ld_close,
            hp_spy.price_1m AS spy_price_1m,
            hp_spy.price_1y AS spy_price_1y
        FROM (SELECT close FROM latest_daily WHERE ticker = 'SPY' LIMIT 1) ld_spy
        CROSS JOIN (SELECT price_1m, price_1y FROM historical_prices WHERE ticker = 'SPY' LIMIT 1) hp_spy
    ),
    computed_metrics AS (
        SELECT
            si.ticker,
            v_now AS calc_time,
            si.securityid AS security_id,
            ld.open,
            ld.high,
            ld.low,
            ld.close,
            hp.price_52w_low AS wk52_low,
            hp.price_52w_high AS wk52_high,
            mcd.pre_market_open,
            mcd.pre_market_high,
            mcd.pre_market_low,
            mcd.pre_market_close,
            si.market_cap,
            si.sector,
            si.industry,
            mcd.pre_market_change,
            mcd.pre_market_change_pct,
            mcd.extended_hours_change,
            mcd.extended_hours_change_pct,
            CASE WHEN ip.price_1m = 0 OR ip.price_1m IS NULL THEN NULL ELSE (ld.close - ip.price_1m) / ip.price_1m * 100 END AS change_1_pct,
            CASE WHEN ip.price_15m = 0 OR ip.price_15m IS NULL THEN NULL ELSE (ld.close - ip.price_15m) / ip.price_15m * 100 END AS change_15_pct,
            CASE WHEN ip.price_1h = 0 OR ip.price_1h IS NULL THEN NULL ELSE (ld.close - ip.price_1h) / ip.price_1h * 100 END AS change_1h_pct,
            CASE WHEN cd.price_4h_ago = 0 OR cd.price_4h_ago IS NULL THEN NULL ELSE (ld.close - cd.price_4h_ago) / cd.price_4h_ago * 100 END AS change_4h_pct,
            mcd.change_1d_pct,
            CASE WHEN hp.price_1w = 0 OR hp.price_1w IS NULL THEN NULL ELSE (ld.close - hp.price_1w) / hp.price_1w * 100 END AS change_1w_pct,
            CASE WHEN hp.price_1m = 0 OR hp.price_1m IS NULL THEN NULL ELSE (ld.close - hp.price_1m) / hp.price_1m * 100 END AS change_1m_pct,
            CASE WHEN hp.price_3m = 0 OR hp.price_3m IS NULL THEN NULL ELSE (ld.close - hp.price_3m) / hp.price_3m * 100 END AS change_3m_pct,
            CASE WHEN hp.price_6m = 0 OR hp.price_6m IS NULL THEN NULL ELSE (ld.close - hp.price_6m) / hp.price_6m * 100 END AS change_6m_pct,
            CASE WHEN hp.price_1y = 0 OR hp.price_1y IS NULL THEN NULL ELSE (ld.close - hp.price_1y) / hp.price_1y * 100 END AS change_ytd_1y_pct,
            CASE WHEN hp.price_5y = 0 OR hp.price_5y IS NULL THEN NULL ELSE (ld.close - hp.price_5y) / hp.price_5y * 100 END AS change_5y_pct,
            CASE WHEN hp.price_10y = 0 OR hp.price_10y IS NULL THEN NULL ELSE (ld.close - hp.price_10y) / hp.price_10y * 100 END AS change_10y_pct,
            CASE WHEN hp.price_all = 0 OR hp.price_all IS NULL THEN NULL ELSE (ld.close - hp.price_all) / hp.price_all * 100 END AS change_all_time_pct,
            (ld.close - ld.open) AS change_from_open,
            CASE WHEN ld.open = 0 OR ld.open IS NULL THEN NULL ELSE ((ld.close - ld.open) / ld.open) * 100 END AS change_from_open_pct,
            CASE WHEN hp.price_52w_high = 0 OR hp.price_52w_high IS NULL THEN NULL ELSE ld.close / hp.price_52w_high * 100 END AS price_over_52wk_high,
            CASE WHEN hp.price_52w_low = 0 OR hp.price_52w_low IS NULL THEN NULL ELSE ld.close / hp.price_52w_low * 100 END AS price_over_52wk_low,
            cd.rsi_14 AS rsi,  -- Use RSI from continuous aggregate
            cd.dma_200,
            cd.dma_50,
            CASE WHEN cd.dma_50 = 0 OR cd.dma_50 IS NULL THEN NULL ELSE ld.close / cd.dma_50 * 100 END AS price_over_50dma,
            CASE WHEN cd.dma_200 = 0 OR cd.dma_200 IS NULL THEN NULL ELSE ld.close / cd.dma_200 * 100 END AS price_over_200dma,
            CASE WHEN spy.spy_price_1y = 0 OR spy.spy_price_1y IS NULL OR hp.price_1y = 0 OR hp.price_1y IS NULL THEN NULL ELSE ((ld.close - hp.price_1y) / hp.price_1y) / ((spy.spy_ld_close - spy.spy_price_1y) / spy.spy_price_1y) * 100 END AS beta_1y_vs_spy,
            CASE WHEN spy.spy_price_1m = 0 OR spy.spy_price_1m IS NULL OR hp.price_1m = 0 OR hp.price_1m IS NULL THEN NULL ELSE ((ld.close - hp.price_1m) / hp.price_1m) / ((spy.spy_ld_close - spy.spy_price_1m) / spy.spy_price_1m) * 100 END AS beta_1m_vs_spy,
            ld.volume,
            av.avg_volume_1m,
            CASE WHEN ld.close IS NULL OR ld.volume IS NULL THEN NULL ELSE ld.close * ld.volume END AS dollar_volume,
            av.avg_dollar_volume_1m,
            cd.pre_market_volume,
            cd.pre_market_dollar_volume,
            CASE WHEN cd.avg_volume_1m_14 = 0 OR cd.avg_volume_1m_14 IS NULL THEN NULL ELSE lm.m_volume / cd.avg_volume_1m_14 END AS relative_volume_14,
            CASE WHEN av.avg_daily_volume_14d = 0 OR av.avg_daily_volume_14d IS NULL THEN NULL ELSE cd.pre_market_volume / av.avg_daily_volume_14d END AS pre_market_vol_over_14d_vol,
            CASE WHEN lm.m_low = 0 OR lm.m_low IS NULL THEN NULL ELSE (lm.m_high - lm.m_low) / lm.m_low * 100 END AS range_1m_pct,
            cd.range_15m_pct,
            cd.range_1h_pct,
            CASE WHEN ld.low = 0 OR ld.low IS NULL THEN NULL ELSE (ld.high - ld.low) / ld.low * 100 END AS day_range_pct,
            cd.volatility_1w,
            cd.volatility_1m,
            CASE WHEN cd.pre_market_low = 0 OR cd.pre_market_low IS NULL THEN NULL ELSE (cd.pre_market_high - cd.pre_market_low) / cd.pre_market_low * 100 END AS pre_market_range_pct
        FROM security_info si
        JOIN latest_daily ld ON ld.ticker = si.ticker
        JOIN latest_minute lm ON lm.ticker = si.ticker
        JOIN cagg_data cd ON cd.ticker = si.ticker
        JOIN historical_prices hp ON hp.ticker = si.ticker
        JOIN intraday_prices ip ON ip.ticker = si.ticker
        JOIN market_close_data mcd ON mcd.ticker = si.ticker
        JOIN avg_volumes av ON av.ticker = si.ticker
        CROSS JOIN spy_metrics spy
        WHERE ld.close IS NOT NULL AND lm.m_close IS NOT NULL  -- Skip if no data
    )
    INSERT INTO screener (
        ticker, calc_time, security_id, open, high, low, close, wk52_low, wk52_high,
        pre_market_open, pre_market_high, pre_market_low, pre_market_close,
        market_cap, sector, industry,
        pre_market_change, pre_market_change_pct, extended_hours_change, extended_hours_change_pct,
        change_1_pct, change_15_pct, change_1h_pct, change_4h_pct,
        change_1d_pct, change_1w_pct, change_1m_pct, change_3m_pct, change_6m_pct,
        change_ytd_1y_pct, change_5y_pct, change_10y_pct, change_all_time_pct,
        change_from_open, change_from_open_pct, price_over_52wk_high, price_over_52wk_low,
        rsi, dma_200, dma_50, price_over_50dma, price_over_200dma,
        beta_1y_vs_spy, beta_1m_vs_spy,
        volume, avg_volume_1m, dollar_volume, avg_dollar_volume_1m,
        pre_market_volume, pre_market_dollar_volume, relative_volume_14, pre_market_vol_over_14d_vol,
        range_1m_pct, range_15m_pct, range_1h_pct, day_range_pct,
        volatility_1w, volatility_1m, pre_market_range_pct
    )
    SELECT *
    FROM computed_metrics
    ON CONFLICT (ticker) DO UPDATE SET
        calc_time = EXCLUDED.calc_time,
        security_id = EXCLUDED.security_id,
        open = EXCLUDED.open,
        high = EXCLUDED.high,
        low = EXCLUDED.low,
        close = EXCLUDED.close,
        wk52_low = EXCLUDED.wk52_low,
        wk52_high = EXCLUDED.wk52_high,
        pre_market_open = EXCLUDED.pre_market_open,
        pre_market_high = EXCLUDED.pre_market_high,
        pre_market_low = EXCLUDED.pre_market_low,
        pre_market_close = EXCLUDED.pre_market_close,
        market_cap = EXCLUDED.market_cap,
        sector = EXCLUDED.sector,
        industry = EXCLUDED.industry,
        pre_market_change = EXCLUDED.pre_market_change,
        pre_market_change_pct = EXCLUDED.pre_market_change_pct,
        extended_hours_change = EXCLUDED.extended_hours_change,
        extended_hours_change_pct = EXCLUDED.extended_hours_change_pct,
        change_1_pct = EXCLUDED.change_1_pct,
        change_15_pct = EXCLUDED.change_15_pct,
        change_1h_pct = EXCLUDED.change_1h_pct,
        change_4h_pct = EXCLUDED.change_4h_pct,
        change_1d_pct = EXCLUDED.change_1d_pct,
        change_1w_pct = EXCLUDED.change_1w_pct,
        change_1m_pct = EXCLUDED.change_1m_pct,
        change_3m_pct = EXCLUDED.change_3m_pct,
        change_6m_pct = EXCLUDED.change_6m_pct,
        change_ytd_1y_pct = EXCLUDED.change_ytd_1y_pct,
        change_5y_pct = EXCLUDED.change_5y_pct,
        change_10y_pct = EXCLUDED.change_10y_pct,
        change_all_time_pct = EXCLUDED.change_all_time_pct,
        change_from_open = EXCLUDED.change_from_open,
        change_from_open_pct = EXCLUDED.change_from_open_pct,
        price_over_52wk_high = EXCLUDED.price_over_52wk_high,
        price_over_52wk_low = EXCLUDED.price_over_52wk_low,
        rsi = EXCLUDED.rsi,
        dma_200 = EXCLUDED.dma_200,
        dma_50 = EXCLUDED.dma_50,
        price_over_50dma = EXCLUDED.price_over_50dma,
        price_over_200dma = EXCLUDED.price_over_200dma,
        beta_1y_vs_spy = EXCLUDED.beta_1y_vs_spy,
        beta_1m_vs_spy = EXCLUDED.beta_1m_vs_spy,
        volume = EXCLUDED.volume,
        avg_volume_1m = EXCLUDED.avg_volume_1m,
        dollar_volume = EXCLUDED.dollar_volume,
        avg_dollar_volume_1m = EXCLUDED.avg_dollar_volume_1m,
        pre_market_volume = EXCLUDED.pre_market_volume,
        pre_market_dollar_volume = EXCLUDED.pre_market_dollar_volume,
        relative_volume_14 = EXCLUDED.relative_volume_14,
        pre_market_vol_over_14d_vol = EXCLUDED.pre_market_vol_over_14d_vol,
        range_1m_pct = EXCLUDED.range_1m_pct,
        range_15m_pct = EXCLUDED.range_15m_pct,
        range_1h_pct = EXCLUDED.range_1h_pct,
        day_range_pct = EXCLUDED.day_range_pct,
        volatility_1w = EXCLUDED.volatility_1w,
        volatility_1m = EXCLUDED.volatility_1m,
        pre_market_range_pct = EXCLUDED.pre_market_range_pct;

    -- Mark as fresh
    UPDATE screener_stale ss
        SET stale = FALSE,
            last_update_time = v_now
        WHERE ss.ticker IN (
            SELECT ticker
            FROM screener_stale
            WHERE stale = TRUE
            ORDER BY last_update_time ASC
            LIMIT p_limit
        );
END;
$$;

-- ❼ Optional bootstrap for recent data  
-- Uncomment if you want to seed historical data immediately
-- SELECT refresh_continuous_aggregate('cagg_rsi_14_day',
--                                     now() - INTERVAL '3 weeks',
--                                     NULL);

COMMIT; 