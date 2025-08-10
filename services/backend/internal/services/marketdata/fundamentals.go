package marketdata

import (
	"backend/internal/data"
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/polygon-io/client-go/rest/models"
)

const (
	redisFilingKey = "fundamentals:last_filing_date"
	minISO         = "2003-01-01"
)

// UpdateAllFundamentals loads Polygon VX stock financials (no ticker filter), ascending by filing_date from 2003-01-01, flattens, and upserts into fundamentals.
func UpdateAllFundamentals(conn *data.Conn) error {
	ctx := context.Background()
	startTime := time.Now()

	startISO := minISO
	if dbMax, err := getDBMaxFilingDate(ctx, conn); err == nil && dbMax != "" {
		startISO = dbMax
	}

	// Build params pointer with chaining
	params := models.ListStockFinancialsParams{}.
		WithOrder(models.Asc).
		WithSort(models.FilingDate)
	if dt, err := time.Parse(time.DateOnly, startISO); err == nil {
		params = params.WithFilingDate(models.GTE, models.Date(dt))
	}

	iter := conn.Polygon.VX.ListStockFinancials(ctx, params)

	const batchSize = 200
	var rows []*row
	var lastISO string
	var batchCount int
	tickerCache := make(map[string]string)
	seen := make(map[string]struct{}) // dedupe within each batch by unique key

	log.Printf("🚀 Fundamentals: starting from filing_date >= %s", startISO)

	for iter.Next() {
		item := iter.Item()
		r := flatten(item)

		// Resolve ticker from CIK when not provided by API
		if r.Ticker == "" && r.CIK != "" {
			if t := lookupTickerByCIK(ctx, conn, r.CIK, tickerCache); t != "" {
				r.Ticker = t
			}
		}
		// Skip records without a resolvable ticker to avoid violating unique key
		if r.Ticker == "" {
			continue
		}

		// Derive a simple timeframe from fiscal period when missing
		if r.Timeframe == "" {
			fp := strings.ToUpper(r.FiscalPeriod)
			if strings.HasPrefix(fp, "Q") {
				r.Timeframe = "quarter"
			} else if fp == "FY" || fp == "ANNUAL" {
				r.Timeframe = "annual"
			}
		}

		// Dedupe by the table's unique key for this batch
		key := fmt.Sprintf("%s|%s|%s|%s|%s", r.Ticker, isoOrEmpty(r.EndDate), r.Timeframe, r.FiscalPeriod, intPtrToStr(r.FiscalYear))
		if _, exists := seen[key]; exists {
			// Skip duplicates in the same INSERT statement to prevent SQLSTATE 21000
			goto update_last
		}
		seen[key] = struct{}{}

		rows = append(rows, r)

	update_last:
		if item.FilingDate != "" {
			lastISO = item.FilingDate
		} else if item.EndDate != "" && lastISO == "" { // fallback only when no filing date seen yet
			lastISO = item.EndDate
		}
		if len(rows) >= batchSize {
			if err := upsertRows(ctx, conn, rows); err != nil {
				return fmt.Errorf("upsert failed: %w", err)
			}
			if lastISO != "" {
				_ = conn.Cache.Set(ctx, redisFilingKey, lastISO, 0).Err()
			}
			rows = rows[:0]
			// reset dedupe set per batch
			for k := range seen {
				delete(seen, k)
			}
			batchCount++
			logProgressEstimate("Fundamentals", batchCount, 5, startISO, lastISO, startTime)
		}
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("polygon iterator error: %w", err)
	}

	if len(rows) > 0 {
		if err := upsertRows(ctx, conn, rows); err != nil {
			return fmt.Errorf("final upsert failed: %w", err)
		}
		if lastISO != "" {
			_ = conn.Cache.Set(ctx, redisFilingKey, lastISO, 0).Err()
		}
		batchCount++
		logProgressEstimate("Fundamentals", batchCount, 5, startISO, lastISO, startTime)
	}

	if lastISO == "" {
		lastISO = startISO
	}
	log.Printf("✅ Fundamentals: complete through filing_date %s", lastISO)
	return nil
}

