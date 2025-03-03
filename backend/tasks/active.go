package tasks

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
)

type GetActiveArgs struct {
	Timeframe       string   `json:"timeframe"`
	Group           string   `json:"group"`
	Metric          string   `json:"metric"`
	MinMarketCap    *float64 `json:"minMarketCap,omitempty"`
	MaxMarketCap    *float64 `json:"maxMarketCap,omitempty"`
	MinDollarVolume *float64 `json:"minDollarVolume,omitempty"`
	MaxDollarVolume *float64 `json:"maxDollarVolume,omitempty"`
}

type StockResult struct {
	Ticker       string  `json:"ticker"`
	SecurityId   int     `json:"securityId"`
	MarketCap    float64 `json:"market_cap"`
	DollarVolume float64 `json:"dollar_volume"`
}

type GroupConstituent struct {
	Ticker       string  `json:"ticker"`
	SecurityId   int     `json:"securityId"`
	MarketCap    float64 `json:"market_cap"`
	DollarVolume float64 `json:"dollar_volume"`
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
	Group        string             `json:"group,omitempty"`
	Constituents []GroupConstituent `json:"constituents,omitempty"`
}

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

	// Apply filters if provided
	if args.MinMarketCap != nil || args.MaxMarketCap != nil || args.MinDollarVolume != nil || args.MaxDollarVolume != nil {
		filteredResults := []ActiveResult{}

		for _, result := range results {
			if result.Group != "" {
				// Handle sector/industry group results
				if result.Constituents != nil {
					filteredConstituents := []GroupConstituent{}

					for _, constituent := range result.Constituents {
						if isValidResult(constituent.MarketCap, constituent.DollarVolume, args.MinMarketCap, args.MaxMarketCap, args.MinDollarVolume, args.MaxDollarVolume) {
							filteredConstituents = append(filteredConstituents, constituent)
						}
					}

					if len(filteredConstituents) > 0 {
						resultCopy := result
						resultCopy.Constituents = filteredConstituents
						filteredResults = append(filteredResults, resultCopy)
					}
				}
			} else {
				// Handle stock results
				if isValidResult(result.MarketCap, result.DollarVolume, args.MinMarketCap, args.MaxMarketCap, args.MinDollarVolume, args.MaxDollarVolume) {
					filteredResults = append(filteredResults, result)
				}
			}
		}

		return filteredResults, nil
	}

	return results, nil
}

// Helper function to check if a result meets the filter criteria
func isValidResult(marketCap, dollarVolume float64, minMarketCap, maxMarketCap, minDollarVolume, maxDollarVolume *float64) bool {
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

	return true
}
