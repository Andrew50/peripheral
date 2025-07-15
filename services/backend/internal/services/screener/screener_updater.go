package screener

import (
	"backend/internal/data"
	"context"
	"fmt"
	"log"
	"time"
)

const (
	refreshInterval    = 60 * time.Second // full screener top-off frequency (fallback)
	refreshTimeout     = 60 * time.Second // per-refresh SQL timeout
	extendedCloseHour  = 20               // 8 PM Eastern – hard stop
	maxTickersPerBatch = 0                // max tickers to process per batch (0 = no limit), used for testing
)

var createStaleQueueQuery = `
CREATE TABLE IF NOT EXISTS screener_stale (
    ticker text PRIMARY KEY,
    last_update_started timestamptz DEFAULT '1970-01-01',
    stale boolean DEFAULT TRUE
);

-- Index the stale-queue hot path for efficient lookups
CREATE INDEX IF NOT EXISTS screener_stale_due_idx
  ON screener_stale (stale, last_update_started)
  INCLUDE (ticker);
`

var createPreMarketStatsQuery = `
CREATE MATERIALIZED VIEW IF NOT EXISTS pre_market_stats
WITH (timescaledb.continuous)
AS
SELECT
    ticker,
    -- Bucket by calendar day in Eastern Time, stored as UTC timestamps
    time_bucket('1 day', "timestamp", 'America/New_York') AS trade_day,
    /* prices & volumes */
    first(open,  "timestamp")                    AS pre_market_open,
    last (close, "timestamp")                    AS pre_market_close,
    max  (high)                                  AS pre_market_high,
    min  (low)                                   AS pre_market_low,
    sum  (volume)                                AS pre_market_volume,
    sum  (volume * close)                        AS pre_market_dollar_volume,
    /* derived metrics */
    (max(high) - min(low)) / NULLIF(min(low),0) * 100        AS pre_market_range_pct,
    last(close, "timestamp") - first(open, "timestamp")   AS pre_market_change,
    (last(close, "timestamp") - first(open, "timestamp"))
        / NULLIF(first(open, "timestamp"),0) * 100          AS pre_market_change_pct
FROM ohlcv_1m
WHERE
    ("timestamp" AT TIME ZONE 'America/New_York')::time
        BETWEEN TIME '04:00' AND TIME '09:29:59'
GROUP BY ticker, trade_day
WITH NO DATA;

-- 2️⃣  Index so REFRESH + queries stay fast
CREATE INDEX IF NOT EXISTS pre_market_stats_ticker_day_idx
         ON pre_market_stats (ticker, trade_day);

-- 3️⃣  Keep it fresh automatically (run every 5 min, back-fills 7 days)
SELECT add_continuous_aggregate_policy(
        'pre_market_stats',
        start_offset     => INTERVAL '7 days',
        end_offset       => INTERVAL '0',
        schedule_interval=> INTERVAL '5 minutes');
`

var createOHLCVIndexesQuery = `
-- Helpful covering indexes on partitioned source tables, these are created by the migration 68.sql script and in ohlcv table post load setup, but also defined here to make sure they are active and available for the screener updater
CREATE INDEX IF NOT EXISTS ohlcv_1d_ticker_ts_desc_inc
        ON ohlcv_1d (ticker, "timestamp" DESC)
        INCLUDE (open, high, low, close, volume);

CREATE INDEX IF NOT EXISTS ohlcv_1m_ticker_ts_desc_inc
        ON ohlcv_1m (ticker, "timestamp" DESC)
        INCLUDE (open, high, low, close, volume);
`