// row represents one flattened financials record aligned to fundamentals schema.
type row struct {
	// meta
	Ticker, CIK, SIC, CompanyName        string
	SourceFilingFileURL, SourceFilingURL string
	StartDate, EndDate, FilingDate       *time.Time
	Timeframe, FiscalPeriod              string
	FiscalYear                           *int

	// balance sheet
	Assets, Liabilities, CurrentAssets, NoncurrentLiabilities, LiabilitiesAndEquity,
	OtherCurrentLiabilities, EquityAttributableToNCI, AccountsPayable, OtherNoncurrentAssets,
	Inventory, EquityAttributableToParent, Equity, CurrentLiabilities, NoncurrentAssets,
	IntangibleAssets, OtherCurrentAssets *string

	// comprehensive income
	OtherComprehensiveIncomeLoss, ComprehensiveIncomeLoss,
	ComprehensiveIncomeLossAttributableToNCI, ComprehensiveIncomeLossAttributableToParent,
	OtherComprehensiveIncomeLossAttributableToParent *string

	// income statement
	CostOfRevenue, Revenues, DilutedAverageShares, BasicAverageShares,
	IncomeLossFromContinuingOperationsBeforeTax, NetIncomeLossAvailableToCommonStockholdersBasic,
	IncomeLossFromContinuingOperationsAfterTax, IncomeTaxExpenseBenefit, BasicEarningsPerShare,
	OperatingExpenses, OperatingIncomeLoss, CostsAndExpenses, NonoperatingIncomeLoss,
	PreferredStockDividendsAndOtherAdjustments, NetIncomeLossAttributableToParent,
	BenefitsCostsExpenses, NetIncomeLoss, SellingGeneralAndAdministrativeExpenses,
	ParticipatingSecuritiesDistributedAndUndistributedEarningsLossBasic,
	IncomeTaxExpenseBenefitDeferred, ResearchAndDevelopment,
	IncomeLossBeforeEquityMethodInvestments, DilutedEarningsPerShare,
	NetIncomeLossAttributableToNCI, GrossProfit, InterestExpenseOperating *string

	// cash flow
	NetCashFlowFromOperatingActivities, NetCashFlowContinuing,
	NetCashFlowFromOperatingActivitiesContinuing, NetCashFlowFromFinancingActivities,
	NetCashFlow, NetCashFlowFromInvestingActivities,
	NetCashFlowFromFinancingActivitiesContinuing,
	NetCashFlowFromInvestingActivitiesContinuing,
	ExchangeGainsLosses *string
}

func parseISODate(s string) *time.Time {
	if s == "" {
		return nil
	}
	dt, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return nil
	}
	t := time.Date(dt.Year(), dt.Month(), dt.Day(), 0, 0, 0, 0, time.UTC)
	return &t
}

func getFrom(fin map[string]models.Financial, group, key string) *string {
	if fin == nil {
		return nil
	}
	grp, ok := fin[group]
	if !ok {
		return nil
	}
	v, ok := grp[key]
	if !ok {
		return nil
	}
	s := strconv.FormatFloat(v.Value, 'f', -1, 64)
	return &s
}

