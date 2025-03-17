package tasks

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"
)

// TradeResponse represents a trade with all its details
type TradeResponse struct {
	TradeID        int             `json:"tradeId"`
	Ticker         string          `json:"ticker"`
	SecurityID     int             `json:"securityId"`
	TradeStart     *int64          `json:"tradeStart"`
	Timestamp      *int64          `json:"timestamp"`
	TradeDirection string          `json:"trade_direction"`
	Date           string          `json:"date"`
	Status         string          `json:"status"`
	OpenQuantity   int64           `json:"openQuantity"`
	ClosedPnL      *float64        `json:"closedPnL"`
	Trades         []TradeActivity `json:"trades"`
}

// TradeActivity represents a single trade activity (entry or exit)
type TradeActivity struct {
	Time   int64   `json:"time"`
	Price  float64 `json:"price"`
	Shares int64   `json:"shares"`
	Type   string  `json:"type"`
}

// TickerStatsResponse represents performance statistics for a ticker
type TickerStatsResponse struct {
	Ticker        string          `json:"ticker"`
	SecurityID    int             `json:"securityId"`
	TotalTrades   int             `json:"total_trades"`
	WinningTrades int             `json:"winning_trades"`
	LosingTrades  int             `json:"losing_trades"`
	AvgPnL        float64         `json:"avg_pnl"`
	TotalPnL      float64         `json:"total_pnl"`
	Timestamp     *int64          `json:"timestamp"`
	Trades        []TradeActivity `json:"trades"`
}

// TradeStatistics represents user's trading statistics
type TradeStatistics struct {
	TotalTrades   int                `json:"total_trades"`
	WinningTrades int                `json:"winning_trades"`
	LosingTrades  int                `json:"losing_trades"`
	WinRate       float64            `json:"win_rate"`
	AvgWin        float64            `json:"avg_win"`
	AvgLoss       float64            `json:"avg_loss"`
	TotalPnL      float64            `json:"total_pnl"`
	TopTrades     []SimpleTrade      `json:"top_trades"`
	BottomTrades  []SimpleTrade      `json:"bottom_trades"`
	HourlyStats   []HourlyStats      `json:"hourly_stats"`
	TickerStats   []TickerStatistics `json:"ticker_stats"`
}

// SimpleTrade represents a simplified trade record for statistics
type SimpleTrade struct {
	Ticker    string  `json:"ticker"`
	Timestamp int64   `json:"timestamp"`
	Direction string  `json:"direction"`
	PnL       float64 `json:"pnl"`
}

// HourlyStats represents trading statistics grouped by hour
type HourlyStats struct {
	Hour          int     `json:"hour"`
	HourDisplay   string  `json:"hour_display"`
	TotalTrades   int     `json:"total_trades"`
	WinningTrades int     `json:"winning_trades"`
	LosingTrades  int     `json:"losing_trades"`
	WinRate       float64 `json:"win_rate"`
	AvgPnL        float64 `json:"avg_pnl"`
	TotalPnL      float64 `json:"total_pnl"`
}

// TickerStatistics represents trading statistics for a ticker
type TickerStatistics struct {
	Ticker        string  `json:"ticker"`
	TotalTrades   int     `json:"total_trades"`
	WinningTrades int     `json:"winning_trades"`
	LosingTrades  int     `json:"losing_trades"`
	WinRate       float64 `json:"win_rate"`
	AvgPnL        float64 `json:"avg_pnl"`
	TotalPnL      float64 `json:"total_pnl"`
}

// Convert database timestamp (Eastern time) to UTC millisecond timestamp
func dbTimeToUTCMillis(dbTime time.Time) int64 {
	// Create an Eastern timezone location
	eastern, _ := time.LoadLocation("America/New_York")

	// Create a new time with the same date/time values but explicitly in Eastern timezone
	// This is crucial - the database time is considered to be in Eastern time
	estTime := time.Date(
		dbTime.Year(), dbTime.Month(), dbTime.Day(),
		dbTime.Hour(), dbTime.Minute(), dbTime.Second(), dbTime.Nanosecond(),
		eastern,
	)

	// Now convert to UTC and get millisecond timestamp
	return estTime.UTC().UnixNano() / int64(time.Millisecond)
}