var intradayPriceRefsQuery = `
CREATE MATERIALIZED VIEW IF NOT EXISTS intraday_stats
WITH (timescaledb.continuous)
AS
SELECT
    ticker,
    bucket_min                                       AS ts,
    100.0 * (close_1m - LAG(close_1m,  60) OVER w)
          / NULLIF(LAG(close_1m,  60) OVER w, 0)     AS change_1h_pct,
    100.0 * (close_1m - LAG(close_1m, 240) OVER w)
          / NULLIF(LAG(close_1m, 240) OVER w, 0)     AS change_4h_pct,
    100.0 * (high_1m - low_1m) / NULLIF(low_1m, 0)   AS range_1m_pct,
    100.0 * (MAX(high_1m) OVER w15
            - MIN(low_1m)  OVER w15)
          / NULLIF(MIN(low_1m) OVER w15, 0)          AS range_15m_pct,
    100.0 * (MAX(high_1m) OVER w60
            - MIN(low_1m)  OVER w60)
          / NULLIF(MIN(low_1m) OVER w60, 0)          AS range_1h_pct,
    AVG(dollar_volume_1m) OVER w14                   AS avg_dollar_volume_1m_14,
    AVG(volume_1m)        OVER w14                   AS avg_volume_1m_14,
    volume_1m /
      NULLIF(AVG(volume_1m) OVER w14, 0)             AS relative_volume_14,
    CASE
        WHEN (bucket_min AT TIME ZONE 'America/New_York')::time
                 BETWEEN TIME '16:00' AND TIME '20:00'
        THEN close_1m
             - FIRST_VALUE(close_1m) OVER w_rth
    END                                              AS extended_hours_change,
    CASE
        WHEN (bucket_min AT TIME ZONE 'America/New_York')::time
                 BETWEEN TIME '16:00' AND TIME '20:00'
        THEN 100.0 * (close_1m
             - FIRST_VALUE(close_1m) OVER w_rth)
             / NULLIF(FIRST_VALUE(close_1m) OVER w_rth, 0)
    END                                              AS extended_hours_change_pct

FROM ohlcv_1m

WINDOW
    w   AS (PARTITION BY ticker ORDER BY bucket_min),                     -- all rows
    w14 AS (PARTITION BY ticker ORDER BY bucket_min ROWS BETWEEN 13 PRECEDING AND CURRENT ROW),
    w15 AS (PARTITION BY ticker ORDER BY bucket_min ROWS BETWEEN 14 PRECEDING AND CURRENT ROW),
    w60 AS (PARTITION BY ticker ORDER BY bucket_min ROWS BETWEEN 59 PRECEDING AND CURRENT ROW),
    w_rth AS (PARTITION BY ticker
              ORDER BY bucket_min
              RANGE BETWEEN INTERVAL '0' PRECEDING   -- start of current RTH session
                        AND INTERVAL '0' PRECEDING)  -- (uses FIRST_VALUE trick)

WITH NO DATA;

CREATE INDEX IF NOT EXISTS intraday_stats_idx
       ON intraday_stats (ticker, ts);

SELECT add_continuous_aggregate_policy(
        'intraday_stats',
        start_offset     => INTERVAL '7 days',
        end_offset       => INTERVAL '0',
        schedule_interval=> INTERVAL '30 seconds');

`

