// <chat.go>
package agent

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"time"

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
	ConversationID     string                   `json:"conversation_id,omitempty"`
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
	ContentChunks  []ContentChunk `json:"content_chunks,omitempty"`
	Citations      []Citation     `json:"citations,omitempty"`
	Suggestions    []string       `json:"suggestions,omitempty"`
	ConversationID string         `json:"conversation_id,omitempty"`
	MessageID      string         `json:"message_id"`
	Timestamp      time.Time      `json:"timestamp"`
	CompletedAt    *time.Time     `json:"completed_at,omitempty"`
}

var defaultSystemPromptTokenCount int

// GetChatRequest is the main context-aware chat request handler
func GetChatRequest(ctx context.Context, conn *data.Conn, userID int, args json.RawMessage) (interface{}, error) {
	// Check if context is already cancelled
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

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

	// Save pending message using the provided conversation ID
	conversationID, messageID, err := SavePendingMessageToConversation(ctx, conn, userID, query.ConversationID, query.Query, query.Context)
	if err != nil {
		return nil, fmt.Errorf("error saving pending message: %w", err)
	}

	// Set up cleanup function to remove pending message on error or cancellation
	defer func() {
		if ctx.Err() != nil {
			// Context was cancelled, clean up the pending message
			if cleanupErr := DeletePendingMessageInConversation(context.Background(), conn, userID, conversationID, query.Query); cleanupErr != nil {
				fmt.Printf("Warning: failed to cleanup pending message after cancellation: %v\n", cleanupErr)
			}
		}
	}()

	var executor *Executor
	var activeResults []ExecuteResult
	var discardedResults []ExecuteResult
	var accumulatedThoughts []string
	planningPrompt := ""
	maxTurns := 15
	totalRequestOutputTokenCount := 0
	totalRequestInputTokenCount := 0
	totalRequestThoughtsTokenCount := 0
	totalRequestTokenCount := 0
	for {
		// Check if context is cancelled during the planning loop
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var firstRound bool
		if planningPrompt == "" {
			firstRound = true
			var err error
			planningPrompt, err = BuildPlanningPromptWithConversationID(conn, userID, conversationID, query.Query, query.Context, query.ActiveChartContext)
			if err != nil {
				// Mark as error instead of deleting for debugging
				if markErr := MarkPendingMessageAsError(ctx, conn, userID, conversationID, query.Query, fmt.Sprintf("Failed to build planning prompt: %v", err)); markErr != nil {
					fmt.Printf("Warning: failed to mark pending message as error: %v\n", markErr)
				}
				return nil, err
			}
		}
		result, err := RunPlanner(ctx, conn, planningPrompt, firstRound)
		if err != nil {
			// Mark as error instead of deleting for debugging
			if markErr := MarkPendingMessageAsError(ctx, conn, userID, conversationID, query.Query, fmt.Sprintf("Planner error: %v", err)); markErr != nil {
				fmt.Printf("Warning: failed to mark pending message as error: %v\n", markErr)
			}
			return nil, fmt.Errorf("error running planner: %w", err)
		}
		switch v := result.(type) {
		case DirectAnswer:
			processedChunks := processContentChunksForTables(ctx, conn, userID, v.ContentChunks)
			totalRequestOutputTokenCount += int(v.TokenCounts.OutputTokenCount)
			totalRequestInputTokenCount += int(v.TokenCounts.InputTokenCount)
			totalRequestThoughtsTokenCount += int(v.TokenCounts.ThoughtsTokenCount)
			totalRequestTokenCount += int(v.TokenCounts.TotalTokenCount)

			// For DirectAnswer, combine all results for storage
			allResults := append(activeResults, discardedResults...)

			// Update pending message to completed and get message data with timestamps
			messageData, err := UpdatePendingMessageToCompletedInConversation(ctx, conn, userID, conversationID, query.Query, processedChunks, []FunctionCall{}, allResults, v.Suggestions, totalRequestTokenCount)
			if err != nil {
				return nil, fmt.Errorf("error updating pending message to completed: %w", err)
			}

			return QueryResponse{
				ContentChunks:  processedChunks,
				Suggestions:    v.Suggestions, // Include suggestions from direct answer
				ConversationID: conversationID,
				MessageID:      messageID,
				Timestamp:      messageData.CreatedAt,
				CompletedAt:    messageData.CompletedAt,
			}, nil
		case Plan:
			// Capture thoughts from this planning iteration
			if v.Thoughts != "" {
				accumulatedThoughts = append(accumulatedThoughts, v.Thoughts)
			}

			// Handle result discarding if specified in the plan
			if len(v.DiscardResults) > 0 {
				// Create a map for quick lookup of IDs to discard
				discardMap := make(map[int64]bool)
				for _, id := range v.DiscardResults {
					discardMap[id] = true
				}

				// Separate active results into kept and discarded
				var newActiveResults []ExecuteResult
				for _, result := range activeResults {
					if discardMap[result.FunctionID] {
						// Move to discarded
						discardedResults = append(discardedResults, result)
					} else {
						// Keep active
						newActiveResults = append(newActiveResults, result)
					}
				}
				activeResults = newActiveResults
			}

			switch v.Stage {
			case StageExecute:
				// Create an executor to handle function calls
				logger, _ := zap.NewProduction()
				if executor == nil {
					executor = NewExecutor(conn, userID, 5, logger)
				}
				for _, round := range v.Rounds {
					// Execute all function calls in this round with context
					results, err := executor.Execute(ctx, round.Calls, round.Parallel)
					if err != nil {
						// Mark as error instead of deleting for debugging
						if markErr := MarkPendingMessageAsError(ctx, conn, userID, conversationID, query.Query, fmt.Sprintf("Execution error: %v", err)); markErr != nil {
							fmt.Printf("Warning: failed to mark pending message as error: %v\n", markErr)
						}
						return nil, fmt.Errorf("error executing function calls: %w", err)
					}
					activeResults = append(activeResults, results...)
				}
				// Update query with active results for next planning iteration
				// Only pass active results to avoid context bloat
				planningPrompt, err = BuildPlanningPromptWithResultsAndConversationID(conn, userID, conversationID, query.Query, query.Context, query.ActiveChartContext, activeResults, accumulatedThoughts)
				if err != nil {
					// Mark as error instead of deleting for debugging
					if markErr := MarkPendingMessageAsError(ctx, conn, userID, conversationID, query.Query, fmt.Sprintf("Failed to build prompt with results: %v", err)); markErr != nil {
						fmt.Printf("Warning: failed to mark pending message as error: %v\n", markErr)
					}
					return nil, err
				}
			case StageFinishedExecuting:
				// Generate final response based on active execution results
				var finalResponse *FinalResponse

				// Get the final response from the model
				finalResponse, err = GetFinalResponseGPT(ctx, conn, userID, query.Query, conversationID, activeResults, accumulatedThoughts)
				if err != nil {
					// Mark as error instead of deleting for debugging
					if markErr := MarkPendingMessageAsError(ctx, conn, userID, conversationID, query.Query, fmt.Sprintf("Final response error: %v", err)); markErr != nil {
						fmt.Printf("Warning: failed to mark pending message as error: %v\n", markErr)
					}
					return nil, fmt.Errorf("error generating final response: %w", err)
				}

				totalRequestOutputTokenCount += int(finalResponse.TokenCounts.OutputTokenCount)
				totalRequestInputTokenCount += int(finalResponse.TokenCounts.InputTokenCount)
				totalRequestThoughtsTokenCount += int(finalResponse.TokenCounts.ThoughtsTokenCount)
				totalRequestTokenCount += int(finalResponse.TokenCounts.TotalTokenCount)

				// Process any table instructions in the content chunks
				processedChunks := processContentChunksForTables(ctx, conn, userID, finalResponse.ContentChunks)

				// For final response, combine all results for storage
				allResults := append(activeResults, discardedResults...)

				// Update pending message to completed and get message data with timestamps
				messageData, err := UpdatePendingMessageToCompletedInConversation(ctx, conn, userID, conversationID, query.Query, processedChunks, []FunctionCall{}, allResults, finalResponse.Suggestions, totalRequestTokenCount)
				if err != nil {
					return nil, fmt.Errorf("error updating pending message to completed: %w", err)
				}

				return QueryResponse{
					ContentChunks:  processedChunks,
					Suggestions:    finalResponse.Suggestions, // Include suggestions from final response
					ConversationID: conversationID,
					MessageID:      messageID,
					Timestamp:      messageData.CreatedAt,
					CompletedAt:    messageData.CompletedAt,
				}, nil
			}
		}
		maxTurns--
		firstRound = false
		if maxTurns <= 0 {
			// Mark as error instead of deleting for debugging
			if markErr := MarkPendingMessageAsError(ctx, conn, userID, conversationID, query.Query, "Model took too many turns to run"); markErr != nil {
				fmt.Printf("Warning: failed to mark pending message as error: %v\n", markErr)
			}
			return nil, fmt.Errorf("model took too many turns to run")
		}
	}
}

