// <chat.go>
package agent

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"log"

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

// Citation represents a citation/source reference
type Citation struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	StartIndex  int    `json:"start_index,omitempty"`
	EndIndex    int    `json:"end_index,omitempty"`
	PublishDate string `json:"publish_date,omitempty"`
}

// QueryResponse represents the response to a user query
type QueryResponse struct {
	Type          string         `json:"type"` //"mixed_content", "function_calls", "simple_text"
	ContentChunks []ContentChunk `json:"content_chunks,omitempty"`
	Text          string         `json:"text,omitempty"`
	Citations     []Citation     `json:"citations,omitempty"`
	Suggestions   []string       `json:"suggestions,omitempty"`
}

func GetChatRequest(conn *data.Conn, userID int, args json.RawMessage) (interface{}, error) {
	ctx := context.Background()
	success, message := conn.TestRedisConnectivity(ctx, userID)
	if !success {
		return nil, fmt.Errorf("%s", message)
	}

	var query ChatRequest
	if err := json.Unmarshal(args, &query); err != nil {
		return nil, fmt.Errorf("error parsing request: %w", err)
	}
	if defaultSystemPromptTokenCount == 0 {
		getDefaultSystemPromptTokenCount(conn)
	}

	// Save pending message at the start of the request
	pendingSaved := false
	if err := savePendingMessageToConversation(conn, userID, query.Query, query.Context); err != nil {
		log.Printf("Error saving pending message: %v", err)
		// Don't fail the request, just log the error
	} else {
		pendingSaved = true
	}

	// Ensure we clean up pending message on any error
	defer func() {
		if r := recover(); r != nil {
			if pendingSaved {
				// Try to remove the pending message if something panicked
				log.Printf("Panic occurred, attempting to clean up pending message: %v", r)
				// Note: In a real implementation, you might want a more sophisticated cleanup
			}
			panic(r) // Re-panic after cleanup
		}
	}()

	var executor *Executor
	var allResults []ExecuteResult
	planningPrompt := ""
	maxTurns := 7
	totalRequestOutputTokenCount := int32(0)
	totalRequestInputTokenCount := int32(0)
	totalRequestThoughtsTokenCount := int32(0)
	totalRequestTokenCount := int32(0)
	for {
		if planningPrompt == "" {
			var err error
			planningPrompt, err = BuildPlanningPrompt(conn, userID, query.Query, query.Context, query.ActiveChartContext)
			if err != nil {
				return nil, err
			}
		}
		result, err := RunPlanner(ctx, conn, planningPrompt)
		if err != nil {
			return nil, fmt.Errorf("error running planner: %w", err)
		}
		switch v := result.(type) {
		case DirectAnswer:
			processedChunks := processContentChunksForTables(ctx, conn, userID, v.ContentChunks)
			totalRequestOutputTokenCount += v.TokenCounts.OutputTokenCount
			totalRequestInputTokenCount += v.TokenCounts.InputTokenCount
			totalRequestThoughtsTokenCount += v.TokenCounts.ThoughtsTokenCount
			totalRequestTokenCount += v.TokenCounts.TotalTokenCount

			// Update pending message to completed instead of saving new message
			if err := updatePendingMessageToCompleted(conn, userID, query.Query, processedChunks, []FunctionCall{}, []ExecuteResult{}, v.Suggestions, totalRequestTokenCount); err != nil {
				log.Printf("Error updating pending message to completed: %v", err)
				// Fallback to saving new message
				if err := saveMessageToConversation(conn, userID, query.Query, query.Context, processedChunks, []FunctionCall{}, []ExecuteResult{}, v.Suggestions, totalRequestTokenCount); err != nil {
					log.Printf("Error saving message to conversation: %v", err)
				}
			}

			return QueryResponse{
				Type:          "mixed_content",
				ContentChunks: processedChunks,
				Suggestions:   v.Suggestions, // Include suggestions from direct answer
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
				planningPrompt, err = BuildPlanningPromptWithResults(conn, userID, query.Query, query.Context, query.ActiveChartContext, allResults)
				if err != nil {
					return nil, err
				}
			case StageFinishedExecuting:
				// Generate final response based on execution results
				finalPrompt, err := BuildFinalResponsePrompt(conn, userID, query.Query, query.Context, query.ActiveChartContext, allResults)
				if err != nil {
					return nil, err
				}

				// Get the final response from the model (now includes suggestions)
				finalResponse, err := GetFinalResponse(ctx, conn, finalPrompt)
				if err != nil {
					return nil, fmt.Errorf("error generating final response: %w", err)
				}

				totalRequestOutputTokenCount += finalResponse.TokenCounts.OutputTokenCount
				totalRequestInputTokenCount += finalResponse.TokenCounts.InputTokenCount
				totalRequestThoughtsTokenCount += finalResponse.TokenCounts.ThoughtsTokenCount
				totalRequestTokenCount += finalResponse.TokenCounts.TotalTokenCount

				// Process any table instructions in the content chunks
				processedChunks := processContentChunksForTables(ctx, conn, userID, finalResponse.ContentChunks)

				// Update pending message to completed instead of saving new message
				if err := updatePendingMessageToCompleted(conn, userID, query.Query, processedChunks, []FunctionCall{}, allResults, finalResponse.Suggestions, totalRequestTokenCount); err != nil {
					log.Printf("Error updating pending message to completed: %v", err)
					// Fallback to saving new message
					if err := saveMessageToConversation(conn, userID, query.Query, query.Context, processedChunks, []FunctionCall{}, allResults, finalResponse.Suggestions, totalRequestTokenCount); err != nil {
						log.Printf("Error saving message to conversation: %v", err)
					}
				}

				return QueryResponse{
					Type:          "mixed_content",
					ContentChunks: processedChunks,
					Suggestions:   finalResponse.Suggestions, // Include suggestions from final response
				}, nil
			}
		}
		maxTurns--
		if maxTurns <= 0 {
			return nil, fmt.Errorf("model took too many turns to run")
		}
	}
}

// </chat.go>