var createHistoricalPriceRefsQuery = `
CREATE MATERIALIZED VIEW IF NOT EXISTS historical_price_refs
WITH (timescaledb.continuous)
AS
SELECT 
    o.ticker,
    last(o.close, o."timestamp") AS current_close,
    -- Time-based price references
    first(o.close, o."timestamp") FILTER (WHERE o."timestamp" >= now() - INTERVAL '7 days')  AS price_1w,
    first(o.close, o."timestamp") FILTER (WHERE o."timestamp" >= now() - INTERVAL '1 month') AS price_1m,
    first(o.close, o."timestamp") FILTER (WHERE o."timestamp" >= now() - INTERVAL '3 months') AS price_3m,
    first(o.close, o."timestamp") FILTER (WHERE o."timestamp" >= now() - INTERVAL '6 months') AS price_6m,
    first(o.close, o."timestamp") FILTER (WHERE o."timestamp" >= now() - INTERVAL '1 year')  AS price_1y,
    first(o.close, o."timestamp") FILTER (WHERE o."timestamp" >= now() - INTERVAL '5 years') AS price_5y,
    first(o.close, o."timestamp") FILTER (WHERE o."timestamp" >= now() - INTERVAL '10 years') AS price_10y,
    first(o.close, o."timestamp") FILTER (WHERE EXTRACT(YEAR FROM o."timestamp") = EXTRACT(YEAR FROM now())) AS price_ytd,
    min(o.close)  FILTER (WHERE o."timestamp" >= now() - INTERVAL '1 year') AS price_52w_low,
    max(o.close)  FILTER (WHERE o."timestamp" >= now() - INTERVAL '1 year') AS price_52w_high,

    -- Technical indicators
    /*
     * RSI-14 calculation: We take the last 15 daily closes, compute day-over-day
     * deltas, separate gains and losses, average them, then apply the classic
     * RSI formula.  The correlated sub-query runs per-ticker and is safe inside
     * the materialised view.
     */
    (
        SELECT CASE 
                 WHEN avg_loss = 0 THEN 100
                 ELSE 100 - 100 / (1 + avg_gain / avg_loss)
               END
        FROM (
            SELECT 
                AVG(CASE WHEN diff > 0 THEN diff ELSE 0 END) AS avg_gain,
                AVG(CASE WHEN diff < 0 THEN -diff ELSE 0 END) AS avg_loss
            FROM (
                SELECT  o2.close - LAG(o2.close) OVER (ORDER BY o2."timestamp") AS diff
                FROM    ohlcv_1d o2
                WHERE   o2.ticker = o.ticker
                  AND   o2."timestamp" >= now() - INTERVAL '15 days'
                ORDER BY o2."timestamp" DESC
                LIMIT   15
            ) diff_sub
        ) rsi_calc
    ) AS rsi_14,

    -- Moving averages
    AVG(o.close) FILTER (WHERE o."timestamp" >= now() - INTERVAL '50 days')  AS sma_50,
    AVG(o.close) FILTER (WHERE o."timestamp" >= now() - INTERVAL '200 days') AS sma_200,

    -- Volatility (standard deviation of closes)
    STDDEV_SAMP(o.close) FILTER (WHERE o."timestamp" >= now() - INTERVAL '7 days')  AS stddev_7,
    STDDEV_SAMP(o.close) FILTER (WHERE o."timestamp" >= now() - INTERVAL '30 days') AS stddev_30
FROM ohlcv_1d o
GROUP BY o.ticker
WITH NO DATA;

CREATE INDEX IF NOT EXISTS historical_price_refs_ticker_idx 
    ON historical_price_refs (ticker);

-- Add continuous aggregate policy for automatic background refresh
SELECT add_continuous_aggregate_policy(
    'historical_price_refs',
    start_offset       => INTERVAL '2 days',
    end_offset         => INTERVAL '1 hour',
    schedule_interval  => INTERVAL '1 hour');
`

