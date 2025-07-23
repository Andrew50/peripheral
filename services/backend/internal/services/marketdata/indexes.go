// Package marketdata provides functionality for retrieving, processing, and storing
// market data including price history, indexes, and other financial time series.
package marketdata

// Ohlcv1mIndexSQLs returns the index creation SQL commands for the ohlcv_1m table
func Ohlcv1mIndexSQLs() []string {
	return []string{
		`CREATE INDEX IF NOT EXISTS ohlcv_1m_ticker_ts_desc_inc
		ON ohlcv_1m (ticker, "timestamp" DESC)
		INCLUDE (open, high, low, close, volume)`,
	}
}

// Ohlcv1dIndexSQLs returns the index creation SQL commands for the ohlcv_1d table
func Ohlcv1dIndexSQLs() []string {
	return []string{
		`CREATE INDEX IF NOT EXISTS ohlcv_1d_ticker_ts_desc_inc
		ON ohlcv_1d (ticker, "timestamp" DESC)
		INCLUDE (open, high, low, close, volume)`,
	}
}

// IndexSQLs returns all OHLCV index SQLs for convenience (both 1m and 1d)
func IndexSQLs() []string {
	return append(Ohlcv1mIndexSQLs(), Ohlcv1dIndexSQLs()...)
}
