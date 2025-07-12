/* ===============================================================
   Screener continuous-aggregate definition – 58 calculated fields
   =============================================================== */

-- 1. Make sure the TimescaleDB extension is present.
CREATE EXTENSION IF NOT EXISTS timescaledb;

------------------------------------------------------------------
-- 2. Base table that will hold materialised rows
------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS screener (
    calc_time               TIMESTAMPTZ NOT NULL,          -- bucketed time of calculation
    security_id             BIGINT      NOT NULL,
    ticker                  TEXT        NOT NULL,

    /* ---- price + basics ---- */
    open                    NUMERIC,
    high                    NUMERIC,
    low                     NUMERIC,
    close                   NUMERIC,
    wk52_low                NUMERIC,
    wk52_high               NUMERIC,

    pre_market_open         NUMERIC,
    pre_market_high         NUMERIC,
    pre_market_low          NUMERIC,
    pre_market_close        NUMERIC,

    market_cap              NUMERIC,
    sector                  TEXT,
    industry                TEXT,

    pre_market_change       NUMERIC,
    pre_market_change_pct   NUMERIC,
    extended_hours_change   NUMERIC,
    extended_hours_change_pct NUMERIC,

    change_1_pct            NUMERIC,
    change_15_pct           NUMERIC,
    change_1h_pct           NUMERIC,
    change_4h_pct           NUMERIC,
    change_1d_pct           NUMERIC,
    change_1w_pct           NUMERIC,
    change_1m_pct           NUMERIC,
    change_3m_pct           NUMERIC,
    change_6m_pct           NUMERIC,
    change_ytd_1y_pct       NUMERIC,
    change_5y_pct           NUMERIC,
    change_10y_pct          NUMERIC,
    change_all_time_pct     NUMERIC,

    change_from_open        NUMERIC,
    change_from_open_pct    NUMERIC,
    price_over_52wk_high    NUMERIC,
    price_over_52wk_low     NUMERIC,

    rsi                     NUMERIC,
    dma_200                 NUMERIC,
    dma_50                  NUMERIC,
    price_over_50dma        NUMERIC,
    price_over_200dma       NUMERIC,

    beta_1y_vs_spy          NUMERIC,
    beta_1m_vs_spy          NUMERIC,

    volume                  BIGINT,
    avg_volume_1m           NUMERIC,
    dollar_volume           NUMERIC,
    avg_dollar_volume_1m    NUMERIC,

    pre_market_volume       BIGINT,
    pre_market_dollar_volume NUMERIC,
    relative_volume_14      NUMERIC,
    pre_market_vol_over_14d_vol NUMERIC,

    range_1m_pct            NUMERIC,
    range_15m_pct           NUMERIC,
    range_1h_pct            NUMERIC,
    day_range_pct           NUMERIC,

    volatility_1w           NUMERIC,
    volatility_1m           NUMERIC,
    pre_market_range_pct    NUMERIC,

    PRIMARY KEY (calc_time, security_id)
);

-- Promote to hypertable for automatic partitioning
SELECT create_hypertable('screener', 'calc_time', if_not_exists => TRUE);

-- Helpful index for latest snapshot look-ups
CREATE INDEX IF NOT EXISTS screener_latest_idx
    ON screener (ticker, calc_time DESC);