var incrementalRefreshQuery = `
-- Incremental screener refresh for stale tickers only
-- This function processes only the tickers that have changed since last refresh

CREATE OR REPLACE FUNCTION public.refresh_screener_delta(
    p_tickers text[]
) RETURNS void
LANGUAGE plpgsql AS $$
DECLARE
    v_now_utc  timestamptz := now();
    v_trade_day date        := (v_now_utc AT TIME ZONE 'America/New_York')::date;
BEGIN
    ------------------------------------------------------------------
    -- Safety exit
    ------------------------------------------------------------------
    IF array_length(p_tickers, 1) IS NULL THEN
        RETURN;
    END IF;

    ------------------------------------------------------------------
    -- Up‑sert screener rows built from securities + latest snapshots
    ------------------------------------------------------------------
    INSERT INTO screener (
        ticker,                -- PK
        calc_time,
        security_id,
        market_cap,
        sector,
        industry,
        /* ───── pre‑market aggregates ───── */
        pre_market_range_pct,
        pre_market_change,
        pre_market_change_pct,
        pre_market_dollar_volume,
        pre_market_open,
        pre_market_high,
        pre_market_low,
        pre_market_close,
        pre_market_volume,
        /* ───── latest daily bar ───── */
        "open", "high", "low", "close", volume, dollar_volume,
        /* ───── moving‑average context ───── */
        price_over_50dma, price_over_200dma,
        /* ───── horizon % changes ───── */
        change_1d_pct, change_1w_pct, change_1m_pct, change_3m_pct,
        change_6m_pct, change_ytd_1y_pct, change_5y_pct, change_10y_pct,
        change_all_time_pct,
        /* ───── volatility & extremes ───── */
        volatility_1w, volatility_1m,
        price_over_52wk_high, price_over_52wk_low,
        wk52_low, wk52_high,
        /* ───── technicals ───── */
        rsi, dma_200, dma_50,
        /* ───── intra‑day ranges & deltas ───── */
        day_range_pct,
        change_from_open, change_from_open_pct,
        change_1_pct, change_15_pct, change_1h_pct, change_4h_pct,
        avg_dollar_volume_1m,
        range_1m_pct, range_15m_pct, range_1h_pct
    )
    SELECT
        /* ——— securities ——— */
        sec.ticker,
        v_now_utc                                        AS calc_time,
        sec.securityid                                   AS security_id,
        sec.market_cap, sec.sector, sec.industry,

        /* ——— pre‑market ——— */
        pm.pre_market_range_pct,
        pm.pre_market_change,
        pm.pre_market_change_pct,
        pm.pre_market_dollar_volume,
        pm.pre_market_open,
        pm.pre_market_high,
        pm.pre_market_low,
        pm.pre_market_close,
        pm.pre_market_volume,

        /* ——— daily bar ——— */
        d.open, d.high, d.low, d.close,
        d.volume,
        d.close * d.volume                               AS dollar_volume,

        /* ——— MA context ——— */
        NULLIF(d.close,0) / NULLIF(hp.sma_50 ,0) * 100   AS price_over_50dma,
        NULLIF(d.close,0) / NULLIF(hp.sma_200,0) * 100   AS price_over_200dma,

        /* ——— Δ vs historic closes ——— */
        pct(d.close, pc.prev_close),
        pct(d.close, hp.price_1w ),
        pct(d.close, hp.price_1m ),
        pct(d.close, hp.price_3m ),
        pct(d.close, hp.price_6m ),
        pct(d.close, hp.price_ytd),
        pct(d.close, hp.price_5y ),
        pct(d.close, hp.price_10y),
        pct(d.close, hp.price_all),

        /* ——— vols & extremes ——— */
        hp.stddev_7                                      AS volatility_1w,
        hp.stddev_30                                     AS volatility_1m,
        NULLIF(d.close,0) / NULLIF(hp.price_52w_high,0) * 100 AS price_over_52wk_high,
        NULLIF(d.close,0) / NULLIF(hp.price_52w_low ,0) * 100 AS price_over_52wk_low,
        hp.price_52w_low                                 AS wk52_low,
        hp.price_52w_high                                AS wk52_high,

        /* ——— technicals ——— */
        hp.rsi_14                                        AS rsi,
        hp.sma_200                                       AS dma_200,
        hp.sma_50                                        AS dma_50,

        /* ——— intraday context ——— */
        rng.day_range_pct,
        d.close - d.open                                 AS change_from_open,
        pct(d.close, d.open),
        delt.change_1_pct,
        delt.change_15_pct,
        ist.change_1h_pct,
        ist.change_4h_pct,
        ist.avg_dollar_volume_1m_14                      AS avg_dollar_volume_1m,
        ist.range_1m_pct,
        ist.range_15m_pct,
        ist.range_1h_pct
    FROM   securities                     sec

    /* ── latest daily bar (covers open/high/low/close/volume) ── */
    JOIN LATERAL (
        SELECT open, high, low, close, volume
        FROM   ohlcv_1d
        WHERE  ticker = sec.ticker
        ORDER  BY "timestamp" DESC
        LIMIT 1
    ) d ON TRUE

    /* ── previous day close (for 1‑day Δ) ── */
    LEFT JOIN LATERAL (
        SELECT "close" AS prev_close
        FROM   ohlcv_1d
        WHERE  ticker = sec.ticker
        ORDER  BY "timestamp" DESC
        OFFSET 1 LIMIT 1
    ) pc ON TRUE

    /* ── pre‑market snapshot ── */
    LEFT JOIN pre_market_stats            pm
           ON pm.ticker     = sec.ticker
          AND pm.trade_day  = v_trade_day

    /* ── latest intraday snapshot ── */
    LEFT JOIN LATERAL (
        SELECT *
        FROM   intraday_stats
        WHERE  ticker = sec.ticker
        ORDER  BY ts DESC
        LIMIT 1
    ) ist ON TRUE

    /* ── 1‑min & 15‑min Δ (not in intraday_stats) ── */
    LEFT JOIN LATERAL (
        SELECT 
            pct(d.close, p1.close_ago)  AS change_1_pct,
            pct(d.close, p15.close_ago) AS change_15_pct
        FROM   (
            SELECT "close" AS close_ago
            FROM   ohlcv_1m
            WHERE  ticker = sec.ticker
              AND  "timestamp" <= v_now_utc - INTERVAL '1 minute'
            ORDER  BY "timestamp" DESC
            LIMIT 1
        ) p1,
        (
            SELECT "close" AS close_ago
            FROM   ohlcv_1m
            WHERE  ticker = sec.ticker
              AND  "timestamp" <= v_now_utc - INTERVAL '15 minutes'
            ORDER  BY "timestamp" DESC
            LIMIT 1
        ) p15
    ) delt ON TRUE

    /* ── historic refs & technicals ── */
    LEFT JOIN historical_price_refs      hp  ON hp.ticker = sec.ticker

    /* ── intra‑day day‑range % (high‑low vs low) ── */
    LEFT JOIN LATERAL (
        SELECT CASE WHEN d.low = 0 THEN NULL ELSE (d.high - d.low)/d.low*100 END AS day_range_pct
    ) rng ON TRUE

    WHERE  sec.ticker = ANY(p_tickers)
      AND  sec.active = TRUE
    ON CONFLICT (ticker) DO UPDATE SET
        calc_time               = EXCLUDED.calc_time,
        market_cap              = EXCLUDED.market_cap,
        sector                  = EXCLUDED.sector,
        industry                = EXCLUDED.industry,
        pre_market_range_pct    = EXCLUDED.pre_market_range_pct,
        pre_market_change       = EXCLUDED.pre_market_change,
        pre_market_change_pct   = EXCLUDED.pre_market_change_pct,
        pre_market_dollar_volume= EXCLUDED.pre_market_dollar_volume,
        pre_market_open         = EXCLUDED.pre_market_open,
        pre_market_high         = EXCLUDED.pre_market_high,
        pre_market_low          = EXCLUDED.pre_market_low,
        pre_market_close        = EXCLUDED.pre_market_close,
        pre_market_volume       = EXCLUDED.pre_market_volume,
        "open"                 = EXCLUDED."open",
        "high"                 = EXCLUDED."high",
        "low"                  = EXCLUDED."low",
        "close"                = EXCLUDED."close",
        volume                  = EXCLUDED.volume,
        dollar_volume           = EXCLUDED.dollar_volume,
        price_over_50dma        = EXCLUDED.price_over_50dma,
        price_over_200dma       = EXCLUDED.price_over_200dma,
        change_1d_pct           = EXCLUDED.change_1d_pct,
        change_1w_pct           = EXCLUDED.change_1w_pct,
        change_1m_pct           = EXCLUDED.change_1m_pct,
        change_3m_pct           = EXCLUDED.change_3m_pct,
        change_6m_pct           = EXCLUDED.change_6m_pct,
        change_ytd_1y_pct       = EXCLUDED.change_ytd_1y_pct,
        change_5y_pct           = EXCLUDED.change_5y_pct,
        change_10y_pct          = EXCLUDED.change_10y_pct,
        change_all_time_pct     = EXCLUDED.change_all_time_pct,
        volatility_1w           = EXCLUDED.volatility_1w,
        volatility_1m           = EXCLUDED.volatility_1m,
        price_over_52wk_high    = EXCLUDED.price_over_52wk_high,
        price_over_52wk_low     = EXCLUDED.price_over_52wk_low,
        wk52_low                = EXCLUDED.wk52_low,
        wk52_high               = EXCLUDED.wk52_high,
        rsi                     = EXCLUDED.rsi,
        dma_200                 = EXCLUDED.dma_200,
        dma_50                  = EXCLUDED.dma_50,
        day_range_pct           = EXCLUDED.day_range_pct,
        change_from_open        = EXCLUDED.change_from_open,
        change_from_open_pct    = EXCLUDED.change_from_open_pct,
        change_1_pct            = EXCLUDED.change_1_pct,
        change_15_pct           = EXCLUDED.change_15_pct,
        change_1h_pct           = EXCLUDED.change_1h_pct,
        change_4h_pct           = EXCLUDED.change_4h_pct,
        avg_dollar_volume_1m    = EXCLUDED.avg_dollar_volume_1m,
        range_1m_pct            = EXCLUDED.range_1m_pct,
        range_15m_pct           = EXCLUDED.range_15m_pct,
        range_1h_pct            = EXCLUDED.range_1h_pct;
END;
$$;

-----------------------------------------------------------------------
-- Helper: percent‑change utility (NULL‑safe division by zero handling)
-----------------------------------------------------------------------
CREATE OR REPLACE FUNCTION pct(curr numeric, ref numeric)
RETURNS numeric LANGUAGE SQL IMMUTABLE STRICT AS $$
    SELECT CASE WHEN ref = 0 THEN NULL
                ELSE (curr - ref) / ref * 100
           END $$;
`

