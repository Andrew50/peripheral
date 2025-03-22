package tools

import (
	"encoding/json"
	"fmt"

	"backend/utils"
)

// StockMovementSummaryRequest represents the request for a stock movement summary
type StockMovementSummaryRequest struct {
	Ticker    string  `json:"ticker"`
	Timestamp int64   `json:"timestamp"`
	Price     float64 `json:"price"`
}

// StockMovementSummaryResponse represents the response for a stock movement summary
type StockMovementSummaryResponse struct {
	Summary string `json:"summary"`
}

// GetStockMovementSummary queries Perplexity API for information about why a stock moved
func GetStockMovementSummary(conn *utils.Conn, userID int, args json.RawMessage) (interface{}, error) {
	var request StockMovementSummaryRequest
	if err := json.Unmarshal(args, &request); err != nil {
		return nil, fmt.Errorf("error parsing request: %w", err)
	}

	// Get summary from Perplexity using our enhanced function
	summary, err := QueryPerplexityWithDate(request.Ticker, request.Timestamp, request.Price)
	if err != nil {
		return nil, fmt.Errorf("error querying Perplexity: %w", err)
	}

	return StockMovementSummaryResponse{
		Summary: summary,
	}, nil
}