// GrabUserTrades gets trades for a user with optional filtering
func GrabUserTrades(conn *utils.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	// Parse arguments
	var args struct {
		Sort   string `json:"sort"`
		Date   string `json:"date"`
		Hour   *int   `json:"hour"`
		Ticker string `json:"ticker"`
	}

	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing arguments: %v", err)
	}

	// Set defaults
	if args.Sort == "" {
		args.Sort = "desc"
	}

	// Build query
	baseQuery := `
		SELECT 
			t.*,
			array_length(entry_times, 1) as num_entries,
			array_length(exit_times, 1) as num_exits
		FROM trades t
		WHERE t.userId = $1
	`
	params := []interface{}{userID}
	paramCount := 1

	// Add filters
	if args.Ticker != "" {
		baseQuery += fmt.Sprintf(" AND (t.ticker = $%d OR t.ticker LIKE $%d)", paramCount+1, paramCount+2)
		params = append(params, args.Ticker, args.Ticker+"%")
		paramCount += 2
	}

	if args.Date != "" {
		baseQuery += fmt.Sprintf(" AND DATE(t.entry_times[1]) = $%d", paramCount+1)
		params = append(params, args.Date)
		paramCount++
	}

	if args.Hour != nil {
		baseQuery += fmt.Sprintf(" AND EXTRACT(HOUR FROM t.entry_times[1]) = $%d", paramCount+1)
		params = append(params, *args.Hour)
		paramCount++
	}

	// Add sorting
	sortDirection := "DESC"
	if args.Sort == "asc" {
		sortDirection = "ASC"
	}
	baseQuery += fmt.Sprintf(" ORDER BY t.entry_times[1] %s", sortDirection)

	// Execute query
	rows, err := conn.DB.Query(context.Background(), baseQuery, params...)
	if err != nil {
		return nil, fmt.Errorf("database query error: %v", err)
	}
	defer rows.Close()

	trades := []TradeResponse{}

	// Process results
	for rows.Next() {
		var (
			tradeID        int
			userID         int
			securityID     int
			ticker         string
			tradeDirection string
			date           time.Time
			status         string
			openQuantity   int64
			closedPnL      *float64
			entryTimes     []time.Time
			entryPrices    []float64
			entryShares    []int64
			exitTimes      []time.Time
			exitPrices     []float64
			exitShares     []int64
			numEntries     *int
			numExits       *int
		)

		err := rows.Scan(
			&tradeID, &userID, &securityID, &ticker, &tradeDirection, &date, &status,
			&openQuantity, &closedPnL, &entryTimes, &entryPrices, &entryShares,
			&exitTimes, &exitPrices, &exitShares, &numEntries, &numExits,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning row: %v", err)
		}

		// Create combined trades array
		combinedTrades := []TradeActivity{}

		// Add entries
		if len(entryTimes) > 0 {
			for i := range entryTimes {
				timestamp := dbTimeToUTCMillis(entryTimes[i])

				tradeType := "Buy"
				if tradeDirection == "Short" {
					tradeType = "Short"
				}

				combinedTrades = append(combinedTrades, TradeActivity{
					Time:   timestamp,
					Price:  entryPrices[i],
					Shares: int64(entryShares[i]),
					Type:   tradeType,
				})
			}
		}

		// Add exits
		if len(exitTimes) > 0 {
			for i := range exitTimes {
				timestamp := dbTimeToUTCMillis(exitTimes[i])

				tradeType := "Sell"
				if tradeDirection == "Short" && openQuantity <= 0 {
					tradeType = "Buy to Cover"
				}

				combinedTrades = append(combinedTrades, TradeActivity{
					Time:   timestamp,
					Price:  exitPrices[i],
					Shares: int64(exitShares[i]),
					Type:   tradeType,
				})
			}
		}

		// Sort combined trades by timestamp
		// In Go we'd implement a sort here if needed

		// Calculate timestamps
		var tradeStart, timestamp *int64
		if len(entryTimes) > 0 {
			startTimestamp := dbTimeToUTCMillis(entryTimes[0])
			tradeStart = &startTimestamp
		}

		if len(exitTimes) > 0 {
			endTimestamp := dbTimeToUTCMillis(exitTimes[len(exitTimes)-1])
			timestamp = &endTimestamp
		} else if tradeStart != nil {
			timestamp = tradeStart
		}

		trade := TradeResponse{
			TradeID:        tradeID,
			Ticker:         ticker,
			SecurityID:     securityID,
			TradeStart:     tradeStart,
			Timestamp:      timestamp,
			TradeDirection: tradeDirection,
			Date:           date.Format("2006-01-02"),
			Status:         status,
			OpenQuantity:   int64(openQuantity),
			ClosedPnL:      closedPnL,
			Trades:         combinedTrades,
		}

		trades = append(trades, trade)
	}

	return trades, nil
}