func StartScreenerUpdaterLoop(conn *data.Conn) error {

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		log.Fatalf("❌ cannot load ET timezone: %v", err)
	}

	err = runScreenerLoopInit(conn)
	if err != nil {
		return fmt.Errorf("failed to setup incremental infrastructure: %v", err)
	}

	ticker := time.NewTicker(refreshInterval)

	for {
		now := time.Now().In(loc)
		if now.Hour() >= extendedCloseHour {
			log.Println("🌙 Post‑market closed — stopping incremental screener updater")
			return nil
		}

		select {
		case <-ticker.C:
			updateStaleScreenerValues(conn)

		}
	}
}

// SQL queries for reuse (avoid re-parsing)
var (
	updateAndRefreshSQLWithLimit = `
        WITH stale_tickers AS (
            SELECT ticker
            FROM   screener_stale
            WHERE  stale = TRUE
              AND  last_update_started <= now() - $1::interval
            LIMIT  $2
        ), updated AS (
            UPDATE screener_stale s
            SET    last_update_started = now(),
                   stale = FALSE
            FROM   stale_tickers st
            WHERE  s.ticker = st.ticker
            RETURNING s.ticker
        )
        SELECT refresh_screener_delta(array(SELECT ticker FROM updated));
    `

	updateAndRefreshSQL = `
        WITH stale_tickers AS (
            SELECT ticker
            FROM   screener_stale
            WHERE  stale = TRUE
              AND  last_update_started <= now() - $1::interval
        ), updated AS (
            UPDATE screener_stale s
            SET    last_update_started = now(),
                   stale = FALSE
            FROM   stale_tickers st
            WHERE  s.ticker = st.ticker
            RETURNING s.ticker
        )
        SELECT refresh_screener_delta(array(SELECT ticker FROM updated));
    `
)

