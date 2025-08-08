package account

import (
	"backend/internal/data"
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// HandleTradeUpload processes an uploaded trade file and inserts the trades into the database
func HandleTradeUpload(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	// Parse arguments
	var args struct {
		FileContent string                 `json:"file_content"`
		Extra       map[string]interface{} `json:"extra,omitempty"`
	}

	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing arguments: %v", err)
	}

	// Decode base64 string back to bytes
	fileBytes, err := base64.StdEncoding.DecodeString(args.FileContent)
	if err != nil {
		return nil, fmt.Errorf("error decoding base64 content: %v", err)
	}

	// Read CSV
	reader := csv.NewReader(strings.NewReader(string(fileBytes)))
	// Make the reader more flexible with field counts
	reader.FieldsPerRecord = -1 // Allow variable number of fields
	reader.LazyQuotes = true    // Be more tolerant of quotes in fields

	// Skip first 3 rows (header) - more fault-tolerant approach
	headerRowsToSkip := 3
	for i := 0; i < headerRowsToSkip; i++ {
		_, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				return nil, fmt.Errorf("CSV file has fewer than %d rows", headerRowsToSkip)
			}
			// Continue even if there's an error with the header rows
			continue
		}
	}

	// Read all records
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("error reading CSV data: %v", err)
	}
	////fmt.Println("debug: records", records)
	// Add a check to identify and skip the column headers row
	// Usually column headers contain non-numeric text in fields that should be numeric
	for i := 0; i < len(records); i++ {
		if len(records[i]) >= 4 {
			// Check if this row looks like a header by seeing if the quantity field contains non-numeric text
			quantity := strings.Trim(records[i][3], "\"")
			quantity = strings.ReplaceAll(quantity, ",", "")
			_, err := strconv.ParseFloat(quantity, 64)
			if err != nil && strings.ToLower(quantity) != "" {
				// This is likely a header row, remove it
				if i < len(records)-1 {
					records = append(records[:i], records[i+1:]...)
				} else {
					records = records[:i]
				}
				break
			}
		}
	}

	// Use the EXISTING connection instead of creating a new one
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Start a transaction
	tx, err := conn.DB.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("error starting transaction: %v", err)
	}

	// Ensure we either commit or rollback
	var txClosed bool
	defer func() {
		if !txClosed && tx != nil {
			_ = tx.Rollback(context.Background())
		}
	}()

	// Process each trade
	for i := len(records) - 1; i >= 0; i-- {
		record := records[i]

		// Skip empty rows or rows with too few columns
		if len(record) < 6 || strings.TrimSpace(strings.Join(record, "")) == "" {
			continue
		}

		// Get column data based on CSV format - corrected column order for this format
		symbol := record[0] // Column 0: Symbol
		status := record[1] // Column 1: Status
		// Column 2: Order Type (not used directly)
		quantity := record[3]         // Column 3: Quantity
		tradeDescription := record[4] // Column 4: Trade Description
		orderTime := record[5]        // Column 5: Order Time

		// Clean up fields - remove quotes
		symbol = strings.Trim(symbol, "\"")
		status = strings.Trim(status, "\"")
		quantity = strings.Trim(quantity, "\"")
		tradeDescription = strings.Trim(tradeDescription, "\"")
		orderTime = strings.Trim(orderTime, "\"")

		// Determine trade direction
		tradeDirection := "Long"
		if strings.Contains(tradeDescription, "Short") ||
			strings.Contains(tradeDescription, "Sell to Open") ||
			strings.Contains(tradeDescription, "Buy to Cover") ||
			strings.Contains(tradeDescription, "Buy to Close") {
			tradeDirection = "Short"
		}

		// Parse datetime - handle potential multi-line format
		orderTime = strings.ReplaceAll(orderTime, "\n", " ")
		tradeDateTime, tradeDateStr, err := ParseDateTime(orderTime)
		if err != nil {
			return nil, fmt.Errorf("error parsing date: %v for time string '%s'", err, orderTime)
		}

		// Get security ID for ticker
		securityID, err := GetSecurityIDFromTickerTrades(conn, symbol)
		if err != nil {
			return nil, fmt.Errorf("error getting security ID for ticker '%s': %v", symbol, err)
		}

		// Parse trade price and shares based on trade type
		var tradePrice float64
		var tradeShares float64

		// Remove commas from quantity
		quantity = strings.ReplaceAll(quantity, ",", "")

		// Parse quantity
		quantityVal, err := strconv.ParseFloat(quantity, 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing quantity: %v", err)
		}
		tradeShares = quantityVal

		// Parse trade price based on trade type and status
		if strings.Contains(tradeDescription, "Limit") || strings.Contains(tradeDescription, "Stop Loss") {
			if strings.Contains(status, "FILLED AT") {
				// Extract price from status - format: "FILLED AT $price"
				parts := strings.Split(status, "$")
				if len(parts) < 2 {
					return nil, fmt.Errorf("invalid filled status format: %s", status)
				}
				priceStr := strings.TrimSpace(strings.ReplaceAll(parts[1], ",", ""))
				tradePrice, err = strconv.ParseFloat(priceStr, 64)
				if err != nil {
					return nil, fmt.Errorf("error parsing filled price: %v", err)
				}
			} else if strings.Contains(status, "PARTIAL") {
				// Extract shares from status
				lines := strings.Split(status, "\n")
				if len(lines) < 4 {
					return nil, fmt.Errorf("invalid partial status format: %s", status)
				}
				sharesStr := strings.TrimSpace(strings.Split(lines[3], " ")[0])
				sharesStr = strings.ReplaceAll(sharesStr, ",", "")
				tradeShares, err = strconv.ParseFloat(sharesStr, 64)
				if err != nil {
					return nil, fmt.Errorf("error parsing partial shares: %v", err)
				}

				// Extract price from description
				descParts := strings.Split(tradeDescription, "$")
				if len(descParts) == 2 {
					priceStr := strings.TrimSpace(strings.ReplaceAll(descParts[1], ",", ""))
					tradePrice, err = strconv.ParseFloat(priceStr, 64)
					if err != nil {
						return nil, fmt.Errorf("error parsing description price: %v", err)
					}
				} else if len(descParts) == 3 {
					priceStr := strings.TrimSpace(strings.ReplaceAll(descParts[2], ",", ""))
					tradePrice, err = strconv.ParseFloat(priceStr, 64)
					if err != nil {
						return nil, fmt.Errorf("error parsing description price: %v", err)
					}
				}
			}
		} else if strings.Contains(tradeDescription, "Market") {
			// Extract price from status - format: "something $price"
			parts := strings.Split(status, "$")
			if len(parts) < 2 {
				return nil, fmt.Errorf("invalid market status format: %s", status)
			}
			priceStr := strings.TrimSpace(strings.ReplaceAll(parts[1], ",", ""))
			tradePrice, err = strconv.ParseFloat(priceStr, 64)
			if err != nil {
				return nil, fmt.Errorf("error parsing market price: %v", err)
			}
		}

		// Adjust shares sign based on direction
		if strings.Contains(tradeDescription, "Sell") || strings.Contains(tradeDescription, "Short") {
			tradeShares = -tradeShares
		}

		// Insert trade execution into database
		_, err = tx.Exec(ctx,
			`INSERT INTO trade_executions 
			(userId, securityId, ticker, date, price, size, timestamp, direction)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			userID, securityID, symbol, tradeDateStr, tradePrice, tradeShares, tradeDateTime, tradeDirection)

		if err != nil {
			return nil, fmt.Errorf("error inserting trade execution: %v", err)
		}
	}

	// Commit the transaction
	err = tx.Commit(ctx)
	if err != nil {
		return nil, fmt.Errorf("error committing transaction: %v", err)
	}
	txClosed = true

	// Process trades using the SAME connection
	_, err = ProcessTradesWithinConn(conn, userID)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"status":  "success",
		"message": "Trades uploaded successfully",
	}, nil
}

// ParseDateTime parses a datetime string in various formats and returns a time.Time value and date string
func ParseDateTime(datetimeStr string) (time.Time, string, error) {
	// Clean up the string
	datetimeStr = strings.TrimSpace(datetimeStr)

	// Try different formats
	formats := []string{
		"03:04:05 PM 01/02/2006", // Format like: 07:55:24 PM 03/03/2025
		"15:04:05 01/02/2006",    // 24-hour format
		"01/02/2006 03:04:05 PM", // Alternative order
		"01/02/2006 15:04:05",    // Alternative order with 24-hour
	}

	var t time.Time
	var err error

	for _, format := range formats {
		t, err = time.Parse(format, datetimeStr)
		if err == nil {
			// Create an Eastern timezone location
			eastern, _ := time.LoadLocation("America/New_York")

			// Explicitly set the timezone to Eastern - this is important!
			// Without this, t is in UTC with the wall clock time of the parsed string
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, eastern)

			dateStr := t.Format("2006-01-02")
			return t, dateStr, nil
		}
	}

	return time.Time{}, "", fmt.Errorf("error parsing datetime: %v", err)
}

// GetSecurityIDFromTickerTrades retrieves the security ID for a given ticker
func GetSecurityIDFromTickerTrades(conn *data.Conn, ticker string) (int, error) {
	// For options tickers (like COIN250307P185), extract the base ticker (COIN)
	baseTicker := ticker

	// Check if this is an options ticker (longer than usual and contains C or P)
	if len(ticker) > 6 && (strings.Contains(ticker, "C") || strings.Contains(ticker, "P")) {
		// Find the position where digits start in the ticker
		for i, char := range ticker {
			if i > 0 && char >= '0' && char <= '9' {
				baseTicker = ticker[:i]
				break
			}
		}
	}

	var securityID int
	err := conn.DB.QueryRow(context.Background(),
		"SELECT securityid FROM securities WHERE ticker = $1 ORDER BY maxdate IS NULL DESC, maxdate DESC LIMIT 1",
		baseTicker).Scan(&securityID)

	if err != nil {
		return 0, err
	}

	return securityID, nil
}

// ProcessTradesWithinConn processes unlinked `trade_executions` for a user and
// updates or creates `trades` rows within a single transaction using the
// provided connection. It returns a simple status payload on success.
func ProcessTradesWithinConn(conn *data.Conn, userID int) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tx, err := conn.DB.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("error starting transaction: %v", err)
	}

	var txClosed bool
	defer func() {
		if !txClosed && tx != nil {
			_ = tx.Rollback(context.Background())
		}
	}()

	// --------------------
	// 1) Get unprocessed executions
	// --------------------
	rows, err := tx.Query(ctx,
		`SELECT executionId, userId, securityId, ticker, date, price, size, timestamp, direction, tradeId
         FROM trade_executions 
         WHERE userId = $1 AND tradeId IS NULL
         ORDER BY timestamp ASC`, userID)
	if err != nil {
		return nil, fmt.Errorf("error getting unprocessed executions: %v", err)
	}

	// We need to read them fully before doing other queries on tx
	var allExecutions []struct {
		ExecutionID int
		UserID      int
		SecurityID  int
		Ticker      string
		Date        time.Time
		Price       float64
		Size        int
		Timestamp   time.Time
		Direction   string
		TradeID     *int
	}

	for rows.Next() {
		var ex struct {
			ExecutionID int
			UserID      int
			SecurityID  int
			Ticker      string
			Date        time.Time
			Price       float64
			Size        int
			Timestamp   time.Time
			Direction   string
			TradeID     *int
		}
		if err := rows.Scan(
			&ex.ExecutionID,
			&ex.UserID,
			&ex.SecurityID,
			&ex.Ticker,
			&ex.Date,
			&ex.Price,
			&ex.Size,
			&ex.Timestamp,
			&ex.Direction,
			&ex.TradeID,
		); err != nil {
			rows.Close()
			return nil, fmt.Errorf("error scanning execution row: %v", err)
		}
		allExecutions = append(allExecutions, ex)
	}
	rows.Close()
	// At this point, the rows are fully read/closed, so the connection is free for more queries.

	// --------------------
	// 2) Process each execution
	// --------------------
	for _, ex := range allExecutions {
		executionID := ex.ExecutionID
		ticker := ex.Ticker
		tradeSize := ex.Size
		direction := ex.Direction
		timestamp := ex.Timestamp
		price := ex.Price
		securityID := ex.SecurityID
		date := ex.Date

		// Check for existing open trade
		var openTradeID, oldOpenQty int
		var tradeDir string

		err = tx.QueryRow(ctx,
			`SELECT tradeId, openQuantity, tradeDirection
             FROM trades
             WHERE userId = $1 AND ticker = $2 AND status = 'Open'
             ORDER BY date DESC
             LIMIT 1`, userID, ticker,
		).Scan(&openTradeID, &oldOpenQty, &tradeDir)

		if err != nil {
			if err.Error() == "no rows in result set" {
				// No open trade => create a new one
				var newTradeID int
				err = tx.QueryRow(ctx,
					`INSERT INTO trades (
						userId, securityId, ticker, tradeDirection, date, status, openQuantity,
						entry_times, entry_prices, entry_shares,
						exit_times, exit_prices, exit_shares
					)
					VALUES (
						$1, $2, $3, $4, $5, 'Open', $6,
						ARRAY[$7]::timestamp[], ARRAY[$8]::decimal(10,4)[], ARRAY[$9]::int[],
						ARRAY[]::timestamp[], ARRAY[]::decimal(10,4)[], ARRAY[]::int[]
					)
					RETURNING tradeId`,
					userID, securityID, ticker, direction, date, tradeSize,
					timestamp, price, tradeSize,
				).Scan(&newTradeID)

				if err != nil {
					return nil, fmt.Errorf("error creating new trade: %v", err)
				}

				// Update execution with trade ID
				_, err = tx.Exec(ctx,
					`UPDATE trade_executions SET tradeId = $1 WHERE executionId = $2`,
					newTradeID, executionID)
				if err != nil {
					return nil, fmt.Errorf("error updating execution: %v", err)
				}
			} else {
				// Some other error occurred
				return nil, fmt.Errorf("error checking for open trade: %v", err)
			}
		} else {
			// We have an open trade
			newOpenQty := oldOpenQty + tradeSize

			if tradeDir == direction {
				// same direction
				isSameDirection := (oldOpenQty > 0 && tradeSize > 0) ||
					(oldOpenQty < 0 && tradeSize < 0)

				if isSameDirection {
					// Adding to position
					_, err = tx.Exec(ctx,
						`UPDATE trades
                         SET entry_times = array_append(entry_times, $1),
                             entry_prices = array_append(entry_prices, $2),
                             entry_shares = array_append(entry_shares, $3),
                             openQuantity = $4
                         WHERE tradeId = $5`,
						timestamp, price, tradeSize, newOpenQty, openTradeID)
					if err != nil {
						return nil, fmt.Errorf("error adding to position: %v", err)
					}
				} else {
					// Reducing the same-direction position
					_, err = tx.Exec(ctx,
						`UPDATE trades
                         SET exit_times = array_append(exit_times, $1),
                             exit_prices = array_append(exit_prices, $2),
                             exit_shares = array_append(exit_shares, $3),
                             openQuantity = $4,
                             status = CASE WHEN $4 = 0 THEN 'Closed' ELSE 'Open' END
                         WHERE tradeId = $5`,
						timestamp, price, tradeSize, newOpenQty, openTradeID)
					if err != nil {
						return nil, fmt.Errorf("error reducing position: %v", err)
					}

					// Update P&L for partial exit
					var entryPrices, exitPrices []float64
					var entryShares, exitShares []int
					var td string

					err = tx.QueryRow(ctx,
						`SELECT entry_prices, entry_shares, exit_prices, exit_shares, tradeDirection
                         FROM trades
                         WHERE tradeId = $1`, openTradeID,
					).Scan(&entryPrices, &entryShares, &exitPrices, &exitShares, &td)
					if err != nil {
						return nil, fmt.Errorf("error getting trade data: %v", err)
					}
					updatedPnL := CalculatePnL(entryPrices, entryShares, exitPrices, exitShares, td, ticker)

					_, err = tx.Exec(ctx,
						`UPDATE trades SET closedPnL = $1 WHERE tradeId = $2`,
						updatedPnL, openTradeID)
					if err != nil {
						return nil, fmt.Errorf("error updating PnL: %v", err)
					}
				}

			} else {
				// Opposite direction => reduce position
				_, err = tx.Exec(ctx,
					`UPDATE trades
                     SET exit_times = array_append(exit_times, $1),
                         exit_prices = array_append(exit_prices, $2),
                         exit_shares = array_append(exit_shares, $3),
                         openQuantity = $4,
                         status = CASE WHEN $4 = 0 THEN 'Closed' ELSE 'Open' END
                     WHERE tradeId = $5`,
					timestamp, price, tradeSize, newOpenQty, openTradeID)
				if err != nil {
					return nil, fmt.Errorf("error reducing position (opposite): %v", err)
				}

				// Update P&L
				var entryPrices, exitPrices []float64
				var entryShares, exitShares []int
				var td string

				err = tx.QueryRow(ctx,
					`SELECT entry_prices, entry_shares, exit_prices, exit_shares, tradeDirection
                     FROM trades
                     WHERE tradeId = $1`, openTradeID,
				).Scan(&entryPrices, &entryShares, &exitPrices, &exitShares, &td)
				if err != nil {
					return nil, fmt.Errorf("error getting trade data: %v", err)
				}
				updatedPnL := CalculatePnL(entryPrices, entryShares, exitPrices, exitShares, td, ticker)

				_, err = tx.Exec(ctx,
					`UPDATE trades SET closedPnL = $1 WHERE tradeId = $2`,
					updatedPnL, openTradeID)
				if err != nil {
					return nil, fmt.Errorf("error updating PnL: %v", err)
				}
			}

			// Update the execution record
			_, err = tx.Exec(ctx,
				`UPDATE trade_executions SET tradeId = $1 WHERE executionId = $2`,
				openTradeID, executionID)
			if err != nil {
				return nil, fmt.Errorf("error updating execution: %v", err)
			}
		}
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("error committing transaction: %v", err)
	}
	txClosed = true

	return map[string]string{
		"status":  "success",
		"message": "Trades processed successfully",
	}, nil
}

// CalculatePnL calculates profit and loss for a completed trade
func CalculatePnL(entryPrices []float64, entryShares []int, exitPrices []float64, exitShares []int, direction, ticker string) float64 {
	// Convert int slices to float64 slices for consistent calculation
	entrySharesFloat := make([]float64, len(entryShares))
	for i, shares := range entryShares {
		entrySharesFloat[i] = float64(shares)
	}

	// Convert to decimal for precise calculation
	totalEntryValue := decimal.NewFromFloat(0)
	totalEntryShares := decimal.NewFromFloat(0)
	totalExitValue := decimal.NewFromFloat(0)
	totalExitShares := decimal.NewFromFloat(0)

	// Calculate totals for entries
	for i := range entryPrices {
		price := decimal.NewFromFloat(entryPrices[i])
		shares := decimal.NewFromFloat(math.Abs(entrySharesFloat[i]))
		totalEntryValue = totalEntryValue.Add(price.Mul(shares))
		totalEntryShares = totalEntryShares.Add(shares)
	}

	// Calculate totals for exits
	for i := range exitPrices {
		price := decimal.NewFromFloat(exitPrices[i])
		shares := decimal.NewFromFloat(math.Abs(float64(exitShares[i])))
		totalExitValue = totalExitValue.Add(price.Mul(shares))
		totalExitShares = totalExitShares.Add(shares)
	}

	// Calculate weighted average prices
	var avgEntryPrice, avgExitPrice decimal.Decimal
	if totalEntryShares.GreaterThan(decimal.Zero) {
		avgEntryPrice = totalEntryValue.Div(totalEntryShares)
	} else {
		avgEntryPrice = decimal.Zero
	}

	if totalExitShares.GreaterThan(decimal.Zero) {
		avgExitPrice = totalExitValue.Div(totalExitShares)
	} else {
		avgExitPrice = decimal.Zero
	}

	// Calculate P&L based on direction
	var pnl decimal.Decimal
	if direction == "Long" {
		pnl = avgExitPrice.Sub(avgEntryPrice).Mul(totalExitShares)
	} else { // Short
		pnl = avgEntryPrice.Sub(avgExitPrice).Mul(totalExitShares)
	}

	// Check if it's an options trade
	isOption := len(ticker) > 4 && (strings.Contains(ticker, "C") || strings.Contains(ticker, "P"))
	if isOption {
		pnl = pnl.Mul(decimal.NewFromInt(100)) // Multiply by 100 for options contracts
		totalContracts := totalEntryShares.Add(totalExitShares)

		// Only apply commission if it's not a buy to close under $0.65
		shouldApplyCommission := true
		if direction == "Short" && avgExitPrice.LessThan(decimal.NewFromFloat(0.65)) {
			shouldApplyCommission = false
		}

		if shouldApplyCommission {
			commission := decimal.NewFromFloat(0.65).Mul(totalContracts) // $0.65 per contract
			pnl = pnl.Sub(commission)
		}
	}

	// Round to 2 decimal places and return as float64
	pnlFloat, _ := pnl.Round(2).Float64()
	return pnlFloat
}

// ProcessTrades processes trade data for a given user
func ProcessTrades(conn *data.Conn, userID int) (interface{}, error) {
	return ProcessTradesWithinConn(conn, userID)
}
