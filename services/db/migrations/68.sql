/* =============================================================
   1.  LIVE SNAPSHOT TABLE  (no hypertable, no compression)
   ============================================================= */
DROP MATERIALIZED VIEW IF EXISTS screener;
CREATE TABLE screener (
    ticker                      TEXT        PRIMARY KEY,      -- one row per ticker
    calc_time                   TIMESTAMPTZ NOT NULL,

    security_id                 BIGINT,

    /* ---- price + basics ---- */
    open                        NUMERIC,
    high                        NUMERIC,
    low                         NUMERIC,
    close                       NUMERIC,
    wk52_low                    NUMERIC,
    wk52_high                   NUMERIC,

    pre_market_open             NUMERIC,
    pre_market_high             NUMERIC,
    pre_market_low              NUMERIC,
    pre_market_close            NUMERIC,

    market_cap                  NUMERIC,
    sector                      TEXT,
    industry                    TEXT,

    pre_market_change           NUMERIC,
    pre_market_change_pct       NUMERIC,
    extended_hours_change       NUMERIC,
    extended_hours_change_pct   NUMERIC,

    change_1_pct                NUMERIC,
    change_15_pct               NUMERIC,
    change_1h_pct               NUMERIC,
    change_4h_pct               NUMERIC,
    change_1d_pct               NUMERIC,
    change_1w_pct               NUMERIC,
    change_1m_pct               NUMERIC,
    change_3m_pct               NUMERIC,
    change_6m_pct               NUMERIC,
    change_ytd_1y_pct           NUMERIC,
    change_5y_pct               NUMERIC,
    change_10y_pct              NUMERIC,
    change_all_time_pct         NUMERIC,

    change_from_open            NUMERIC,
    change_from_open_pct        NUMERIC,
    price_over_52wk_high        NUMERIC,
    price_over_52wk_low         NUMERIC,

    rsi                         NUMERIC,
    dma_200                     NUMERIC,
    dma_50                      NUMERIC,
    price_over_50dma            NUMERIC,
    price_over_200dma           NUMERIC,

    beta_1y_vs_spy              NUMERIC,
    beta_1m_vs_spy              NUMERIC,

    volume                      BIGINT,
    avg_volume_1m               NUMERIC,
    dollar_volume               NUMERIC,
    avg_dollar_volume_1m        NUMERIC,

    pre_market_volume           BIGINT,
    pre_market_dollar_volume    NUMERIC,
    relative_volume_14          NUMERIC,
    pre_market_vol_over_14d_vol NUMERIC,

    range_1m_pct                NUMERIC,
    range_15m_pct               NUMERIC,
    range_1h_pct                NUMERIC,
    day_range_pct               NUMERIC,

    volatility_1w               NUMERIC,
    volatility_1m               NUMERIC,
    pre_market_range_pct        NUMERIC
);

/* helpful covering indexes on source tables (run once) */
CREATE INDEX IF NOT EXISTS ohlcv_1d_ticker_ts_desc_inc
        ON ohlcv_1d (ticker, "timestamp" DESC)
        INCLUDE (open, high, low, close, volume);

CREATE INDEX IF NOT EXISTS ohlcv_1m_ticker_ts_desc_inc
        ON ohlcv_1m (ticker, "timestamp" DESC)
        INCLUDE (open, high, low, close, volume);