// GetTradeStatistics calculates trading statistics for a user
func GetTradeStatistics(conn *utils.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	// Parse arguments
	var args struct {
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
		Ticker    string `json:"ticker"`
	}

	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing arguments: %v", err)
	}

	// Calculate overall statistics
	query := `
		SELECT 
			COUNT(*) as total_trades,
			COUNT(CASE WHEN closedPnL > 0 THEN 1 END) as winning_trades,
			COUNT(CASE WHEN closedPnL <= 0 THEN 1 END) as losing_trades,
			AVG(CASE WHEN closedPnL > 0 THEN closedPnL END) as avg_win,
			AVG(CASE WHEN closedPnL <= 0 THEN closedPnL END) as avg_loss,
			COALESCE(SUM(closedPnL), 0) as total_pnl
		FROM trades 
		WHERE userId = $1 
		AND status = 'Closed'
		AND closedPnL IS NOT NULL
	`
	params := []interface{}{userID}
	paramCount := 1

	// Add filters
	if args.Ticker != "" {
		query += fmt.Sprintf(" AND (ticker = $%d OR ticker LIKE $%d)", paramCount+1, paramCount+2)
		params = append(params, args.Ticker, args.Ticker+"%")
		paramCount += 2
	}

	if args.StartDate != "" {
		query += fmt.Sprintf(" AND DATE(entry_times[1]) >= $%d", paramCount+1)
		params = append(params, args.StartDate)
		paramCount++
	}

	if args.EndDate != "" {
		query += fmt.Sprintf(" AND DATE(entry_times[1]) <= $%d", paramCount+1)
		params = append(params, args.EndDate)
		paramCount++
	}

	var (
		totalTrades   int
		winningTrades int
		losingTrades  int
		avgWin        *float64
		avgLoss       *float64
		totalPnL      float64
	)

	row := conn.DB.QueryRow(context.Background(), query, params...)
	err := row.Scan(
		&totalTrades,
		&winningTrades,
		&losingTrades,
		&avgWin,
		&avgLoss,
		&totalPnL,
	)
	if err != nil {
		return nil, fmt.Errorf("error scanning statistics: %v", err)
	}

	// Calculate win rate
	winRate := 0.0
	if totalTrades > 0 {
		winRate = math.Round(float64(winningTrades)/float64(totalTrades)*100.0*100) / 100
	}

	// Get avg values, handling NULL
	avgWinValue := 0.0
	if avgWin != nil {
		avgWinValue = math.Round(*avgWin*100) / 100
	}

	avgLossValue := 0.0
	if avgLoss != nil {
		avgLossValue = math.Round(*avgLoss*100) / 100
	}

	// Round total PnL
	totalPnL = math.Round(totalPnL*100) / 100

	// Get top and bottom trades
	topTrades, err := getTopBottomTrades(conn, userID, args.StartDate, args.EndDate, args.Ticker, true)
	if err != nil {
		return nil, fmt.Errorf("error getting top trades: %v", err)
	}

	bottomTrades, err := getTopBottomTrades(conn, userID, args.StartDate, args.EndDate, args.Ticker, false)
	if err != nil {
		return nil, fmt.Errorf("error getting bottom trades: %v", err)
	}

	// Get hourly statistics
	hourlyStats, err := getHourlyStats(conn, userID, args.StartDate, args.EndDate, args.Ticker)
	if err != nil {
		return nil, fmt.Errorf("error getting hourly stats: %v", err)
	}

	// Get ticker statistics
	tickerStats, err := getTickerStats(conn, userID, args.StartDate, args.EndDate, args.Ticker)
	if err != nil {
		return nil, fmt.Errorf("error getting ticker stats: %v", err)
	}

	// Create response
	statistics := TradeStatistics{
		TotalTrades:   totalTrades,
		WinningTrades: winningTrades,
		LosingTrades:  losingTrades,
		WinRate:       winRate,
		AvgWin:        avgWinValue,
		AvgLoss:       avgLossValue,
		TotalPnL:      totalPnL,
		TopTrades:     topTrades,
		BottomTrades:  bottomTrades,
		HourlyStats:   hourlyStats,
		TickerStats:   tickerStats,
	}

	return statistics, nil
}