// processContentChunksForTables iterates through chunks and generates tables for "backtest_table" type.
func processContentChunksForTables(ctx context.Context, conn *data.Conn, userID int, inputChunks []ContentChunk) []ContentChunk {
	processedChunks := make([]ContentChunk, 0, len(inputChunks))
	for _, chunk := range inputChunks {
		// Check for the type "backtest_table"
		if chunk.Type == "backtest_table" {
			// Attempt to parse the instruction content
			instructionBytes, err := json.Marshal(chunk.Content)
			if err != nil {
				// Replace with an error chunk
				processedChunks = append(processedChunks, ContentChunk{
					Type:    "text",
					Content: fmt.Sprintf("[Internal Error: Could not process table instruction: %v]", err),
				})
				continue
			}

			var instructionData TableInstructionData
			if err := json.Unmarshal(instructionBytes, &instructionData); err != nil {
				// Replace with an error chunk
				processedChunks = append(processedChunks, ContentChunk{
					Type:    "text",
					Content: fmt.Sprintf("[Internal Error: Could not parse table instruction: %v]", err),
				})
				continue
			}

			// Generate the actual table chunk
			tableChunk, err := GenerateBacktestTableFromInstruction(ctx, conn, userID, instructionData)
			if err != nil {
				// Replace with an error chunk
				processedChunks = append(processedChunks, ContentChunk{
					Type:    "text",
					Content: fmt.Sprintf("[Internal Error: Could not generate table: %v]", err),
				})
			} else {
				processedChunks = append(processedChunks, *tableChunk)
			}
		} else {
			// Keep non-instruction chunks as they are
			processedChunks = append(processedChunks, chunk)
		}
	}
	return processedChunks
}

// </chat.go>
