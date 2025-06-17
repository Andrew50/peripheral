package strategy

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
)

// BacktestArgs represents arguments for backtesting (kept for API compatibility)
type BacktestArgs struct {
	StrategyID    int   `json:"strategyId"`
	Securities    []int `json:"securities"`
	Start         int64 `json:"start"`
	ReturnWindows []int `json:"returnWindows"`
	FullResults   bool  `json:"fullResults"`
}

// RunBacktest is disabled for the new prompt-based strategy system
func RunBacktest(_ context.Context, _ *data.Conn, _ int, _ json.RawMessage) (any, error) {
	return nil, fmt.Errorf("backtest functionality is currently disabled for prompt-based strategies - coming soon")
}
