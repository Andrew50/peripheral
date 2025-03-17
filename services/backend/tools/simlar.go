package tasks

import (
	"backend/utils"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)
// GetSimilarInstancesArgs represents a structure for handling GetSimilarInstancesArgs data.
type GetSimilarInstancesArgs struct {
	Ticker     string `json:"ticker"`
	SecurityID int    `json:"securityId"`
	Timestamp  int64  `json:"timestamp"`
	Timeframe  string `json:"timeframe"`
}
// GetSimilarInstancesResults represents a structure for handling GetSimilarInstancesResults data.
type GetSimilarInstancesResults struct {
	Ticker     string  `json:"ticker"`
	SecurityID int     `json:"securityId"`
	Timestamp  int64   `json:"timestamp"`
	Timeframe  string  `json:"timeframe"`
	MarketCap  float64 `json:"marketCap"`
	Sector     string  `json:"sector"`
	Industry   string  `json:"industry"`
}
// GetSimilarInstances performs operations related to GetSimilarInstances functionality.
func GetSimilarInstances(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetSimilarInstancesArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("9hsdf invalid args: %v", err)
	}

	// Get reference security details
	var refSecurity struct {
		Ticker    string
		MarketCap float64
		Sector    string
		Industry  string
	}
	err = conn.DB.QueryRow(context.Background(), `
		SELECT 
			ticker, 
			COALESCE(market_cap, 0), 
			COALESCE(sector, ''), 
			COALESCE(industry, '')
		FROM securities 
		WHERE securityId = $1`, args.SecurityID).Scan(
		&refSecurity.Ticker,
		&refSecurity.MarketCap,
		&refSecurity.Sector,
		&refSecurity.Industry,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get reference security: %v", err)
	}

	// Add debug logging
	fmt.Printf("Reference security: %+v\n", refSecurity)

	// Add more detailed debug logging
	var sectorCount, industryCount int
	err = conn.DB.QueryRow(context.Background(), `
		SELECT 
			COUNT(*) FILTER (WHERE sector = $1),
			COUNT(*) FILTER (WHERE sector = $1 AND industry = $2)
		FROM securities 
		WHERE active = true`,
		refSecurity.Sector, refSecurity.Industry).Scan(&sectorCount, &industryCount)

	if err != nil {
		fmt.Printf("Error checking counts: %v\n", err)
	} else {
		fmt.Printf("Found %d securities in sector '%s'\n", sectorCount, refSecurity.Sector)
		fmt.Printf("Found %d securities in industry '%s'\n", industryCount, refSecurity.Industry)
	}

	// Add timestamp debug logging
	var minDate, maxDate time.Time
	err = conn.DB.QueryRow(context.Background(), `
		SELECT MIN(minDate), MAX(maxDate) 
		FROM securities 
		WHERE sector = $1`, refSecurity.Sector).Scan(&minDate, &maxDate)
	if err != nil {
		fmt.Printf("Error checking dates: %v\n", err)
	} else {
		fmt.Printf("Date range for sector: %v to %v\n", minDate, maxDate)
	}

	// Get related tickers from Polygon
	polygonTickers, err := utils.GetPolygonRelatedTickers(conn.Polygon, refSecurity.Ticker)
	if err != nil {
		return nil, fmt.Errorf("failed to get related tickers: %v", err)
	}
	fmt.Printf("Polygon related tickers: %v\n", polygonTickers)

	// Get securities in same sector/industry
	timestamp := time.Unix(args.Timestamp, 0)
	fmt.Printf("Searching for securities at timestamp: %v\n", timestamp)

	// Modified query with debug CTEs
	rows, err := conn.DB.Query(context.Background(), `
		WITH sector_matches AS (
			SELECT 
				ticker, 
				securityId, 
				market_cap::NUMERIC,
				sector,
				industry,
				COALESCE(minDate, '1970-01-01'::timestamp) as minDate,
				COALESCE(maxDate, '2999-12-31'::timestamp) as maxDate,
				CASE 
					WHEN sector = $1 AND industry = $2 THEN 3
					WHEN sector = $1 THEN 2
					ELSE 0
				END as similarity_score
			FROM securities
			WHERE active = true
				AND sector = $1
				AND securityId != $3
				-- Relaxed date filtering
				AND (
					maxDate IS NULL 
					OR maxDate >= (SELECT MIN(minDate) FROM securities WHERE sector = $1)
				)
		),
		sector_debug AS (
			SELECT COUNT(*) as sector_count FROM sector_matches
		),
		polygon_matches AS (
			SELECT 
				ticker, 
				securityId, 
				market_cap::NUMERIC,
				sector,
				industry,
				minDate,
				maxDate,
				1 as similarity_score
			FROM securities
			WHERE active = true
				AND ticker = ANY($4)
				AND securityId != $3
				AND securityId NOT IN (SELECT securityId FROM sector_matches)
				-- Relaxed date filtering for polygon matches
				AND (
					maxDate IS NULL 
					OR maxDate >= (SELECT MIN(minDate) FROM securities)
				)
		),
		polygon_debug AS (
			SELECT COUNT(*) as polygon_count FROM polygon_matches
		),
		combined_matches AS (
			SELECT * FROM sector_matches
			UNION ALL
			SELECT * FROM polygon_matches
		),
		debug_counts AS (
			SELECT 
				(SELECT sector_count FROM sector_debug) as sector_matches,
				(SELECT polygon_count FROM polygon_debug) as polygon_matches
		)
		SELECT 
			m.ticker, 
			m.securityId,
			COALESCE(m.market_cap, 0) as market_cap,
			COALESCE(m.sector, '') as sector,
			COALESCE(m.industry, '') as industry,
			m.similarity_score,
			d.sector_matches,
			d.polygon_matches,
			m.minDate,
			m.maxDate
		FROM combined_matches m
		CROSS JOIN debug_counts d
		ORDER BY similarity_score DESC, 
				 CASE 
					WHEN market_cap > 0 AND $5::NUMERIC > 0 
					THEN ABS(market_cap - $5::NUMERIC) / GREATEST(market_cap, $5::NUMERIC)
					ELSE 1
				 END
		LIMIT 20;
	`, refSecurity.Sector, refSecurity.Industry, args.SecurityID, polygonTickers, refSecurity.MarketCap)

	if err != nil {
		return nil, fmt.Errorf("1imvd: %v", err)
	}
	defer rows.Close()

	var results []GetSimilarInstancesResults
	var sectorMatches, polygonMatches int
	first := true

	for rows.Next() {
		var result GetSimilarInstancesResults
		var similarityScore int
		var minDate, maxDate sql.NullTime
		err := rows.Scan(
			&result.Ticker,
			&result.SecurityID,
			&result.MarketCap,
			&result.Sector,
			&result.Industry,
			&similarityScore,
			&sectorMatches,
			&polygonMatches,
			&minDate,
			&maxDate,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}

		if first {
			fmt.Printf("Debug counts - Sector matches: %d, Polygon matches: %d\n", sectorMatches, polygonMatches)
			first = false
		}

		// Handle NULL dates in the debug output
		minDateStr := "NULL"
		maxDateStr := "NULL"
		if minDate.Valid {
			minDateStr = minDate.Time.String()
		}
		if maxDate.Valid {
			maxDateStr = maxDate.Time.String()
		}

		fmt.Printf("Found similar security: %+v (score: %d, date range: %v to %v)\n",
			result, similarityScore, minDateStr, maxDateStr)

		result.Timestamp = args.Timestamp
		result.Timeframe = args.Timeframe
		results = append(results, result)
	}

	fmt.Printf("Total results found: %d\n", len(results))
	if len(results) == 0 {
		fmt.Printf("No results found for sector: '%s', industry: '%s', timestamp: %v\n",
			refSecurity.Sector, refSecurity.Industry, timestamp)
	}

	return results, nil
}