------------------------------------------------------------------
-- 3. Continuous aggregate view (1-minute buckets)
------------------------------------------------------------------
-- Refresh every minute; backfill 32 days.
CREATE MATERIALIZED VIEW screener_ca
WITH (
    timescaledb.continuous,
    timescaledb.refresh_lag = INTERVAL '2 minutes',
    timescaledb.refresh_interval = INTERVAL '1 minute',
    timescaledb.compress = FALSE
) AS
WITH
    -- === runtime helpers ====================================================
    params AS (
        SELECT
            NOW()                       AS now_utc,
            NOW() AT TIME ZONE 'America/New_York'  AS now_et,
            NOW() AT TIME ZONE 'America/New_York'::DATE
                                      + TIME '04:00' AS pre_start_et,
            NOW() AT TIME ZONE 'America/New_York'::DATE
                                      + TIME '09:30' AS reg_start_et,
            NOW() AT TIME ZONE 'America/New_York'::DATE
                                      + TIME '16:00' AS reg_end_et,
            NOW() AT TIME ZONE 'America/New_York'::DATE
                                      + TIME '20:00' AS ext_end_et
    ),

    -- === latest daily bar ====================================================
    daily_last AS (
        SELECT DISTINCT ON (ticker)
            ticker,
            securityid,
            close  AS d_close,
            high   AS d_high,
            low    AS d_low,
            open   AS d_open,
            volume AS d_volume,
            market_cap,
            sector,
            industry,
            time_bucket('1 day', "timestamp") AS bar_day
        FROM ohlcv_1d
        ORDER BY ticker, "timestamp" DESC
    ),

    -- === 52-week extremes ====================================================
    extremes_52wk AS (
        SELECT ticker,
               MAX(high) AS wk52_high,
               MIN(low)  AS wk52_low
        FROM ohlcv_1d
        WHERE "timestamp" >= (SELECT now_utc - INTERVAL '52 weeks' FROM params)
        GROUP BY ticker
    ),

    -- === pre-market slice (04:00-09:30 ET) ==================================
    pm_slice AS (
        SELECT
            o.ticker,
            MIN(o."timestamp")  KEEP (WHERE o.ticker IS NOT NULL) AS pm_open_ts,
            MAX(o."timestamp")  KEEP (WHERE o.ticker IS NOT NULL) AS pm_close_ts,
            MIN(o.low)  AS pm_low,
            MAX(o.high) AS pm_high,
            SUM(o.volume) AS pm_volume
        FROM ohlcv_1m o, params p
        WHERE o."timestamp" >= p.pre_start_et
          AND o."timestamp" <  p.reg_start_et
        GROUP BY o.ticker
    ),

    pm_values AS (
        SELECT
            s.ticker,
            (SELECT open  FROM ohlcv_1m WHERE ticker = s.ticker AND "timestamp" = pm_open_ts LIMIT 1)  AS pm_open,
            (SELECT close FROM ohlcv_1m WHERE ticker = s.ticker AND "timestamp" = pm_close_ts LIMIT 1) AS pm_close,
            pm_high,
            pm_low,
            pm_volume
        FROM pm_slice s
    ),

    -- === extended-hours change (close vs prev close) ========================
    prev_close AS (
        SELECT DISTINCT ON (ticker)
               ticker,
               close AS prev_close
        FROM (
            SELECT ticker,
                   close,
                   ROW_NUMBER() OVER (PARTITION BY ticker ORDER BY "timestamp" DESC) AS rn
            FROM ohlcv_1d
        ) t
        WHERE rn = 2
        ORDER BY ticker
    ),

    -- === intraday returns windows ===========================================
    intraday_refs AS (
        SELECT
            o.ticker,
            MAX(o.close) FILTER (WHERE o."timestamp" <= (SELECT now_utc - INTERVAL '1 minute' FROM params))  AS price_1m,
            MAX(o.close) FILTER (WHERE o."timestamp" <= (SELECT now_utc - INTERVAL '15 minutes' FROM params)) AS price_15m,
            MAX(o.close) FILTER (WHERE o."timestamp" <= (SELECT now_utc - INTERVAL '1 hour' FROM params))    AS price_1h,
            MAX(o.close) FILTER (WHERE o."timestamp" <= (SELECT now_utc - INTERVAL '4 hours' FROM params))   AS price_4h
        FROM ohlcv_1m o, params p
        WHERE o."timestamp" >= p.now_utc - INTERVAL '4 hours'
        GROUP BY o.ticker
    ),

    -- === daily/historical reference closes ==================================
    daily_refs AS (
        SELECT
            o.ticker,
            MAX(o.close) FILTER (WHERE o."timestamp" <= (SELECT now_utc - INTERVAL '1 day'  FROM params)) AS price_1d,
            MAX(o.close) FILTER (WHERE o."timestamp" <= (SELECT now_utc - INTERVAL '7 days' FROM params)) AS price_1w,
            MAX(o.close) FILTER (WHERE o."timestamp" <= (SELECT now_utc - INTERVAL '1 month' FROM params)) AS price_1mo,
            MAX(o.close) FILTER (WHERE o."timestamp" <= (SELECT now_utc - INTERVAL '3 months' FROM params)) AS price_3mo,
            MAX(o.close) FILTER (WHERE o."timestamp" <= (SELECT now_utc - INTERVAL '6 months' FROM params)) AS price_6mo,
            MAX(o.close) FILTER (WHERE o."timestamp" <= (SELECT now_utc - INTERVAL '1 year'   FROM params)) AS price_1y,
            MAX(o.close) FILTER (WHERE o."timestamp" <= (SELECT now_utc - INTERVAL '5 years'  FROM params)) AS price_5y,
            MAX(o.close) FILTER (WHERE o."timestamp" <= (SELECT now_utc - INTERVAL '10 years' FROM params)) AS price_10y,
            MAX(o.close) FILTER (WHERE DATE_TRUNC('year', o."timestamp" AT TIME ZONE 'America/New_York')
                                  = DATE_TRUNC('year',(SELECT now_et FROM params)))               AS price_ytd,
            MIN(o.close)  AS price_all
        FROM ohlcv_1d o
        GROUP BY o.ticker
    ),

    -- === moving averages (50d & 200d) =======================================
    dma AS (
        SELECT
            ticker,
            AVG(close) FILTER (WHERE rn <= 50)  AS dma_50,
            AVG(close) FILTER (WHERE rn <= 200) AS dma_200
        FROM (
            SELECT ticker,
                   close,
                   ROW_NUMBER() OVER (PARTITION BY ticker ORDER BY "timestamp" DESC) AS rn
            FROM ohlcv_1d
        ) x
        WHERE rn <= 200
        GROUP BY ticker
    ),

    -- === RSI (14-period, daily closes) ======================================
    rsi_calc AS (
        SELECT
            ticker,
            CASE
                WHEN avg_loss = 0 THEN 100
                ELSE 100 - (100 / (1 + avg_gain/avg_loss))
            END AS rsi
        FROM (
            SELECT
                ticker,
                AVG(GREATEST(close - LAG(close) OVER w,0))  AS avg_gain,
                AVG(GREATEST(LAG(close) OVER w - close,0))  AS avg_loss
            FROM ohlcv_1d
            WINDOW w AS (PARTITION BY ticker ORDER BY "timestamp" DESC ROWS BETWEEN 14 PRECEDING AND 1 PRECEDING)
        ) sub
    ),

    -- === volume averages & relative vol =====================================
    volumes AS (
        SELECT
            ticker,
            AVG(volume) FILTER (WHERE rn <= 30) AS avg_volume_30d,
            AVG(volume) FILTER (WHERE rn <= 14) AS avg_volume_14d
        FROM (
            SELECT
                ticker, volume,
                ROW_NUMBER() OVER (PARTITION BY ticker ORDER BY "timestamp" DESC) AS rn
            FROM ohlcv_1d
        ) q
        WHERE rn <= 30
        GROUP BY ticker
    ),

    -- === intraday high/low windows (1m / 15m / 1h) ==========================
    intraday_ranges AS (
        SELECT
            ticker,
            -- latest 1-minute bar
            (MAX(high) FILTER (WHERE rn = 1)) AS high_1m,
            (MIN(low)  FILTER (WHERE rn = 1)) AS low_1m,
            -- trailing 15-minute
            MAX(high) FILTER (WHERE "timestamp" >= (SELECT now_utc - INTERVAL '15 minutes' FROM params)) AS high_15m,
            MIN(low)  FILTER (WHERE "timestamp" >= (SELECT now_utc - INTERVAL '15 minutes' FROM params)) AS low_15m,
            -- trailing 1-hour
            MAX(high) FILTER (WHERE "timestamp" >= (SELECT now_utc - INTERVAL '1 hour' FROM params)) AS high_1h,
            MIN(low)  FILTER (WHERE "timestamp" >= (SELECT now_utc - INTERVAL '1 hour' FROM params)) AS low_1h
        FROM (
            SELECT
                o.*,
                ROW_NUMBER() OVER (PARTITION BY o.ticker ORDER BY o."timestamp" DESC) AS rn
            FROM ohlcv_1m o
            WHERE o."timestamp" >= (SELECT now_utc - INTERVAL '1 hour' FROM params)
        ) x
        GROUP BY ticker
    ),

    -- === daily return volatility ============================================
    volas AS (
        WITH rets AS (
            SELECT
                ticker,
                (close - LAG(close) OVER w) / LAG(close) OVER w AS ret,
                ROW_NUMBER() OVER (PARTITION BY ticker ORDER BY "timestamp" DESC) AS rn
            FROM ohlcv_1d
            WINDOW w AS (PARTITION BY ticker ORDER BY "timestamp")
        )
        SELECT
            ticker,
            STDDEV(ret) FILTER (WHERE rn <= 7)  AS vol_1w,
            STDDEV(ret) FILTER (WHERE rn <= 30) AS vol_1m
        FROM rets
        GROUP BY ticker
    ),

    -- === β calc (cov / var) vs SPY ==========================================
    beta AS (
        WITH spy AS (
            SELECT
                (close - LAG(close) OVER w) / LAG(close) OVER w AS spy_ret,
                "timestamp"
            FROM ohlcv_1d
            WHERE ticker = 'SPY'
            WINDOW w AS (ORDER BY "timestamp")
        ),
        tgt AS (
            SELECT
                ticker,
                (close - LAG(close) OVER w) / LAG(close) OVER w AS tgt_ret,
                "timestamp"
            FROM ohlcv_1d
            WINDOW w AS (PARTITION BY ticker ORDER BY "timestamp")
        ),
        merged AS (
            SELECT
                t.ticker,
                t.tgt_ret,
                s.spy_ret,
                t."timestamp"
            FROM tgt t
            JOIN spy s USING ("timestamp")
            WHERE t.tgt_ret IS NOT NULL AND s.spy_ret IS NOT NULL
        )
        SELECT
            ticker,
            COVAR_POP(tgt_ret, spy_ret) FILTER (WHERE "timestamp" >= (SELECT now_utc - INTERVAL '1 year' FROM params))
                / VAR_POP(spy_ret) FILTER (WHERE "timestamp" >= (SELECT now_utc - INTERVAL '1 year' FROM params))
                AS beta_1y,
            COVAR_POP(tgt_ret, spy_ret) FILTER (WHERE "timestamp" >= (SELECT now_utc - INTERVAL '30 days' FROM params))
                / VAR_POP(spy_ret) FILTER (WHERE "timestamp" >= (SELECT now_utc - INTERVAL '30 days' FROM params))
                AS beta_1m
        FROM merged
        GROUP BY ticker
    )