// getTopBottomTrades gets the top or bottom trades by P&L
func getTopBottomTrades(conn *utils.Conn, userID int, startDate, endDate, ticker string, isTop bool) ([]SimpleTrade, error) {
	// Build query
	query := `
		SELECT 
			ticker,
			entry_times[1] as trade_time,
			tradedirection,
			closedPnL
		FROM trades 
		WHERE userId = $1 
		AND status = 'Closed'
		AND closedPnL IS NOT NULL
	`
	params := []interface{}{userID}
	paramCount := 1

	// Add filters
	if ticker != "" {
		query += fmt.Sprintf(" AND (ticker = $%d OR ticker LIKE $%d)", paramCount+1, paramCount+2)
		params = append(params, ticker, ticker+"%")
		paramCount += 2
	}

	if startDate != "" {
		query += fmt.Sprintf(" AND DATE(entry_times[1]) >= $%d", paramCount+1)
		params = append(params, startDate)
		paramCount++
	}

	if endDate != "" {
		query += fmt.Sprintf(" AND DATE(entry_times[1]) <= $%d", paramCount+1)
		params = append(params, endDate)
		paramCount++
	}

	// Add order and limit
	orderDir := "DESC"
	if !isTop {
		orderDir = "ASC"
	}
	query += fmt.Sprintf(" ORDER BY closedPnL %s LIMIT 5", orderDir)

	// Execute query
	rows, err := conn.DB.Query(context.Background(), query, params...)
	if err != nil {
		return nil, fmt.Errorf("database query error: %v", err)
	}
	defer rows.Close()

	trades := []SimpleTrade{}

	// Process results
	for rows.Next() {
		var (
			ticker         string
			tradeTime      time.Time
			tradeDirection string
			closedPnL      float64
		)

		err := rows.Scan(&ticker, &tradeTime, &tradeDirection, &closedPnL)
		if err != nil {
			return nil, fmt.Errorf("error scanning row: %v", err)
		}

		// Convert EST time to Unix timestamp in milliseconds
		timestamp := dbTimeToUTCMillis(tradeTime)

		trade := SimpleTrade{
			Ticker:    ticker,
			Timestamp: timestamp,
			Direction: tradeDirection,
			PnL:       math.Round(closedPnL*100) / 100,
		}

		trades = append(trades, trade)
	}

	return trades, nil
}

// getHourlyStats gets trading statistics grouped by hour
func getHourlyStats(conn *utils.Conn, userID int, startDate, endDate, ticker string) ([]HourlyStats, error) {
	// Build query
	query := `
		SELECT 
			EXTRACT(HOUR FROM entry_times[1]) as hour,
			COUNT(*) as total_trades,
			COUNT(CASE WHEN closedPnL > 0 THEN 1 END) as winning_trades,
			COUNT(CASE WHEN closedPnL <= 0 THEN 1 END) as losing_trades,
			AVG(closedPnL) as avg_pnl,
			SUM(closedPnL) as total_pnl
		FROM trades 
		WHERE userId = $1 
		AND status = 'Closed'
		AND closedPnL IS NOT NULL
	`
	params := []interface{}{userID}
	paramCount := 1

	// Add filters
	if ticker != "" {
		query += fmt.Sprintf(" AND (ticker = $%d OR ticker LIKE $%d)", paramCount+1, paramCount+2)
		params = append(params, ticker, ticker+"%")
		paramCount += 2
	}

	if startDate != "" {
		query += fmt.Sprintf(" AND DATE(entry_times[1]) >= $%d", paramCount+1)
		params = append(params, startDate)
		paramCount++
	}

	if endDate != "" {
		query += fmt.Sprintf(" AND DATE(entry_times[1]) <= $%d", paramCount+1)
		params = append(params, endDate)
		paramCount++
	}

	query += " GROUP BY hour ORDER BY hour"

	// Execute query
	rows, err := conn.DB.Query(context.Background(), query, params...)
	if err != nil {
		return nil, fmt.Errorf("database query error: %v", err)
	}
	defer rows.Close()

	stats := []HourlyStats{}

	// Process results
	for rows.Next() {
		var (
			hour          float64
			totalTrades   int
			winningTrades int
			losingTrades  int
			avgPnL        *float64
			totalPnL      *float64
		)

		err := rows.Scan(&hour, &totalTrades, &winningTrades, &losingTrades, &avgPnL, &totalPnL)
		if err != nil {
			return nil, fmt.Errorf("error scanning row: %v", err)
		}

		// Calculate win rate
		winRate := 0.0
		if totalTrades > 0 {
			winRate = math.Round(float64(winningTrades)/float64(totalTrades)*100.0*100) / 100
		}

		// Handle NULL values
		avgPnLValue := 0.0
		if avgPnL != nil {
			avgPnLValue = math.Round(*avgPnL*100) / 100
		}

		totalPnLValue := 0.0
		if totalPnL != nil {
			totalPnLValue = math.Round(*totalPnL*100) / 100
		}

		// Format hour display
		hourInt := int(hour)
		hourDisplay := fmt.Sprintf("%02d:00", hourInt)

		stat := HourlyStats{
			Hour:          hourInt,
			HourDisplay:   hourDisplay,
			TotalTrades:   totalTrades,
			WinningTrades: winningTrades,
			LosingTrades:  losingTrades,
			WinRate:       winRate,
			AvgPnL:        avgPnLValue,
			TotalPnL:      totalPnLValue,
		}

		stats = append(stats, stat)
	}

	return stats, nil
}