func updateStaleScreenerValues(conn *data.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
	defer cancel()
	intervalStr := fmt.Sprintf("%d seconds", int(refreshInterval.Seconds()))

	// Use pre-defined SQL strings to avoid re-parsing the same SQL every minute
	if maxTickersPerBatch > 0 {
		if _, err := conn.DB.Exec(ctx, updateAndRefreshSQLWithLimit, intervalStr, maxTickersPerBatch); err != nil {
			log.Printf("❌ updateStaleScreenerValues: failed to refresh screener data: %v", err)
		}
	} else {
		if _, err := conn.DB.Exec(ctx, updateAndRefreshSQL, intervalStr); err != nil {
			log.Printf("❌ updateStaleScreenerValues: failed to refresh screener data: %v", err)
		}
	}
}

// setupIncrementalInfrastructure sets up the stale queue, triggers, and functions
func runScreenerLoopInit(conn *data.Conn) error {
	log.Println("🔧 Setting up incremental infrastructure (time-based batching)...")

	log.Println("📊 Creating OHLCV indexes...")
	if _, err := conn.DB.Exec(context.Background(), createOHLCVIndexesQuery); err != nil {
		return fmt.Errorf("failed to create OHLCV indexes: %v", err)
	}

	log.Println("📊 Creating pre-market stats materialized view...")
	if _, err := conn.DB.Exec(context.Background(), createPreMarketStatsQuery); err != nil {
		return fmt.Errorf("failed to create pre-market stats view: %v", err)
	}

	log.Println("📊 Creating intraday stats materialized view...")
	if _, err := conn.DB.Exec(context.Background(), intradayPriceRefsQuery); err != nil {
		return fmt.Errorf("failed to create intraday stats view: %v", err)
	}

	log.Println("📊 Creating historical price refs materialized view...")
	if _, err := conn.DB.Exec(context.Background(), createHistoricalPriceRefsQuery); err != nil {
		return fmt.Errorf("failed to create historical price refs view: %v", err)
	}

	log.Println("📊 Creating stale queue table...")
	if _, err := conn.DB.Exec(context.Background(), createStaleQueueQuery); err != nil {
		return fmt.Errorf("failed to create processing state table: %v", err)
	}

	log.Println("🔄 Creating incremental refresh function (delta)...")
	if _, err := conn.DB.Exec(context.Background(), incrementalRefreshQuery); err != nil {
		return fmt.Errorf("failed to create incremental refresh function: %v", err)
	}

	log.Println("📋 Ensuring continuous aggregate policies exist...")
	if _, err := conn.DB.Exec(context.Background(), continuousAggregatePoliciesQuery); err != nil {
		log.Printf("⚠️  Failed to add continuous aggregate policies (may already exist): %v", err)
	}

	log.Println("✅ Incremental infrastructure (time-based batching) setup complete")
	return nil
}