func flatten(it models.StockFinancial) *row {
	r := &row{}
	// meta
	r.Ticker = "" // VX financials model in this client version does not include ticker
	r.CIK = it.CIK
	r.SIC = ""
	r.CompanyName = it.CompanyName
	r.SourceFilingFileURL = it.SourceFilingFileUrl
	r.SourceFilingURL = it.SourceFilingUrl

	// period
	r.StartDate = parseISODate(it.StartDate)
	r.EndDate = parseISODate(it.EndDate)
	r.FilingDate = parseISODate(it.FilingDate)
	r.Timeframe = ""
	r.FiscalPeriod = it.FiscalPeriod
	if it.FiscalYear != "" {
		if fy, err := strconv.Atoi(it.FiscalYear); err == nil {
			r.FiscalYear = &fy
		}
	}

	f := it.Financials
	if f != nil {
		// balance sheet
		r.Assets = getFrom(f, "balance_sheet", "assets")
		r.Liabilities = getFrom(f, "balance_sheet", "liabilities")
		r.CurrentAssets = getFrom(f, "balance_sheet", "current_assets")
		r.NoncurrentLiabilities = getFrom(f, "balance_sheet", "noncurrent_liabilities")
		r.LiabilitiesAndEquity = getFrom(f, "balance_sheet", "liabilities_and_equity")
		r.OtherCurrentLiabilities = getFrom(f, "balance_sheet", "other_current_liabilities")
		r.EquityAttributableToNCI = getFrom(f, "balance_sheet", "equity_attributable_to_noncontrolling_interest")
		r.AccountsPayable = getFrom(f, "balance_sheet", "accounts_payable")
		r.OtherNoncurrentAssets = getFrom(f, "balance_sheet", "other_noncurrent_assets")
		r.Inventory = getFrom(f, "balance_sheet", "inventory")
		r.EquityAttributableToParent = getFrom(f, "balance_sheet", "equity_attributable_to_parent")
		r.Equity = getFrom(f, "balance_sheet", "equity")
		r.CurrentLiabilities = getFrom(f, "balance_sheet", "current_liabilities")
		r.NoncurrentAssets = getFrom(f, "balance_sheet", "noncurrent_assets")
		r.IntangibleAssets = getFrom(f, "balance_sheet", "intangible_assets")
		r.OtherCurrentAssets = getFrom(f, "balance_sheet", "other_current_assets")

		// comprehensive income
		r.OtherComprehensiveIncomeLoss = getFrom(f, "comprehensive_income", "other_comprehensive_income_loss")
		r.ComprehensiveIncomeLoss = getFrom(f, "comprehensive_income", "comprehensive_income_loss")
		r.ComprehensiveIncomeLossAttributableToNCI = getFrom(f, "comprehensive_income", "comprehensive_income_loss_attributable_to_noncontrolling_interest")
		r.ComprehensiveIncomeLossAttributableToParent = getFrom(f, "comprehensive_income", "comprehensive_income_loss_attributable_to_parent")
		r.OtherComprehensiveIncomeLossAttributableToParent = getFrom(f, "comprehensive_income", "other_comprehensive_income_loss_attributable_to_parent")

		// income statement
		r.CostOfRevenue = getFrom(f, "income_statement", "cost_of_revenue")
		r.Revenues = getFrom(f, "income_statement", "revenues")
		r.DilutedAverageShares = getFrom(f, "income_statement", "diluted_average_shares")
		r.BasicAverageShares = getFrom(f, "income_statement", "basic_average_shares")
		r.IncomeLossFromContinuingOperationsBeforeTax = getFrom(f, "income_statement", "income_loss_from_continuing_operations_before_tax")
		r.NetIncomeLossAvailableToCommonStockholdersBasic = getFrom(f, "income_statement", "net_income_loss_available_to_common_stockholders_basic")
		r.IncomeLossFromContinuingOperationsAfterTax = getFrom(f, "income_statement", "income_loss_from_continuing_operations_after_tax")
		r.IncomeTaxExpenseBenefit = getFrom(f, "income_statement", "income_tax_expense_benefit")
		r.BasicEarningsPerShare = getFrom(f, "income_statement", "basic_earnings_per_share")
		r.OperatingExpenses = getFrom(f, "income_statement", "operating_expenses")
		r.OperatingIncomeLoss = getFrom(f, "income_statement", "operating_income_loss")
		r.CostsAndExpenses = getFrom(f, "income_statement", "costs_and_expenses")
		r.NonoperatingIncomeLoss = getFrom(f, "income_statement", "nonoperating_income_loss")
		r.PreferredStockDividendsAndOtherAdjustments = getFrom(f, "income_statement", "preferred_stock_dividends_and_other_adjustments")
		r.NetIncomeLossAttributableToParent = getFrom(f, "income_statement", "net_income_loss_attributable_to_parent")
		r.BenefitsCostsExpenses = getFrom(f, "income_statement", "benefits_costs_expenses")
		r.NetIncomeLoss = getFrom(f, "income_statement", "net_income_loss")
		r.SellingGeneralAndAdministrativeExpenses = getFrom(f, "income_statement", "selling_general_and_administrative_expenses")
		r.ParticipatingSecuritiesDistributedAndUndistributedEarningsLossBasic = getFrom(f, "income_statement", "participating_securities_distributed_and_undistributed_earnings_loss_basic")
		r.IncomeTaxExpenseBenefitDeferred = getFrom(f, "income_statement", "income_tax_expense_benefit_deferred")
		r.ResearchAndDevelopment = getFrom(f, "income_statement", "research_and_development")
		r.IncomeLossBeforeEquityMethodInvestments = getFrom(f, "income_statement", "income_loss_before_equity_method_investments")
		r.DilutedEarningsPerShare = getFrom(f, "income_statement", "diluted_earnings_per_share")
		r.NetIncomeLossAttributableToNCI = getFrom(f, "income_statement", "net_income_loss_attributable_to_noncontrolling_interest")
		r.GrossProfit = getFrom(f, "income_statement", "gross_profit")
		r.InterestExpenseOperating = getFrom(f, "income_statement", "interest_expense_operating")

		// cash flow statement
		r.NetCashFlowFromOperatingActivities = getFrom(f, "cash_flow_statement", "net_cash_flow_from_operating_activities")
		r.NetCashFlowContinuing = getFrom(f, "cash_flow_statement", "net_cash_flow_continuing")
		r.NetCashFlowFromOperatingActivitiesContinuing = getFrom(f, "cash_flow_statement", "net_cash_flow_from_operating_activities_continuing")
		r.NetCashFlowFromFinancingActivities = getFrom(f, "cash_flow_statement", "net_cash_flow_from_financing_activities")
		r.NetCashFlow = getFrom(f, "cash_flow_statement", "net_cash_flow")
		r.NetCashFlowFromInvestingActivities = getFrom(f, "cash_flow_statement", "net_cash_flow_from_investing_activities")
		r.NetCashFlowFromFinancingActivitiesContinuing = getFrom(f, "cash_flow_statement", "net_cash_flow_from_financing_activities_continuing")
		r.NetCashFlowFromInvestingActivitiesContinuing = getFrom(f, "cash_flow_statement", "net_cash_flow_from_investing_activities_continuing")
		r.ExchangeGainsLosses = getFrom(f, "cash_flow_statement", "exchange_gains_losses")
	}

	return r
}