// getTickerStats gets trading statistics grouped by ticker
func getTickerStats(conn *utils.Conn, userID int, startDate, endDate, filterTicker string) ([]TickerStatistics, error) {
	// Build query
	query := `
		SELECT 
			ticker,
			COUNT(*) as total_trades,
			COUNT(CASE WHEN closedPnL > 0 THEN 1 END) as winning_trades,
			COUNT(CASE WHEN closedPnL <= 0 THEN 1 END) as losing_trades,
			AVG(closedPnL) as avg_pnl,
			SUM(closedPnL) as total_pnl
		FROM trades 
		WHERE userId = $1 
		AND status = 'Closed'
		AND closedPnL IS NOT NULL
	`
	params := []interface{}{userID}
	paramCount := 1

	// Add filters
	if filterTicker != "" {
		query += fmt.Sprintf(" AND (ticker = $%d OR ticker LIKE $%d)", paramCount+1, paramCount+2)
		params = append(params, filterTicker, filterTicker+"%")
		paramCount += 2
	}

	if startDate != "" {
		query += fmt.Sprintf(" AND DATE(entry_times[1]) >= $%d", paramCount+1)
		params = append(params, startDate)
		paramCount++
	}

	if endDate != "" {
		query += fmt.Sprintf(" AND DATE(entry_times[1]) <= $%d", paramCount+1)
		params = append(params, endDate)
		paramCount++
	}

	query += " GROUP BY ticker ORDER BY SUM(closedPnL) DESC"

	// Execute query
	rows, err := conn.DB.Query(context.Background(), query, params...)
	if err != nil {
		return nil, fmt.Errorf("database query error: %v", err)
	}
	defer rows.Close()

	stats := []TickerStatistics{}

	// Process results
	for rows.Next() {
		var (
			ticker        string
			totalTrades   int
			winningTrades int
			losingTrades  int
			avgPnL        *float64
			totalPnL      *float64
		)

		err := rows.Scan(&ticker, &totalTrades, &winningTrades, &losingTrades, &avgPnL, &totalPnL)
		if err != nil {
			return nil, fmt.Errorf("error scanning row: %v", err)
		}

		// Calculate win rate
		winRate := 0.0
		if totalTrades > 0 {
			winRate = math.Round(float64(winningTrades)/float64(totalTrades)*100.0*100) / 100
		}

		// Handle NULL values
		avgPnLValue := 0.0
		if avgPnL != nil {
			avgPnLValue = math.Round(*avgPnL*100) / 100
		}

		totalPnLValue := 0.0
		if totalPnL != nil {
			totalPnLValue = math.Round(*totalPnL*100) / 100
		}

		stat := TickerStatistics{
			Ticker:        ticker,
			TotalTrades:   totalTrades,
			WinningTrades: winningTrades,
			LosingTrades:  losingTrades,
			WinRate:       winRate,
			AvgPnL:        avgPnLValue,
			TotalPnL:      totalPnLValue,
		}

		stats = append(stats, stat)
	}

	return stats, nil
}

