package tools

import (
	"encoding/json"
	"fmt"
	"time"

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

	// Convert timestamp to a readable date format
	date := time.Unix(request.Timestamp/1000, 0).Format("January 2, 2006")

	// Create the query for Perplexity
	query := fmt.Sprintf("What factors or news caused %s stock to move on %s? The price was $%.2f. Please provide a concise summary of the main reasons.",
		request.Ticker,
		date,
		request.Price)

	// Get summary from Perplexity
	summary, err := queryPerplexity(query)
	if err != nil {
		return nil, fmt.Errorf("error querying Perplexity: %w", err)
	}

	return StockMovementSummaryResponse{
		Summary: summary,
	}, nil
}
