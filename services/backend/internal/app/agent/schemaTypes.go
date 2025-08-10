package agent

/*// 1) TEXT --------------------------------------------------------------
type ChunkText struct {
	Type    string `json:"type"    jsonschema:"const=text,required"`
	Content string `json:"content" jsonschema:"required"`
}

// 2) TABLE -------------------------------------------------------------
type TableContent struct {
	Caption string   `json:"caption,omitempty"`
	Headers []string `json:"headers" jsonschema:"required"`
	Rows    [][]any  `json:"rows"    jsonschema:"required"`
}
type ChunkTable struct {
	Type    string       `json:"type"    jsonschema:"const=table,required"`
	Content TableContent `json:"content" jsonschema:"required"`
}

// 3) BACKTEST_TABLE ----------------------------------------------------
type KeyVal struct{ K, V string }
type BacktestContent struct {
	StrategyID   int      `json:"strategyId" jsonschema:"required"`
	Columns      []string `json:"columns"    jsonschema:"required"`
	ColumnFormat []KeyVal `json:"columnFormat,omitempty"`
	ColumnMap    []KeyVal `json:"columnMapping,omitempty"`
}
type ChunkBacktest struct {
	Type    string          `json:"type"    jsonschema:"const=backtest_table,required"`
	Content BacktestContent `json:"content" jsonschema:"required"`
}

// 4) PLOT --------------------------------------------------------------
type PlotTrace struct {
	X, Y       []any
	Name, Type string
}
type PlotContent struct {
	ChartType string      `json:"chart_type" jsonschema:"enum=line,enum=bar,enum=scatter,enum=histogram,enum=heatmap,required"`
	Data      []PlotTrace `json:"data"       jsonschema:"required"`
	Title     string      `json:"title,omitempty"`
}
type ChunkPlot struct {
	Type    string      `json:"type"    jsonschema:"const=plot,required"`
	Content PlotContent `json:"content" jsonschema:"required"`
}

// ---------- the union ----------------------------------------------------- */

// Generic chunk: only two required keys
type AtlantisContentChunk struct {
	Type    string      `json:"type"    jsonschema:"enum=text,enum=table,enum=plot,enum=backtest_table,enum=backtest_plot,enum=agent_plot,required"`
	Content interface{} `json:"content" jsonschema:"required"`
}

// The top-level assistant reply.
type AtlantisFinalResponse struct {
	ContentChunks []AtlantisContentChunk `json:"content_chunks" jsonschema:"required"`
	Suggestions   []string               `json:"suggestions"`
}
type PlanningOutput struct {
	// planning-output fields
	Stage          *string          `json:"stage,omitempty"  jsonschema:"enum=execute,enum=finished_executing,required"`
	Rounds         []ExecutionRound `json:"rounds,omitempty" jsonschema:"required"`
	Thoughts       *string          `json:"thoughts,omitempty" jsonschema:"required"`
	DiscardResults []int            `json:"discard_results,omitempty" jsonschema:"required"`
}

// A single execution round.
// If Parallel is true, all Calls in the round can run concurrently.
type ExecutionRound struct {
	Parallel bool       `json:"parallel" jsonschema:"required"`
	Calls    []ToolCall `json:"calls" jsonschema:"required"`
}

// A planned function/tool invocation.
type ToolCall struct {
	Name string      `json:"name" jsonschema:"required"`
	Args interface{} `json:"args,omitempty" jsonschema:"required"`
}
