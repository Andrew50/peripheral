-- Wrap everything in an explicit transaction
BEGIN;

CREATE TABLE screener (
    -- FK to your `securities` (or similar) table
    security_id            BIGINT NOT NULL PRIMARY KEY,

    /* -------- price -------- */
    open                   NUMERIC(18,6),
    high                   NUMERIC(18,6),
    low                    NUMERIC(18,6),
    close                  NUMERIC(18,6), --price
    wk52_low               NUMERIC(18,6),
    wk52_high              NUMERIC(18,6),

    /* -------- pre‑market price -------- */
    pre_market_open        NUMERIC(18,6),
    pre_market_high        NUMERIC(18,6),
    pre_market_low         NUMERIC(18,6),
    pre_market_close       NUMERIC(18,6),

    /* -------- basics (from `securities`) -------- */
    market_cap             NUMERIC(20,2),
    sector                 TEXT,
    industry               TEXT,

    /* -------- performance -------- */
    pre_market_change          NUMERIC(18,6),
    pre_market_change_pct      NUMERIC(10,5),
    extended_hours_change      NUMERIC(18,6),
    extended_hours_change_pct  NUMERIC(10,5),

    change_1                NUMERIC(18,6),   -- 1‑minute
    change_15               NUMERIC(18,6),   -- 15‑minute
    change_1h               NUMERIC(18,6),
    change_4h               NUMERIC(18,6),
    change_1d               NUMERIC(18,6),
    change_1w               NUMERIC(18,6),
    change_1m               NUMERIC(18,6),   -- 1‑month
    change_3m               NUMERIC(18,6),
    change_6m               NUMERIC(18,6),
    change_ytd_1y           NUMERIC(18,6),   -- YTD / 1‑year
    change_5y               NUMERIC(18,6),
    change_10y              NUMERIC(18,6),
    change_all_time         NUMERIC(18,6),

    change_from_open        NUMERIC(18,6),
    change_from_open_pct    NUMERIC(10,5),
    price_over_52wk_high    NUMERIC(10,5),
    price_over_52wk_low     NUMERIC(10,5),

    /* -------- indicators -------- */
    rsi                     NUMERIC(10,2),
    dma_200                 NUMERIC(18,6),
    dma_50                  NUMERIC(18,6),
    price_over_50dma        NUMERIC(10,5),
    price_over_200dma       NUMERIC(10,5),

    /* -------- beta vs. SPY -------- */
    beta_1y_vs_spy          NUMERIC(10,5),
    beta_1m_vs_spy          NUMERIC(10,5),

    /* -------- volume -------- */
    volume                      BIGINT,
    avg_volume_1m               BIGINT,
    dollar_volume               NUMERIC(20,2),
    avg_dollar_volume_1m        NUMERIC(20,2),
    pre_market_volume           BIGINT,
    pre_market_dollar_volume    NUMERIC(20,2),
    relative_volume_14          NUMERIC(10,5),
    pre_market_vol_over_14d_vol NUMERIC(10,5),

    /* -------- volatility / ranges -------- */
    range_1m_pct            NUMERIC(10,5),
    range_15m_pct           NUMERIC(10,5),
    range_1h_pct            NUMERIC(10,5),
    day_range_pct           NUMERIC(10,5),
    volatility_1w           NUMERIC(10,5),
    volatility_1m           NUMERIC(10,5),
    pre_market_range_pct    NUMERIC(10,5)
);

COMMIT;
