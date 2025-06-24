// <chat.go>
package agent

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type Stage string

const (
	StagePlanMore          Stage = "plan_more"
	StageExecute           Stage = "execute"
	StageFinishedExecuting Stage = "finished_executing"
)

// FunctionCall represents a function to be called with its arguments
type FunctionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
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

// ProgressCallback is a function type for sending progress updates
type ProgressCallback func(message string)

// GetChatRequestWithProgress is the main context-aware chat request handler with progress updates
func GetChatRequestWithProgress(ctx context.Context, conn *data.Conn, userID int, args json.RawMessage, progressCallback ProgressCallback) (interface{}, error) {
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

	progressCallback("Saving message to conversation...")

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
	currentTurn := 0
	totalRequestOutputTokenCount := 0
	totalRequestInputTokenCount := 0
	totalRequestThoughtsTokenCount := 0
	totalRequestTokenCount := 0

	for {
		currentTurn++
		// Check if context is cancelled during the planning loop
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var firstRound bool
		if planningPrompt == "" {
			firstRound = true
			progressCallback("Building planning prompt with conversation context...")
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

		progressCallback(fmt.Sprintf("Running planner (turn %d/%d)...", currentTurn, maxTurns))
		plannerResult, err := RunPlanner(ctx, conn, planningPrompt, firstRound)
		if err != nil {
			// Mark as error instead of deleting for debugging
			if markErr := MarkPendingMessageAsError(ctx, conn, userID, conversationID, query.Query, fmt.Sprintf("Planner error: %v", err)); markErr != nil {
				fmt.Printf("Warning: failed to mark pending message as error: %v\n", markErr)
			}
			return nil, fmt.Errorf("error running planner: %w", err)
		}

		// Type assert the result to either Plan or DirectAnswer
		switch result := plannerResult.(type) {
		case Plan:
			totalRequestOutputTokenCount += int(result.TokenCounts.OutputTokenCount)
			totalRequestInputTokenCount += int(result.TokenCounts.InputTokenCount)
			totalRequestThoughtsTokenCount += int(result.TokenCounts.ThoughtsTokenCount)
			totalRequestTokenCount += int(result.TokenCounts.TotalTokenCount)

			// Add the thoughts to our accumulated list
			if result.Thoughts != "" {
				accumulatedThoughts = append(accumulatedThoughts, result.Thoughts)
			}

			switch result.Stage {
			case StagePlanMore:
				progressCallback("Planner requested more planning...")
				// The planner wants to continue planning
				if result.Thoughts != "" {
					planningPrompt += "\n\nPrevious thoughts: " + result.Thoughts
				}
				continue
			case StageExecute:
				progressCallback("Executing planned functions...")
				// The planner wants to execute some functions
				if executor == nil {
					executor = NewExecutor(conn, userID, 5, nil) // maxWorkers=5, logger=nil
				}

				// Convert Plan's Rounds to FunctionCalls for executor
				var functionCalls []FunctionCall
				for _, round := range result.Rounds {
					functionCalls = append(functionCalls, round.Calls...)
				}

				executeResults, err := executor.Execute(ctx, functionCalls, true)
				if err != nil {
					// Mark as error instead of deleting for debugging
					if markErr := MarkPendingMessageAsError(ctx, conn, userID, conversationID, query.Query, fmt.Sprintf("Execution error: %v", err)); markErr != nil {
						fmt.Printf("Warning: failed to mark pending message as error: %v\n", markErr)
					}
					return nil, fmt.Errorf("error executing functions: %w", err)
				}

				// Categorize results
				for _, execResult := range executeResults {
					if execResult.Error != "" {
						discardedResults = append(discardedResults, execResult)
					} else {
						activeResults = append(activeResults, execResult)
					}
				}

				// Update the planning prompt with execution results
				planningPrompt += "\n\nExecution results:\n"
				for _, execResult := range executeResults {
					if execResult.Error != "" {
						planningPrompt += fmt.Sprintf("Function %s (ERROR): %s\n", execResult.FunctionName, execResult.Error)
					} else {
						planningPrompt += fmt.Sprintf("Function %s: %s\n", execResult.FunctionName, execResult.Result)
					}
				}

				if result.Thoughts != "" {
					planningPrompt += "\n\nPrevious thoughts: " + result.Thoughts
				}
				continue

			case StageFinishedExecuting:
				progressCallback("Generating final response...")
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

				progressCallback("Processing content chunks and finalizing response...")

				// Process any table instructions in the content chunks
				processedChunks := processContentChunksForTables(ctx, conn, userID, finalResponse.ContentChunks)

				// For final response, combine all results for storage
				allResults := append(activeResults, discardedResults...)

				// Update pending message to completed and get message data with timestamps
				messageData, err := UpdatePendingMessageToCompletedInConversation(ctx, conn, userID, conversationID, query.Query, processedChunks, []FunctionCall{}, allResults, finalResponse.Suggestions, totalRequestTokenCount)
				if err != nil {
					return nil, fmt.Errorf("error updating pending message to completed: %w", err)
				}

				progressCallback("Chat processing completed successfully!")

				return QueryResponse{
					ContentChunks:  processedChunks,
					Suggestions:    finalResponse.Suggestions, // Include suggestions from final response
					ConversationID: conversationID,
					MessageID:      messageID,
					Timestamp:      messageData.CreatedAt,
					CompletedAt:    messageData.CompletedAt,
				}, nil
			}

		case DirectAnswer:
			// Handle direct answer case - no planning needed, just return the response
			totalRequestOutputTokenCount += int(result.TokenCounts.OutputTokenCount)
			totalRequestInputTokenCount += int(result.TokenCounts.InputTokenCount)
			totalRequestThoughtsTokenCount += int(result.TokenCounts.ThoughtsTokenCount)
			totalRequestTokenCount += int(result.TokenCounts.TotalTokenCount)

			progressCallback("Processing direct answer...")

			// Process any table instructions in the content chunks
			processedChunks := processContentChunksForTables(ctx, conn, userID, result.ContentChunks)

			// Update pending message to completed and get message data with timestamps
			messageData, err := UpdatePendingMessageToCompletedInConversation(ctx, conn, userID, conversationID, query.Query, processedChunks, []FunctionCall{}, []ExecuteResult{}, result.Suggestions, totalRequestTokenCount)
			if err != nil {
				return nil, fmt.Errorf("error updating pending message to completed: %w", err)
			}

			progressCallback("Chat processing completed successfully!")

			return QueryResponse{
				ContentChunks:  processedChunks,
				Suggestions:    result.Suggestions,
				ConversationID: conversationID,
				MessageID:      messageID,
				Timestamp:      messageData.CreatedAt,
				CompletedAt:    messageData.CompletedAt,
			}, nil

		default:
			return nil, fmt.Errorf("unexpected planner result type: %T", plannerResult)
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

// GetChatRequest is the main context-aware chat request handler (original version)
func GetChatRequest(ctx context.Context, conn *data.Conn, userID int, args json.RawMessage) (interface{}, error) {
	// Use the progress version with a no-op callback
	return GetChatRequestWithProgress(ctx, conn, userID, args, func(message string) {
		// No-op callback for non-streaming requests
	})
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
