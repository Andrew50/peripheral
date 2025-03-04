package tasks

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type GetActiveArgs struct {
	Timeframe       string   `json:"timeframe"`
	Group           string   `json:"group"`
	Metric          string   `json:"metric"`
	MinMarketCap    *float64 `json:"minMarketCap,omitempty"`
	MaxMarketCap    *float64 `json:"maxMarketCap,omitempty"`
	MinDollarVolume *float64 `json:"minDollarVolume,omitempty"`
	MaxDollarVolume *float64 `json:"maxDollarVolume,omitempty"`
	MinADR          *float64 `json:"minADR,omitempty"`
	MaxADR          *float64 `json:"maxADR,omitempty"`
}

type StockResult struct {
	Ticker       string  `json:"ticker"`
	SecurityId   int     `json:"securityId"`
	MarketCap    float64 `json:"market_cap"`
	DollarVolume float64 `json:"dollar_volume"`
	ADR          float64 `json:"adr"`
}

type GroupConstituent struct {
	Ticker       string  `json:"ticker"`
	SecurityId   int     `json:"securityId"`
	MarketCap    float64 `json:"market_cap"`
	DollarVolume float64 `json:"dollar_volume"`
	ADR          float64 `json:"adr"`
}

type GroupResult struct {
	Group        string             `json:"group"`
	Constituents []GroupConstituent `json:"constituents"`
}

// ActiveResult is a union type that can be either StockResult or GroupResult
type ActiveResult struct {
	Ticker       string             `json:"ticker,omitempty"`
	SecurityId   int                `json:"securityId,omitempty"`
	MarketCap    float64            `json:"market_cap,omitempty"`
	DollarVolume float64            `json:"dollar_volume,omitempty"`
	ADR          float64            `json:"adr,omitempty"`
	Group        string             `json:"group,omitempty"`
	Constituents []GroupConstituent `json:"constituents,omitempty"`
}

const MAX_RESULTS = 20 // Number of results to return

func GetActive(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetActiveArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("error parsing args: %w", err)
	}

	cacheKey := fmt.Sprintf("active:%s:%s:%s", args.Timeframe, args.Group, args.Metric)

	// Try to get from cache

	cached, err := conn.Cache.Get(context.Background(), cacheKey).Result()
	if err != nil {
		return []ActiveResult{}, nil // Return empty array if not found
	}

	var results []ActiveResult
	err = json.Unmarshal([]byte(cached), &results)
	if err != nil {
		return []ActiveResult{}, nil // Return empty array if unmarshal fails
	}

	// Apply filters to the results
	filteredResults := filterResults(results, args)

	// Sort the filtered results based on metric (leader/laggard)
	// Leaders should have highest values first, laggards should have lowest values first
	isLeader := strings.Contains(args.Metric, "leader")
	sortResults(filteredResults, isLeader, args.Group)

	// Return only the top MAX_RESULTS items
	if len(filteredResults) > MAX_RESULTS {
		return filteredResults[:MAX_RESULTS], nil
	}

	return filteredResults, nil
}

// sortResults ensures that filtered results are properly sorted before being returned
func sortResults(results []ActiveResult, isLeader bool, group string) {
	if len(results) <= 1 {
		return // No need to sort a single item or empty list
	}

	if group == "stock" {
		// For stocks, sort directly by ADR
		if isLeader {
			// Sort by descending ADR for leaders (highest first)
			sort.Slice(results, func(i, j int) bool {
				return results[i].ADR > results[j].ADR
			})
		} else {
			// Sort by ascending ADR for laggards (lowest first)
			sort.Slice(results, func(i, j int) bool {
				return results[i].ADR < results[j].ADR
			})
		}
	} else {
		// For sectors/industries, sort by their group name
		// This doesn't affect the constituents sorting
		// as that was already done by the Python worker
		sort.Slice(results, func(i, j int) bool {
			return results[i].Group < results[j].Group
		})
	}
}

// Filter results based on provided criteria
func filterResults(results []ActiveResult, args GetActiveArgs) []ActiveResult {
	if args.MinMarketCap == nil && args.MaxMarketCap == nil &&
		args.MinDollarVolume == nil && args.MaxDollarVolume == nil &&
		args.MinADR == nil && args.MaxADR == nil {
		return results // No filtering needed
	}

	filteredResults := []ActiveResult{}

	for _, result := range results {
		if result.Group != "" {
			// Handle sector/industry group results
			if result.Constituents != nil {
				filteredConstituents := []GroupConstituent{}

				for _, constituent := range result.Constituents {
					if isValidResult(
						constituent.MarketCap,
						constituent.DollarVolume,
						constituent.ADR,
						args.MinMarketCap,
						args.MaxMarketCap,
						args.MinDollarVolume,
						args.MaxDollarVolume,
						args.MinADR,
						args.MaxADR) {
						filteredConstituents = append(filteredConstituents, constituent)
					}
				}

				// Only keep the top MAX_RESULTS constituents
				if len(filteredConstituents) > MAX_RESULTS {
					filteredConstituents = filteredConstituents[:MAX_RESULTS]
				}

				if len(filteredConstituents) > 0 {
					resultCopy := result
					resultCopy.Constituents = filteredConstituents
					filteredResults = append(filteredResults, resultCopy)
				}
			}
		} else {
			// Handle stock results
			if isValidResult(
				result.MarketCap,
				result.DollarVolume,
				result.ADR,
				args.MinMarketCap,
				args.MaxMarketCap,
				args.MinDollarVolume,
				args.MaxDollarVolume,
				args.MinADR,
				args.MaxADR) {
				filteredResults = append(filteredResults, result)
			}
		}
	}

	return filteredResults
}

// Helper function to check if a result meets the filter criteria
func isValidResult(
	marketCap,
	dollarVolume,
	adr float64,
	minMarketCap,
	maxMarketCap,
	minDollarVolume,
	maxDollarVolume,
	minADR,
	maxADR *float64) bool {

	// Check market cap filters
	if minMarketCap != nil && marketCap < *minMarketCap {
		return false
	}
	if maxMarketCap != nil && marketCap > *maxMarketCap {
		return false
	}

	// Check dollar volume filters
	if minDollarVolume != nil && dollarVolume < *minDollarVolume {
		return false
	}
	if maxDollarVolume != nil && dollarVolume > *maxDollarVolume {
		return false
	}

	// Check ADR filters
	if minADR != nil && adr < *minADR {
		return false
	}
	if maxADR != nil && adr > *maxADR {
		return false
	}

	return true
}
