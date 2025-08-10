 -- Migration: 092_create_fundamentals
BEGIN;

DROP TABLE IF EXISTS fundamentals;

CREATE TABLE IF NOT EXISTS fundamentals (
  id BIGSERIAL PRIMARY KEY,

  -- identity/meta
  ticker TEXT NOT NULL,
  cik TEXT,
  sic TEXT,
  company_name TEXT,
  source_filing_file_url TEXT,
  source_filing_url TEXT,

  -- dates and period info
  start_date DATE,
  end_date DATE,
  filing_date DATE,
  timeframe TEXT,
  fiscal_period TEXT,
  fiscal_year INT,

  -- balance_sheet
  assets NUMERIC,
  liabilities NUMERIC,
  current_assets NUMERIC,
  noncurrent_liabilities NUMERIC,
  liabilities_and_equity NUMERIC,
  other_current_liabilities NUMERIC,
  equity_attributable_to_noncontrolling_interest NUMERIC,
  accounts_payable NUMERIC,
  other_noncurrent_assets NUMERIC,
  inventory NUMERIC,
  equity_attributable_to_parent NUMERIC,
  equity NUMERIC,
  current_liabilities NUMERIC,
  noncurrent_assets NUMERIC,
  intangible_assets NUMERIC,
  other_current_assets NUMERIC,

  -- comprehensive_income
  other_comprehensive_income_loss NUMERIC,
  comprehensive_income_loss NUMERIC,
  comprehensive_income_loss_attributable_to_noncontrolling_interest NUMERIC,
  comprehensive_income_loss_attributable_to_parent NUMERIC,
  other_comprehensive_income_loss_attributable_to_parent NUMERIC,

  -- income_statement
  cost_of_revenue NUMERIC,
  revenues NUMERIC,
  diluted_average_shares NUMERIC,
  basic_average_shares NUMERIC,
  income_loss_from_continuing_operations_before_tax NUMERIC,
  net_income_loss_available_to_common_stockholders_basic NUMERIC,
  income_loss_from_continuing_operations_after_tax NUMERIC,
  income_tax_expense_benefit NUMERIC,
  basic_earnings_per_share NUMERIC,
  operating_expenses NUMERIC,
  operating_income_loss NUMERIC,
  costs_and_expenses NUMERIC,
  nonoperating_income_loss NUMERIC,
  preferred_stock_dividends_and_other_adjustments NUMERIC,
  net_income_loss_attributable_to_parent NUMERIC,
  benefits_costs_expenses NUMERIC,
  net_income_loss NUMERIC,
  selling_general_and_administrative_expenses NUMERIC,
  participating_securities_distributed_and_undistributed_earnings_loss_basic NUMERIC,
  income_tax_expense_benefit_deferred NUMERIC,
  research_and_development NUMERIC,
  income_loss_before_equity_method_investments NUMERIC,
  diluted_earnings_per_share NUMERIC,
  net_income_loss_attributable_to_noncontrolling_interest NUMERIC,
  gross_profit NUMERIC,
  interest_expense_operating NUMERIC,

  -- cash_flow_statement
  net_cash_flow_from_operating_activities NUMERIC,
  net_cash_flow_continuing NUMERIC,
  net_cash_flow_from_operating_activities_continuing NUMERIC,
  net_cash_flow_from_financing_activities NUMERIC,
  net_cash_flow NUMERIC,
  net_cash_flow_from_investing_activities NUMERIC,
  net_cash_flow_from_financing_activities_continuing NUMERIC,
  net_cash_flow_from_investing_activities_continuing NUMERIC,
  exchange_gains_losses NUMERIC,

  ingested_at TIMESTAMPTZ DEFAULT now(),

  -- idempotency key
  UNIQUE (ticker, end_date, timeframe, fiscal_period, fiscal_year)
);

CREATE INDEX IF NOT EXISTS idx_fundamentals_ticker_enddate ON fundamentals (ticker, end_date DESC);
CREATE INDEX IF NOT EXISTS idx_fundamentals_filingdate ON fundamentals (filing_date);

INSERT INTO schema_versions (version, description)
VALUES (92, 'Create fundamentals table')
ON CONFLICT (version) DO NOTHING;

COMMIT;