// GetTickerPerformance gets performance data for tickers
func GetTickerPerformance(conn *utils.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	// Parse arguments
	var args struct {
		Sort   string `json:"sort"`
		Date   string `json:"date"`
		Hour   *int   `json:"hour"`
		Ticker string `json:"ticker"`
	}

	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing arguments: %v", err)
	}

	// Set defaults
	if args.Sort == "" {
		args.Sort = "desc"
	}

	// Build base SQL query with aggregation functions
	baseQuery := `
		WITH ticker_agg AS (
			SELECT 
				ticker,
				MAX(securityId) as securityId,
				COUNT(*) as total_trades,
				COUNT(CASE WHEN closedPnL > 0 THEN 1 END) as winning_trades,
				COUNT(CASE WHEN closedPnL <= 0 THEN 1 END) as losing_trades,
				AVG(closedPnL) as avg_pnl,
				SUM(closedPnL) as total_pnl,
				MAX(entry_times[1]) as last_trade_time
			FROM trades 
			WHERE userId = $1 
			AND status = 'Closed'
			AND closedPnL IS NOT NULL
	`
	params := []interface{}{userID}
	paramCount := 1

	// Add filters
	if args.Ticker != "" {
		baseQuery += fmt.Sprintf(" AND (ticker = $%d OR ticker LIKE $%d)", paramCount+1, paramCount+2)
		params = append(params, args.Ticker, args.Ticker+"%")
		paramCount += 2
	}

	if args.Date != "" {
		baseQuery += fmt.Sprintf(" AND DATE(entry_times[1]) = $%d", paramCount+1)
		params = append(params, args.Date)
		paramCount++
	}

	if args.Hour != nil {
		baseQuery += fmt.Sprintf(" AND EXTRACT(HOUR FROM entry_times[1]) = $%d", paramCount+1)
		params = append(params, *args.Hour)
		paramCount++
	}

	baseQuery += `
			GROUP BY ticker
		)
		SELECT 
			t.ticker,
			t.securityId,
			t.total_trades,
			t.winning_trades,
			t.losing_trades,
			t.avg_pnl,
			t.total_pnl,
			t.last_trade_time
		FROM ticker_agg t
	`

	// Add sorting
	sortDirection := "DESC"
	if args.Sort == "asc" {
		sortDirection = "ASC"
	}
	baseQuery += fmt.Sprintf(" ORDER BY t.last_trade_time %s", sortDirection)

	// Execute query
	rows, err := conn.DB.Query(context.Background(), baseQuery, params...)
	if err != nil {
		return nil, fmt.Errorf("database query error: %v", err)
	}
	defer rows.Close()

	tickerStats := []TickerStatsResponse{}

	// Process results
	for rows.Next() {
		var (
			ticker        string
			securityID    int
			totalTrades   int
			winningTrades int
			losingTrades  int
			avgPnL        *float64
			totalPnL      *float64
			lastTradeTime time.Time
		)

		err := rows.Scan(
			&ticker,
			&securityID,
			&totalTrades,
			&winningTrades,
			&losingTrades,
			&avgPnL,
			&totalPnL,
			&lastTradeTime,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning row: %v", err)
		}

		// Handle NULL values
		avgPnLValue := 0.0
		if avgPnL != nil {
			avgPnLValue = math.Round(*avgPnL*100) / 100
		}

		totalPnLValue := 0.0
		if totalPnL != nil {
			totalPnLValue = math.Round(*totalPnL*100) / 100
		}

		// Convert EST time to Unix timestamp in milliseconds
		timestamp := dbTimeToUTCMillis(lastTradeTime)
		timestampPtr := &timestamp

		// Create stats object
		tickerStat := TickerStatsResponse{
			Ticker:        ticker,
			SecurityID:    securityID,
			TotalTrades:   totalTrades,
			WinningTrades: winningTrades,
			LosingTrades:  losingTrades,
			AvgPnL:        avgPnLValue,
			TotalPnL:      totalPnLValue,
			Timestamp:     timestampPtr,
			Trades:        []TradeActivity{},
		}

		// Fetch all trade activities for this ticker - both entries and exits
		tradeActivitiesQuery := `
			WITH all_trades AS (
				SELECT tradeId
				FROM trades 
				WHERE userId = $1 AND ticker = $2
				AND status = 'Closed'
				AND closedPnL IS NOT NULL
		`

		// Add the same filters as in the main query
		params := []interface{}{userID, ticker}
		paramCount := 2

		if args.Date != "" {
			tradeActivitiesQuery += fmt.Sprintf(" AND DATE(entry_times[1]) = $%d", paramCount+1)
			params = append(params, args.Date)
			paramCount++
		}

		if args.Hour != nil {
			tradeActivitiesQuery += fmt.Sprintf(" AND EXTRACT(HOUR FROM entry_times[1]) = $%d", paramCount+1)
			params = append(params, *args.Hour)
			paramCount++
		}

		tradeActivitiesQuery += `
			)
			SELECT 
				'entry' as activity_type,
				t.entry_times as times,
				t.entry_prices as prices,
				t.entry_shares as shares,
				t.tradeDirection
			FROM trades t
			JOIN all_trades at ON t.tradeId = at.tradeId
			UNION ALL
			SELECT 
				'exit' as activity_type,
				t.exit_times as times,
				t.exit_prices as prices,
				t.exit_shares as shares,
				t.tradeDirection
			FROM trades t
			JOIN all_trades at ON t.tradeId = at.tradeId
			WHERE array_length(t.exit_times, 1) > 0
		`

		execRows, err := conn.DB.Query(context.Background(), tradeActivitiesQuery, params...)
		if err != nil {
			return nil, fmt.Errorf("error fetching trade activities: %v", err)
		}

		for execRows.Next() {
			var (
				activityType   string
				times          []time.Time
				prices         []float64
				shares         []int64
				tradeDirection string
			)

			err := execRows.Scan(
				&activityType,
				&times,
				&prices,
				&shares,
				&tradeDirection,
			)
			if err != nil {
				execRows.Close()
				return nil, fmt.Errorf("error scanning trade activity: %v", err)
			}

			// Process each time/price/share entry
			for i := range times {
				if i < len(times) && i < len(prices) && i < len(shares) {
					// Convert EST time to Unix timestamp in milliseconds
					entryTimestamp := dbTimeToUTCMillis(times[i])

					tradeType := "Buy"
					if activityType == "entry" && tradeDirection == "Short" {
						tradeType = "Short"
					} else if activityType == "exit" && tradeDirection == "Long" {
						tradeType = "Sell"
					} else if activityType == "exit" && tradeDirection == "Short" {
						tradeType = "Buy to Cover"
					}

					tickerStat.Trades = append(tickerStat.Trades, TradeActivity{
						Time:   entryTimestamp,
						Price:  prices[i],
						Shares: shares[i],
						Type:   tradeType,
					})
				}
			}
		}
		execRows.Close()

		// Sort trades by timestamp (newest first)
		if len(tickerStat.Trades) > 1 {
			sort.Slice(tickerStat.Trades, func(i, j int) bool {
				return tickerStat.Trades[i].Time > tickerStat.Trades[j].Time
			})
		}

		tickerStats = append(tickerStats, tickerStat)
	}

	return tickerStats, nil
}

// DeleteAllUserTrades deletes all trades for a user
func DeleteAllUserTrades(conn *utils.Conn, userID int, _ json.RawMessage) (interface{}, error) {
	// Delete all trade executions for the user first
	execTag, err := conn.DB.Exec(context.Background(),
		"DELETE FROM trade_executions WHERE userId = $1",
		userID)
	if err != nil {
		return map[string]string{
			"status":  "error",
			"message": fmt.Sprintf("Error deleting trade executions: %v", err),
		}, nil
	}
	executionsDeleted := execTag.RowsAffected()

	// Then delete all trades for the user
	tradeTag, err := conn.DB.Exec(context.Background(),
		"DELETE FROM trades WHERE userId = $1",
		userID)
	if err != nil {
		return map[string]string{
			"status":  "error",
			"message": fmt.Sprintf("Error deleting trades: %v", err),
		}, nil
	}
	tradesDeleted := tradeTag.RowsAffected()

	return map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Successfully deleted %d trades and %d trade executions", tradesDeleted, executionsDeleted),
	}, nil
}