// lookupTickerByCIK resolves a ticker from the securities table using a CIK string.
// Uses a small in-memory cache keyed by trimmed CIK to avoid repeated lookups.
func lookupTickerByCIK(ctx context.Context, conn *data.Conn, cik string, cache map[string]string) string {
	if cik == "" || conn == nil || conn.DB == nil {
		return ""
	}
	// Normalize CIK by removing leading zeros for DB compare
	trimmed := strings.TrimLeft(cik, "0")
	if trimmed == "" {
		trimmed = cik
	}
	if t, ok := cache[trimmed]; ok {
		return t
	}
	var ticker string
	// Prefer currently active security
	err := conn.DB.QueryRow(ctx, `SELECT ticker FROM securities WHERE cik = $1 AND maxdate IS NULL LIMIT 1`, trimmed).Scan(&ticker)
	if err != nil || ticker == "" {
		// Fallback: try any matching row
		_ = conn.DB.QueryRow(ctx, `SELECT ticker FROM securities WHERE cik = $1 LIMIT 1`, trimmed).Scan(&ticker)
	}
	if ticker != "" {
		cache[trimmed] = ticker
	}
	return ticker
}

func isoOrEmpty(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.DateOnly)
}

func intPtrToStr(v *int) string {
	if v == nil {
		return ""
	}
	return strconv.Itoa(*v)
}

// getDBMaxFilingDate returns the max filing_date already present in fundamentals as ISO string.
func getDBMaxFilingDate(ctx context.Context, conn *data.Conn) (string, error) {
	const q = `SELECT COALESCE(TO_CHAR(MAX(filing_date), 'YYYY-MM-DD'), '') FROM fundamentals`
	var iso string
	err := conn.DB.QueryRow(ctx, q).Scan(&iso)
	if err != nil {
		return "", err
	}
	return iso, nil
}

