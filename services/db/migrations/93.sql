-- Create table to store short interest and short volume aggregates
CREATE TABLE IF NOT EXISTS short_data (
  ticker TEXT NOT NULL,
  data_date DATE NOT NULL,

  -- short interest
  short_interest BIGINT,
  avg_daily_volume BIGINT,
  days_to_cover NUMERIC(18,4),

  -- short volume aggregates
  short_volume BIGINT,
  short_volume_ratio NUMERIC(18,4),
  total_volume BIGINT,
  non_exempt_volume BIGINT,
  exempt_volume BIGINT,

  ingested_at TIMESTAMPTZ NOT NULL DEFAULT now(),

  PRIMARY KEY (ticker, data_date)
);

-- Helpful indexes
CREATE INDEX IF NOT EXISTS short_data_data_date_idx ON short_data (data_date);
CREATE INDEX IF NOT EXISTS short_data_ticker_date_desc_idx ON short_data (ticker, data_date DESC);