-- === FINAL SELECT ===========================================================
SELECT
    -- bucket time (1-minute granularity)
    time_bucket('1 minute', (SELECT now_utc FROM params))          AS calc_time,

    d.securityid,
    d.ticker,

    /* ---- price + basics ---- */
    d.d_open                       AS open,
    d.d_high                       AS high,
    d.d_low                        AS low,
    d.d_close                      AS close,
    e.wk52_low,
    e.wk52_high,

    /* ---- pre-market ---- */
    pm.pm_open                     AS pre_market_open,
    pm.pm_high                     AS pre_market_high,
    pm.pm_low                      AS pre_market_low,
    pm.pm_close                    AS pre_market_close,

    d.market_cap,
    d.sector,
    d.industry,

    /* derived pre-market changes */
    (pm.pm_close - pm.pm_open)                         AS pre_market_change,
    CASE WHEN pm.pm_open = 0 THEN NULL
         ELSE (pm.pm_close - pm.pm_open)/pm.pm_open*100
    END                                                AS pre_market_change_pct,

    /* extended hours change vs previous close */
    (d.d_close - pc.prev_close)                        AS extended_hours_change,
    CASE WHEN pc.prev_close = 0 THEN NULL
         ELSE (d.d_close - pc.prev_close)/pc.prev_close*100
    END                                                AS extended_hours_change_pct,

    /* intraday % changes */
    pct(d.d_close, ir.price_1m)    AS change_1_pct,
    pct(d.d_close, ir.price_15m)   AS change_15_pct,
    pct(d.d_close, ir.price_1h)    AS change_1h_pct,
    pct(d.d_close, ir.price_4h)    AS change_4h_pct,

    /* daily / weekly / … % changes */
    pct(d.d_close, dr.price_1d)    AS change_1d_pct,
    pct(d.d_close, dr.price_1w)    AS change_1w_pct,
    pct(d.d_close, dr.price_1mo)   AS change_1m_pct,
    pct(d.d_close, dr.price_3mo)   AS change_3m_pct,
    pct(d.d_close, dr.price_6mo)   AS change_6m_pct,
    pct(d.d_close, dr.price_ytd)   AS change_ytd_1y_pct,
    pct(d.d_close, dr.price_5y)    AS change_5y_pct,
    pct(d.d_close, dr.price_10y)   AS change_10y_pct,
    pct(d.d_close, dr.price_all)   AS change_all_time_pct,

    /* from open */
    d.d_close - d.d_open                           AS change_from_open,
    CASE WHEN d.d_open = 0 THEN NULL
         ELSE (d.d_close - d.d_open)/d.d_open*100
    END                                            AS change_from_open_pct,

    /* price / wk52 */
    CASE WHEN e.wk52_high = 0 THEN NULL ELSE d.d_close/e.wk52_high*100 END AS price_over_52wk_high,
    CASE WHEN e.wk52_low  = 0 THEN NULL ELSE d.d_close/e.wk52_low *100 END AS price_over_52wk_low,

    /* technicals */
    r.rsi,
    ma.dma_200,
    ma.dma_50,
    CASE WHEN ma.dma_50  = 0 THEN NULL ELSE d.d_close/ma.dma_50*100  END    AS price_over_50dma,
    CASE WHEN ma.dma_200 = 0 THEN NULL ELSE d.d_close/ma.dma_200*100 END    AS price_over_200dma,

    b.beta_1y                   AS beta_1y_vs_spy,
    b.beta_1m                   AS beta_1m_vs_spy,

    /* volume & dollar volume */
    d.d_volume                  AS volume,
    v.avg_volume_30d            AS avg_volume_1m,
    d.d_volume * d.d_close      AS dollar_volume,
    v.avg_volume_30d * d.d_close AS avg_dollar_volume_1m,

    /* pre-market volume/dollar/rel-vol */
    pm.pm_volume                AS pre_market_volume,
    pm.pm_volume * pm.pm_close  AS pre_market_dollar_volume,
    CASE WHEN v.avg_volume_14d = 0 THEN NULL ELSE d.d_volume / v.avg_volume_14d END AS relative_volume_14,
    CASE WHEN v.avg_volume_14d = 0 THEN NULL ELSE pm.pm_volume / v.avg_volume_14d END AS pre_market_vol_over_14d_vol,

    /* intra-day ranges */
    rng.range_1m_pct,
    rng.range_15m_pct,
    rng.range_1h_pct,
    CASE WHEN d.d_low = 0 THEN NULL ELSE (d.d_high - d.d_low)/d.d_low*100 END AS day_range_pct,

    /* volatilities */
    vola.vol_1w                 AS volatility_1w,
    vola.vol_1m                 AS volatility_1m,

    /* pre-market range % */
    CASE WHEN pm.pm_low = 0 THEN NULL ELSE (pm.pm_high - pm.pm_low)/pm.pm_low*100 END AS pre_market_range_pct