// upsertRows inserts or updates rows idempotently using the unique key.
func upsertRows(ctx context.Context, conn *data.Conn, rows []*row) error {
	if len(rows) == 0 {
		return nil
	}
	// Build a single INSERT with multiple VALUES tuples
	// For clarity and to keep parameter count manageable, insert in batches before calling here.
	var (
		sb   strings.Builder
		args []interface{}
		idx  = 1
	)
	sb.WriteString("INSERT INTO fundamentals (" +
		"ticker,cik,sic,company_name,source_filing_file_url,source_filing_url," +
		"start_date,end_date,filing_date,timeframe,fiscal_period,fiscal_year," +
		"assets,liabilities,current_assets,noncurrent_liabilities,liabilities_and_equity,other_current_liabilities,equity_attributable_to_noncontrolling_interest,accounts_payable,other_noncurrent_assets,inventory,equity_attributable_to_parent,equity,current_liabilities,noncurrent_assets,intangible_assets,other_current_assets," +
		"other_comprehensive_income_loss,comprehensive_income_loss,comprehensive_income_loss_attributable_to_noncontrolling_interest,comprehensive_income_loss_attributable_to_parent,other_comprehensive_income_loss_attributable_to_parent," +
		"cost_of_revenue,revenues,diluted_average_shares,basic_average_shares,income_loss_from_continuing_operations_before_tax,net_income_loss_available_to_common_stockholders_basic,income_loss_from_continuing_operations_after_tax,income_tax_expense_benefit,basic_earnings_per_share,operating_expenses,operating_income_loss,costs_and_expenses,nonoperating_income_loss,preferred_stock_dividends_and_other_adjustments,net_income_loss_attributable_to_parent,benefits_costs_expenses,net_income_loss,selling_general_and_administrative_expenses,participating_securities_distributed_and_undistributed_earnings_loss_basic,income_tax_expense_benefit_deferred,research_and_development,income_loss_before_equity_method_investments,diluted_earnings_per_share,net_income_loss_attributable_to_noncontrolling_interest,gross_profit,interest_expense_operating," +
		"net_cash_flow_from_operating_activities,net_cash_flow_continuing,net_cash_flow_from_operating_activities_continuing,net_cash_flow_from_financing_activities,net_cash_flow,net_cash_flow_from_investing_activities,net_cash_flow_from_financing_activities_continuing,net_cash_flow_from_investing_activities_continuing,exchange_gains_losses" +
		") VALUES ")

	addVal := func(s *string) interface{} {
		if s == nil {
			return nil
		}
		return *s
	}

	for i, r := range rows {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("(")
		// meta
		sb.WriteString(fmt.Sprintf("$%d,$%d,$%d,$%d,$%d,$%d,", idx, idx+1, idx+2, idx+3, idx+4, idx+5))
		args = append(args, r.Ticker, r.CIK, r.SIC, r.CompanyName, r.SourceFilingFileURL, r.SourceFilingURL)
		idx += 6
		// dates/period
		sb.WriteString(fmt.Sprintf("$%d,$%d,$%d,$%d,$%d,$%d,", idx, idx+1, idx+2, idx+3, idx+4, idx+5))
		args = append(args, r.StartDate, r.EndDate, r.FilingDate, r.Timeframe, r.FiscalPeriod, r.FiscalYear)
		idx += 6
		// balance sheet (15)
		sb.WriteString(fmt.Sprintf("$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,", idx, idx+1, idx+2, idx+3, idx+4, idx+5, idx+6, idx+7, idx+8, idx+9, idx+10, idx+11, idx+12, idx+13, idx+14))
		args = append(args,
			addVal(r.Assets), addVal(r.Liabilities), addVal(r.CurrentAssets), addVal(r.NoncurrentLiabilities), addVal(r.LiabilitiesAndEquity), addVal(r.OtherCurrentLiabilities), addVal(r.EquityAttributableToNCI), addVal(r.AccountsPayable), addVal(r.OtherNoncurrentAssets), addVal(r.Inventory), addVal(r.EquityAttributableToParent), addVal(r.Equity), addVal(r.CurrentLiabilities), addVal(r.NoncurrentAssets), addVal(r.IntangibleAssets),
		)
		idx += 15
		// other_current_assets
		sb.WriteString(fmt.Sprintf("$%d,", idx))
		args = append(args, addVal(r.OtherCurrentAssets))
		idx += 1
		// comprehensive income (5)
		sb.WriteString(fmt.Sprintf("$%d,$%d,$%d,$%d,$%d,", idx, idx+1, idx+2, idx+3, idx+4))
		args = append(args,
			addVal(r.OtherComprehensiveIncomeLoss), addVal(r.ComprehensiveIncomeLoss), addVal(r.ComprehensiveIncomeLossAttributableToNCI), addVal(r.ComprehensiveIncomeLossAttributableToParent), addVal(r.OtherComprehensiveIncomeLossAttributableToParent),
		)
		idx += 5
		// income statement (24)
		sb.WriteString(fmt.Sprintf("$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,", idx, idx+1, idx+2, idx+3, idx+4, idx+5, idx+6, idx+7, idx+8, idx+9, idx+10, idx+11, idx+12, idx+13, idx+14, idx+15, idx+16, idx+17, idx+18, idx+19, idx+20, idx+21, idx+22, idx+23))
		args = append(args,
			addVal(r.CostOfRevenue), addVal(r.Revenues), addVal(r.DilutedAverageShares), addVal(r.BasicAverageShares), addVal(r.IncomeLossFromContinuingOperationsBeforeTax), addVal(r.NetIncomeLossAvailableToCommonStockholdersBasic), addVal(r.IncomeLossFromContinuingOperationsAfterTax), addVal(r.IncomeTaxExpenseBenefit), addVal(r.BasicEarningsPerShare), addVal(r.OperatingExpenses), addVal(r.OperatingIncomeLoss), addVal(r.CostsAndExpenses), addVal(r.NonoperatingIncomeLoss), addVal(r.PreferredStockDividendsAndOtherAdjustments), addVal(r.NetIncomeLossAttributableToParent), addVal(r.BenefitsCostsExpenses), addVal(r.NetIncomeLoss), addVal(r.SellingGeneralAndAdministrativeExpenses), addVal(r.ParticipatingSecuritiesDistributedAndUndistributedEarningsLossBasic), addVal(r.IncomeTaxExpenseBenefitDeferred), addVal(r.ResearchAndDevelopment), addVal(r.IncomeLossBeforeEquityMethodInvestments), addVal(r.DilutedEarningsPerShare), addVal(r.NetIncomeLossAttributableToNCI),
		)
		idx += 24
		// gross_profit, interest_expense_operating
		sb.WriteString(fmt.Sprintf("$%d,$%d,", idx, idx+1))
		args = append(args, addVal(r.GrossProfit), addVal(r.InterestExpenseOperating))
		idx += 2
		// cash flow (9)
		sb.WriteString(fmt.Sprintf("$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)", idx, idx+1, idx+2, idx+3, idx+4, idx+5, idx+6, idx+7, idx+8))
		args = append(args,
			addVal(r.NetCashFlowFromOperatingActivities), addVal(r.NetCashFlowContinuing), addVal(r.NetCashFlowFromOperatingActivitiesContinuing), addVal(r.NetCashFlowFromFinancingActivities), addVal(r.NetCashFlow), addVal(r.NetCashFlowFromInvestingActivities), addVal(r.NetCashFlowFromFinancingActivitiesContinuing), addVal(r.NetCashFlowFromInvestingActivitiesContinuing), addVal(r.ExchangeGainsLosses),
		)
		idx += 9
	}

	sb.WriteString(" ON CONFLICT (ticker,end_date,timeframe,fiscal_period,fiscal_year) DO UPDATE SET ")
	// Update all mutable columns to new values (exclude PK and unique key components except for allowed updates)
	setCols := []string{
		"cik=EXCLUDED.cik",
		"sic=EXCLUDED.sic",
		"company_name=EXCLUDED.company_name",
		"source_filing_file_url=EXCLUDED.source_filing_file_url",
		"source_filing_url=EXCLUDED.source_filing_url",
		"start_date=EXCLUDED.start_date",
		"filing_date=EXCLUDED.filing_date",
		"assets=EXCLUDED.assets",
		"liabilities=EXCLUDED.liabilities",
		"current_assets=EXCLUDED.current_assets",
		"noncurrent_liabilities=EXCLUDED.noncurrent_liabilities",
		"liabilities_and_equity=EXCLUDED.liabilities_and_equity",
		"other_current_liabilities=EXCLUDED.other_current_liabilities",
		"equity_attributable_to_noncontrolling_interest=EXCLUDED.equity_attributable_to_noncontrolling_interest",
		"accounts_payable=EXCLUDED.accounts_payable",
		"other_noncurrent_assets=EXCLUDED.other_noncurrent_assets",
		"inventory=EXCLUDED.inventory",
		"equity_attributable_to_parent=EXCLUDED.equity_attributable_to_parent",
		"equity=EXCLUDED.equity",
		"current_liabilities=EXCLUDED.current_liabilities",
		"noncurrent_assets=EXCLUDED.noncurrent_assets",
		"intangible_assets=EXCLUDED.intangible_assets",
		"other_current_assets=EXCLUDED.other_current_assets",
		"other_comprehensive_income_loss=EXCLUDED.other_comprehensive_income_loss",
		"comprehensive_income_loss=EXCLUDED.comprehensive_income_loss",
		"comprehensive_income_loss_attributable_to_noncontrolling_interest=EXCLUDED.comprehensive_income_loss_attributable_to_noncontrolling_interest",
		"comprehensive_income_loss_attributable_to_parent=EXCLUDED.comprehensive_income_loss_attributable_to_parent",
		"other_comprehensive_income_loss_attributable_to_parent=EXCLUDED.other_comprehensive_income_loss_attributable_to_parent",
		"cost_of_revenue=EXCLUDED.cost_of_revenue",
		"revenues=EXCLUDED.revenues",
		"diluted_average_shares=EXCLUDED.diluted_average_shares",
		"basic_average_shares=EXCLUDED.basic_average_shares",
		"income_loss_from_continuing_operations_before_tax=EXCLUDED.income_loss_from_continuing_operations_before_tax",
		"net_income_loss_available_to_common_stockholders_basic=EXCLUDED.net_income_loss_available_to_common_stockholders_basic",
		"income_loss_from_continuing_operations_after_tax=EXCLUDED.income_loss_from_continuing_operations_after_tax",
		"income_tax_expense_benefit=EXCLUDED.income_tax_expense_benefit",
		"basic_earnings_per_share=EXCLUDED.basic_earnings_per_share",
		"operating_expenses=EXCLUDED.operating_expenses",
		"operating_income_loss=EXCLUDED.operating_income_loss",
		"costs_and_expenses=EXCLUDED.costs_and_expenses",
		"nonoperating_income_loss=EXCLUDED.nonoperating_income_loss",
		"preferred_stock_dividends_and_other_adjustments=EXCLUDED.preferred_stock_dividends_and_other_adjustments",
		"net_income_loss_attributable_to_parent=EXCLUDED.net_income_loss_attributable_to_parent",
		"benefits_costs_expenses=EXCLUDED.benefits_costs_expenses",
		"net_income_loss=EXCLUDED.net_income_loss",
		"selling_general_and_administrative_expenses=EXCLUDED.selling_general_and_administrative_expenses",
		"participating_securities_distributed_and_undistributed_earnings_loss_basic=EXCLUDED.participating_securities_distributed_and_undistributed_earnings_loss_basic",
		"income_tax_expense_benefit_deferred=EXCLUDED.income_tax_expense_benefit_deferred",
		"research_and_development=EXCLUDED.research_and_development",
		"income_loss_before_equity_method_investments=EXCLUDED.income_loss_before_equity_method_investments",
		"diluted_earnings_per_share=EXCLUDED.diluted_earnings_per_share",
		"net_income_loss_attributable_to_noncontrolling_interest=EXCLUDED.net_income_loss_attributable_to_noncontrolling_interest",
		"gross_profit=EXCLUDED.gross_profit",
		"interest_expense_operating=EXCLUDED.interest_expense_operating",
		"net_cash_flow_from_operating_activities=EXCLUDED.net_cash_flow_from_operating_activities",
		"net_cash_flow_continuing=EXCLUDED.net_cash_flow_continuing",
		"net_cash_flow_from_operating_activities_continuing=EXCLUDED.net_cash_flow_from_operating_activities_continuing",
		"net_cash_flow_from_financing_activities=EXCLUDED.net_cash_flow_from_financing_activities",
		"net_cash_flow=EXCLUDED.net_cash_flow",
		"net_cash_flow_from_investing_activities=EXCLUDED.net_cash_flow_from_investing_activities",
		"net_cash_flow_from_financing_activities_continuing=EXCLUDED.net_cash_flow_from_financing_activities_continuing",
		"net_cash_flow_from_investing_activities_continuing=EXCLUDED.net_cash_flow_from_investing_activities_continuing",
		"exchange_gains_losses=EXCLUDED.exchange_gains_losses",
		"ingested_at=now()"}
	sb.WriteString(strings.Join(setCols, ","))

	sql := sb.String()
	if _, err := data.ExecWithRetry(ctx, conn.DB, sql, args...); err != nil {
		return err
	}
	return nil
}