/*var coveringIndexesQuery = `
-- Comprehensive covering & partial indexes for screener performance
-- These enable index-only scans for LATERAL lookups and filter operations

-- Critical indexes for LATERAL lookups (most recent data)
-- Note: Time-based partial indexes removed due to IMMUTABLE function requirement
-- Basic covering indexes from migration 68.sql are sufficient for current workload
/*
CREATE INDEX IF NOT EXISTS ohlcv_1d_recent_ticker_ts_desc_covering
    ON ohlcv_1d (ticker, "timestamp" DESC)
    INCLUDE (open, high, low, close, volume)
    WHERE "timestamp" >= now() - INTERVAL '7 days';

CREATE INDEX IF NOT EXISTS ohlcv_1m_recent_ticker_ts_desc_covering
    ON ohlcv_1m (ticker, "timestamp" DESC)
    INCLUDE (open, high, low, close, volume)
    WHERE "timestamp" >= now() - INTERVAL '1 day';

-- Indexes for intraday lookups (4-hour window)
CREATE INDEX IF NOT EXISTS ohlcv_1m_intraday_ticker_ts_desc_covering
    ON ohlcv_1m (ticker, "timestamp" DESC)
    INCLUDE (close)
    WHERE "timestamp" >= now() - INTERVAL '4 hours';

-- Indexes for 1-hour range calculations
CREATE INDEX IF NOT EXISTS ohlcv_1m_1h_ticker_ts_desc_covering
    ON ohlcv_1m (ticker, "timestamp" DESC)
    INCLUDE (high, low)
    WHERE "timestamp" >= now() - INTERVAL '1 hour';

-- Indexes for daily historical data (365 days)
CREATE INDEX IF NOT EXISTS ohlcv_1d_365d_ticker_ts_desc_covering
    ON ohlcv_1d (ticker, "timestamp" DESC)
    INCLUDE (close, high, low, volume)
    WHERE "timestamp" >= now() - INTERVAL '365 days';

-- Indexes for 52-week extremes
CREATE INDEX IF NOT EXISTS ohlcv_1d_52w_ticker_high_low_covering
    ON ohlcv_1d (ticker, high DESC, low ASC)
    INCLUDE ("timestamp")
    WHERE "timestamp" >= now() - INTERVAL '52 weeks';

-- Indexes for pre-market continuous aggregate
CREATE INDEX IF NOT EXISTS pm_stats_bucket_ticker_covering
    ON pm_stats (bucket, ticker)
    INCLUDE (pm_open, pm_close, pm_high, pm_low, pm_volume);

-- Indexes for 52-week extremes materialized view
CREATE INDEX IF NOT EXISTS ohlcv_52w_extremes_ticker_covering
    ON ohlcv_52w_extremes (ticker)
    INCLUDE (wk52_high, wk52_low);

-- BRIN indexes for time-series data (append-only optimization)
CREATE INDEX IF NOT EXISTS ohlcv_1d_ts_brin
    ON ohlcv_1d USING BRIN ("timestamp")
    WITH (pages_per_range = 128);

CREATE INDEX IF NOT EXISTS ohlcv_1m_ts_brin
    ON ohlcv_1m USING BRIN ("timestamp")
    WITH (pages_per_range = 128);

-- Specialized indexes for technical indicators
/*
CREATE INDEX IF NOT EXISTS ohlcv_1d_rsi_ticker_ts_covering
    ON ohlcv_1d (ticker, "timestamp" DESC)
    INCLUDE (close)
    WHERE "timestamp" >= now() - INTERVAL '30 days';

-- Indexes for volume calculations
CREATE INDEX IF NOT EXISTS ohlcv_1d_volume_ticker_ts_covering
    ON ohlcv_1d (ticker, "timestamp" DESC)
    INCLUDE (volume)
    WHERE "timestamp" >= now() - INTERVAL '30 days';

-- Note: Additional specialized indexes removed to avoid duplication
-- Basic covering indexes from migration 68.sql provide sufficient coverage

-- Security table index for active tickers
CREATE INDEX IF NOT EXISTS securities_active_ticker_covering
    ON securities (ticker)
    INCLUDE (securityid, market_cap, sector, industry)
    WHERE active = TRUE;

-- Maintenance function (simplified - no partial index maintenance needed)
CREATE OR REPLACE FUNCTION refresh_partial_indexes() RETURNS void AS $$
BEGIN
    -- Note: In a production system, you would recreate these indexes
    -- with updated time predicates periodically

    -- For now, just log that maintenance is needed
    RAISE NOTICE 'Partial index maintenance completed. Consider recreating time-based partial indexes monthly.';
    -- No partial indexes to maintain - basic indexes are sufficient
END;
$$ LANGUAGE plpgsql;
`
*/