FROM daily_last d
LEFT JOIN extremes_52wk      e   ON e.ticker = d.ticker
LEFT JOIN pm_values          pm  ON pm.ticker = d.ticker
LEFT JOIN prev_close         pc  ON pc.ticker = d.ticker
LEFT JOIN intraday_refs      ir  ON ir.ticker = d.ticker
LEFT JOIN daily_refs         dr  ON dr.ticker = d.ticker
LEFT JOIN dma                ma  ON ma.ticker = d.ticker
LEFT JOIN rsi_calc           r   ON r.ticker  = d.ticker
LEFT JOIN volumes            v   ON v.ticker  = d.ticker
LEFT JOIN intraday_ranges    rng ON rng.ticker= d.ticker
LEFT JOIN volas              vola ON vola.ticker = d.ticker
LEFT JOIN beta               b   ON b.ticker  = d.ticker;

------------------------------------------------------------------
-- 4. Helper function for %-change to keep the SELECT readable
------------------------------------------------------------------
CREATE OR REPLACE FUNCTION pct(curr NUMERIC, ref NUMERIC)
RETURNS NUMERIC LANGUAGE SQL IMMUTABLE AS $$
    SELECT CASE
             WHEN ref IS NULL OR ref = 0 OR curr IS NULL THEN NULL
             ELSE (curr - ref)/ref*100
           END;
$$;

------------------------------------------------------------------
-- 5. Policy: refresh the most-recent 2-day window every minute - handle by scheduler
------------------------------------------------------------------
/*CALL add_continuous_aggregate_policy(
    'screener_ca',
    start_offset => INTERVAL '2 days',
    end_offset   => INTERVAL '0 minutes',
    schedule_interval => INTERVAL '1 minute'
);*/
