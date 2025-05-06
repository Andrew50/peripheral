package agent

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
)

type Stage string

const (
	StagePlanMore          Stage = "plan_more"
	StageExecute           Stage = "execute"
	StageFinishedExecuting Stage = "finished_executing"
)

// FunctionCall represents a function to be called with its arguments
type FunctionCall struct {
	Name   string          `json:"name"`
	CallID string          `json:"call_id,omitempty"`
	Args   json.RawMessage `json:"args,omitempty"`
}

type ChatRequest struct {
	Query              string                   `json:"query"`
	Context            []map[string]interface{} `json:"context,omitempty"`
	ActiveChartContext map[string]interface{}   `json:"activeChartContext,omitempty"`
}

// ContentChunk represents a piece of content in the response sequence
type ContentChunk struct {
	Type    string      `json:"type"`    // "text" or "table" (or others later, e.g., "image")
	Content interface{} `json:"content"` // string for "text", TableData for "table"
}

func GetChatRequest(conn *utils.Conn, userID int, args json.RawMessage) (interface{}, error) {
	ctx := context.Background()
	success, message := conn.TestRedisConnectivity(ctx, userID)
	if !success {
		fmt.Printf("WARNING: %s\n", message)
	} else {
		fmt.Println(message)
	}

	var query ChatRequest
	if err := json.Unmarshal(args, &query); err != nil {
		return nil, fmt.Errorf("error parsing request: %w", err)
	}

	var executor *Executor
	var allResults []ExecuteResult
	planningPrompt := ""
	maxTurns := 7
	for {
		if planningPrompt == "" {
			planningPrompt = BuildPlanningPrompt(conn, userID, query.Query, query.Context, query.ActiveChartContext)
		}
		result, err := RunPlanner(ctx, conn, planningPrompt)
		if err != nil {
			return nil, fmt.Errorf("error running planner: %w", err)
		}
		switch v := result.(type) {
		case DirectAnswer:
			processedChunks := processContentChunksForTables(ctx, conn, userID, v.ContentChunks)
			return QueryResponse{
				Type:          "mixed_content",
				ContentChunks: processedChunks,
			}, nil
		case Plan:
			switch v.Stage {
			case StagePlanMore:

			case StageExecute:
				// Create an executor to handle function calls
				logger, _ := zap.NewProduction()
				if executor == nil {
					executor = NewExecutor(conn, userID, 3, logger)
				}
				for _, round := range v.Rounds {
					// Execute all function calls in this round
					results, err := executor.Execute(ctx, round.Calls, round.Parallel)
					if err != nil {
						return nil, fmt.Errorf("error executing function calls: %w", err)
					}
					allResults = append(allResults, results...)
				}
				// Update query with results for next planning iteration
				// The planner will process these results to either plan more or finalize
				planningPrompt = BuildPlanningPromptWithResults(conn, userID, query.Query, query.Context, query.ActiveChartContext, allResults)
			case StageFinishedExecuting:
				// Generate final response based on execution results
				finalPrompt := BuildFinalResponsePrompt(conn, userID, query.Query, query.Context, query.ActiveChartContext, allResults)

				// Get the final response from the model
				finalResponse, err := GetFinalResponse(ctx, conn, finalPrompt)
				if err != nil {
					return nil, fmt.Errorf("error generating final response: %w", err)
				}

				// Process any table instructions in the content chunks
				processedChunks := processContentChunksForTables(ctx, conn, userID, finalResponse.ContentChunks)

				return QueryResponse{
					Type:          "mixed_content",
					ContentChunks: processedChunks,
				}, nil
			}
		}
		maxTurns--
		if maxTurns <= 0 {
			return nil, fmt.Errorf("Model took too many turns to run.")
		}
	}

}