var continuousAggregatePoliciesQuery = `
-- Add TimescaleDB policies for automatic continuous aggregate refresh
-- This eliminates the need for manual refresh on every tick

-- Policy for pre_market_stats: refresh every 1 minute during market hours
-- (Note: This view already has its policy defined inline during creation)
SELECT add_continuous_aggregate_policy('pre_market_stats',
    start_offset => INTERVAL '7 days',
    end_offset => INTERVAL '0',
    schedule_interval => INTERVAL '1 minute')
ON CONFLICT DO NOTHING;

-- Policy for intraday_stats: refresh every 30 seconds for real-time data
-- (Note: This view already has its policy defined inline during creation)
SELECT add_continuous_aggregate_policy('intraday_stats',
    start_offset => INTERVAL '7 days',
    end_offset => INTERVAL '0',
    schedule_interval => INTERVAL '30 seconds')
ON CONFLICT DO NOTHING;

-- Policy for historical_price_refs: refresh every 1 hour for historical data
-- (Note: This view already has its policy defined inline during creation)
SELECT add_continuous_aggregate_policy('historical_price_refs',
    start_offset => INTERVAL '2 days',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour')
ON CONFLICT DO NOTHING;
`

// dropAllViewsQuery - UNUSED - For fresh restart when needed
// This query drops all materialized views and aggregates created by the screener updater
// Use with caution - will require full rebuild of all screener infrastructure

var _ = `
-- Drop all continuous aggregate policies first
SELECT remove_continuous_aggregate_policy('pre_market_stats', true);
SELECT remove_continuous_aggregate_policy('intraday_stats', true);
SELECT remove_continuous_aggregate_policy('historical_price_refs', true);

-- Drop all materialized views and continuous aggregates
DROP MATERIALIZED VIEW IF EXISTS pre_market_stats CASCADE;
DROP MATERIALIZED VIEW IF EXISTS intraday_stats CASCADE;
DROP MATERIALIZED VIEW IF EXISTS historical_price_refs CASCADE;

-- Drop supporting tables and functions
DROP TABLE IF EXISTS screener_stale CASCADE;
DROP FUNCTION IF EXISTS public.refresh_screener_delta(text[]) CASCADE;
DROP FUNCTION IF EXISTS pct(numeric, numeric) CASCADE;

-- Drop indexes (will be dropped with CASCADE but listed for clarity)
-- DROP INDEX IF EXISTS pre_market_stats_ticker_day_idx;
-- DROP INDEX IF EXISTS intraday_stats_idx;
-- DROP INDEX IF EXISTS historical_price_refs_ticker_idx;
-- DROP INDEX IF EXISTS ohlcv_1d_ticker_ts_desc_inc;
-- DROP INDEX IF EXISTS ohlcv_1m_ticker_ts_desc_inc;

-- Note: After running this cleanup, you must restart the screener updater
-- to recreate all views, functions, and policies
`