// progressState holds smoothed timing info per label to improve ETA near the end of the date range
type progressState struct {
	lastCycles int
	lastTime   time.Time
	ewmaBatch  time.Duration
}

var progressStates = map[string]*progressState{}

// logProgressEstimate prints cycles, elapsed time, and an ETA based on date coverage
// from a fixed baseline ISO date (e.g., 2003-01-01) up to now, updated every `every` cycles.
func logProgressEstimate(label string, cycles int, every int, baselineStartISO string, lastISO string, startTime time.Time) {
	if every <= 0 || cycles%every != 0 {
		return
	}
	now := time.Now()
	elapsed := now.Sub(startTime)

	// Per-label EWMA of batch duration for late-stage ETA when date progress saturates
	st, ok := progressStates[label]
	if !ok {
		st = &progressState{lastCycles: cycles, lastTime: now}
		progressStates[label] = st
	} else {
		if cycles > st.lastCycles {
			cycleDelta := cycles - st.lastCycles
			dt := now.Sub(st.lastTime)
			if cycleDelta > 0 && dt > 0 {
				inst := dt / time.Duration(cycleDelta)
				if st.ewmaBatch == 0 {
					st.ewmaBatch = inst
				} else {
					// EWMA with alpha=0.3
					st.ewmaBatch = time.Duration(0.7*float64(st.ewmaBatch) + 0.3*float64(inst))
				}
			}
			st.lastCycles = cycles
			st.lastTime = now
		}
	}

	// Parse baseline start and last completed ISO dates
	baselineStart, errStart := time.Parse(time.DateOnly, baselineStartISO)
	lastDone, errLast := time.Parse(time.DateOnly, lastISO)

	// Guard against invalid timelines
	if errStart != nil || now.Before(baselineStart) {
		log.Printf("⏳ %s: cycles=%d elapsed=%s eta=n/a", label, cycles, elapsed.Truncate(time.Second))
		return
	}

	// Clamp lastDone within [baselineStart, now]
	if errLast != nil || lastISO == "" || lastDone.Before(baselineStart) {
		lastDone = baselineStart
	}
	if lastDone.After(now) {
		lastDone = now
	}

	// Work with whole days for coverage math
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	total := today.Sub(baselineStart) + 24*time.Hour // inclusive of today
	if total <= 0 {
		log.Printf("⏳ %s: cycles=%d elapsed=%s eta=n/a", label, cycles, elapsed.Truncate(time.Second))
		return
	}
	progressed := lastDone.Sub(baselineStart) + 24*time.Hour // inclusive of lastDone day

	// If no progress yet, avoid div-by-zero and show unknown ETA
	if progressed <= 0 {
		log.Printf("⏳ %s: cycles=%d elapsed=%s eta=n/a (through %s)", label, cycles, elapsed.Truncate(time.Second), lastDone.Format(time.DateOnly))
		return
	}

	// If we've already reached today's date, date-coverage saturates and yields eta≈0.
	// Fall back to throughput-based ETA using EWMA batch duration and cycles-per-day observed
	if lastDone.Equal(today) {
		daysProgressed := int(progressed / (24 * time.Hour))
		totalDays := int(total / (24 * time.Hour))
		daysBeforeToday := daysProgressed - 1
		if daysBeforeToday < 1 || st.ewmaBatch <= 0 {
			log.Printf("⏳ %s: cycles=%d elapsed=%s eta=n/a (through %s)", label, cycles, elapsed.Truncate(time.Second), lastDone.Format(time.DateOnly))
			return
		}
		cyclesPerDay := float64(cycles) / float64(daysBeforeToday)
		if cyclesPerDay <= 0 {
			log.Printf("⏳ %s: cycles=%d elapsed=%s eta=n/a (through %s)", label, cycles, elapsed.Truncate(time.Second), lastDone.Format(time.DateOnly))
			return
		}
		estTotalCycles := cyclesPerDay * float64(totalDays)
		remainingCycles := estTotalCycles - float64(cycles)
		if remainingCycles < 0 {
			remainingCycles = 0
		}
		eta := time.Duration(remainingCycles * float64(st.ewmaBatch))
		if eta < time.Second && remainingCycles > 0 {
			// Show sub-second ETAs clearly
			log.Printf("⏳ %s: cycles=%d elapsed=%s eta=<1s (through %s)", label, cycles, elapsed.Truncate(time.Second), lastDone.Format(time.DateOnly))
			return
		}
		log.Printf("⏳ %s: cycles=%d elapsed=%s eta=%s (through %s)", label, cycles, elapsed.Truncate(time.Second), eta.Truncate(time.Second), lastDone.Format(time.DateOnly))
		return
	}

	// Default: date-coverage ETA
	frac := float64(progressed) / float64(total)
	if frac <= 0 {
		log.Printf("⏳ %s: cycles=%d elapsed=%s eta=n/a (through %s)", label, cycles, elapsed.Truncate(time.Second), lastDone.Format(time.DateOnly))
		return
	}
	eta := time.Duration(float64(elapsed) * (1 - frac) / frac)
	if eta < time.Second {
		log.Printf("⏳ %s: cycles=%d elapsed=%s eta=<1s (through %s)", label, cycles, elapsed.Truncate(time.Second), lastDone.Format(time.DateOnly))
		return
	}
	log.Printf("⏳ %s: cycles=%d elapsed=%s eta=%s (through %s)", label, cycles, elapsed.Truncate(time.Second), eta.Truncate(time.Second), lastDone.Format(time.DateOnly))
}
