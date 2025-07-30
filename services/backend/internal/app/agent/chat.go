// <chat.go>
package agent

import (
	"backend/internal/app/helpers"
	"backend/internal/app/limits"
	"backend/internal/app/strategy"
	"backend/internal/data"
	"backend/internal/services/plotly"
	"backend/internal/services/socket"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/responses"
)

// Active chat cancellation management
var (
	activeChatMu   sync.RWMutex
	activeChatCanc = make(map[int]context.CancelFunc) // key = userID
)

// registerChatCancel stores the cancel function for a user's active chat
func registerChatCancel(userID int, cancel context.CancelFunc) {
	activeChatMu.Lock()
	activeChatCanc[userID] = cancel
	activeChatMu.Unlock()
}

// clearChatCancel removes the cancel function for a user's chat
func clearChatCancel(userID int) {
	activeChatMu.Lock()
	delete(activeChatCanc, userID)
	activeChatMu.Unlock()
}

// Custom types for context keys to avoid collisions
type contextKey string

const (
	CONVERSATION_ID_KEY                        contextKey = "conversationID"
	MESSAGE_ID_KEY                             contextKey = "messageID"
	PERIPHERAL_LATEST_MODEL_THOUGHTS_KEY       contextKey = "peripheralLatestModelThoughts"
	PERIPHERAL_ALREADY_USED_MODEL_THOUGHTS_KEY contextKey = "peripheralAlreadyUsedModelThoughts"
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
type TokenCounts struct {
	InputTokenCount    int64 `json:"input_token_count,omitempty"`
	OutputTokenCount   int64 `json:"output_token_count,omitempty"`
	ThoughtsTokenCount int64 `json:"thoughts_token_count,omitempty"`
	TotalTokenCount    int64 `json:"total_token_count,omitempty"`
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

	// Check if user has usage allowance for queries (skip for user ID 0 which is public access)
	allowed, _, err := limits.CheckUsageAllowed(conn, userID, limits.UsageTypeCredits, 1)
	if err != nil {
		return nil, fmt.Errorf("error checking usage limits: %w", err)
	}
	if !allowed {
		return nil, fmt.Errorf("USAGE_LIMIT_REACHED")
	}

	// Save pending message using the provided conversation ID
	conversationID, messageID, err := SavePendingMessageToConversation(ctx, conn, userID, query.ConversationID, query.Query, query.Context)
	if err != nil {
		return QueryResponse{
			ContentChunks:  []ContentChunk{},
			Suggestions:    []string{},
			ConversationID: query.ConversationID,
			MessageID:      messageID,
			Timestamp:      time.Now(),
		}, fmt.Errorf("error saving pending message: %w", err)
	}
	ctx = context.WithValue(ctx, CONVERSATION_ID_KEY, conversationID)
	ctx = context.WithValue(ctx, MESSAGE_ID_KEY, messageID)
	go socket.SendChatInitializationUpdate(userID, messageID, conversationID)

	// replaced with activeChatMu
	if userID != 0 {
		// Check for existing active chat and cancel it before starting new one
		activeChatMu.Lock()
		if existingCancel, hasActiveChat := activeChatCanc[userID]; hasActiveChat {
			fmt.Printf("duplicate chat detected for user %d, cancelling existing chat\n", userID)
			existingCancel()               // Cancel the existing chat
			delete(activeChatCanc, userID) // Clean up the map entry
		}
		activeChatMu.Unlock()
	}

	// Create cancellable context and register cancel function
	ctx, cancel := context.WithCancel(ctx)
	registerChatCancel(userID, cancel)
	defer clearChatCancel(userID) // guarantees cleanup

	var executor *Executor
	var activeResults []ExecuteResult
	var discardedResults []ExecuteResult
	var accumulatedThoughts []string
	planningPrompt := ""
	maxTurns := 15
	var totalTokenCounts TokenCounts
	totalTokenCounts.InputTokenCount = 0
	totalTokenCounts.OutputTokenCount = 0
	totalTokenCounts.ThoughtsTokenCount = 0
	totalTokenCounts.TotalTokenCount = 0

	for {
		// Check if context is cancelled during the planning loop
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var result interface{}
		var err error
		if planningPrompt == "" {
			planningPrompt, err = BuildPlanningPromptWithConversationID(conn, userID, conversationID, query.Query, query.Context, query.ActiveChartContext)
			if err != nil {
				// Mark as error instead of deleting for debugging
				if markErr := MarkPendingMessageAsError(ctx, conn, userID, conversationID, messageID, fmt.Sprintf("Planner error: %v", err)); markErr != nil {
					fmt.Printf("Warning: failed to mark pending message as error: %v\n", markErr)
				}
				return QueryResponse{
					ContentChunks:  []ContentChunk{},
					Suggestions:    []string{},
					ConversationID: conversationID,
					MessageID:      messageID,
					Timestamp:      time.Now(),
				}, fmt.Errorf("error building planning prompt: %w", err)
			}
			result, err = RunPlanner(ctx, conn, conversationID, userID, planningPrompt, "defaultSystemPrompt", activeResults, accumulatedThoughts)
		} else {
			result, err = RunPlanner(ctx, conn, conversationID, userID, planningPrompt, "IntermediateSystemPrompt", activeResults, accumulatedThoughts)
		}
		if err != nil {
			// Mark as error instead of deleting for debugging
			if markErr := MarkPendingMessageAsError(ctx, conn, userID, conversationID, messageID, fmt.Sprintf("Planner error: %v", err)); markErr != nil {
				fmt.Printf("Warning: failed to mark pending message as error: %v\n", markErr)
			}
			return QueryResponse{
				ContentChunks:  []ContentChunk{},
				Suggestions:    []string{},
				ConversationID: conversationID,
				MessageID:      messageID,
				Timestamp:      time.Now(),
			}, fmt.Errorf("error fetching response from model: %w", err)
		}
		switch v := result.(type) {
		case DirectAnswer:
			totalTokenCounts.OutputTokenCount += int64(v.TokenCounts.OutputTokenCount)
			totalTokenCounts.InputTokenCount += int64(v.TokenCounts.InputTokenCount)
			totalTokenCounts.ThoughtsTokenCount += int64(v.TokenCounts.ThoughtsTokenCount)
			totalTokenCounts.TotalTokenCount += int64(v.TokenCounts.TotalTokenCount)

			// For DirectAnswer, combine all results for storage
			allResults := append(activeResults, discardedResults...)
			// process content chunks for storing in db

			chunksForDB := processContentChunksForDB(ctx, conn, userID, v.ContentChunks)

			// Update pending message to completed and get message data with timestamps
			messageData, err := UpdatePendingMessageToCompletedInConversation(ctx, conn, userID, conversationID, query.Query, chunksForDB, []FunctionCall{}, allResults, v.Suggestions, totalTokenCounts)

			if err != nil {
				return QueryResponse{
					ContentChunks:  v.ContentChunks,
					Suggestions:    v.Suggestions,
					ConversationID: conversationID,
					MessageID:      messageID,
					Timestamp:      time.Now(),
				}, fmt.Errorf("error updating pending message to completed: %w", err)
			}
			// Process any table instructions in the content chunks for frontend viewing for backtest table and backtest plot chunks
			processedChunks := processContentChunksForFrontend(ctx, conn, userID, v.ContentChunks)
			go func() {
				// Record usage and deduct 1 credit now that chat completed successfully
				metadata := map[string]interface{}{
					"query":           query.Query,
					"conversation_id": conversationID,
					"message_id":      messageID,
					"token_count":     totalTokenCounts.TotalTokenCount,
					"result_type":     "direct_answer",
				}
				if err := limits.RecordUsage(conn, userID, limits.UsageTypeCredits, 1, metadata); err != nil {
					fmt.Printf("Warning: Failed to record usage for user %d: %v\n", userID, err)
				}
				// Update conversation plot if there is none yet
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()
				hasPlot, err := HasConversationPlot(ctx, conn, conversationID)
				fmt.Printf("\n\n\nHas plot: %v\n", hasPlot)
				if err != nil {
					fmt.Printf("Warning: failed to check if conversation has plot: %v\n", err)
				}
				if !hasPlot {
					for _, chunk := range v.ContentChunks {
						if chunk.Type == "plot" {
							plotBase64, err := plotly.RenderTwitterPlotToBase64(conn, chunk.Content, false)
							if err != nil {
								fmt.Printf("Warning: failed to render plot: %v\n", err)
								continue
							}
							err = UpdateConversationPlot(ctx, conn, conversationID, plotBase64)
							fmt.Printf("\n\n\nUpdated conversation plot")
							if err != nil {
								fmt.Printf("Warning: failed to update conversation plot: %v\n", err)
								continue
							}
							break
						}
					}
				}
			}()

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
				ctx = context.WithValue(ctx, PERIPHERAL_LATEST_MODEL_THOUGHTS_KEY, v.Thoughts)
			}

			// Handle result discarding if specified in the plan
			if len(v.DiscardResults) > 0 && v.Stage != StageFinishedExecuting {
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
					executor = NewExecutor(conn, userID, 5, logger, conversationID, messageID)
				}
				// Safely extract data before goroutine to prevent race conditions and panics
				if len(v.Rounds) > 0 && len(v.Rounds[0].Calls) > 0 {
					firstCall := v.Rounds[0].Calls[0]
					callName := firstCall.Name
					callArgs := firstCall.Args

					go func() {
						var cleanedModelThoughts string
						if thoughtsValue := ctx.Value(PERIPHERAL_LATEST_MODEL_THOUGHTS_KEY); thoughtsValue != nil && thoughtsValue != ctx.Value(PERIPHERAL_ALREADY_USED_MODEL_THOUGHTS_KEY) {
							ctx = context.WithValue(ctx, PERIPHERAL_ALREADY_USED_MODEL_THOUGHTS_KEY, thoughtsValue)
							if thoughtsStr, ok := thoughtsValue.(string); ok {
								cleanedModelThoughts = cleanStatusMessage(conn, thoughtsStr)
							}
						}

						var argsMap map[string]interface{}
						_ = json.Unmarshal(callArgs, &argsMap)

						// Safely check if the tool exists before accessing its properties
						if tool, exists := Tools[callName]; exists && tool.StatusMessage != "" {
							data := map[string]interface{}{
								"message":  cleanedModelThoughts,
								"headline": formatStatusMessage(tool.StatusMessage, argsMap),
							}
							socket.SendAgentStatusUpdate(userID, "FunctionUpdate", data)
						}
					}()
				}
				for _, round := range v.Rounds {
					// Execute all function calls in this round with context
					results, err := executor.Execute(ctx, round.Calls, round.Parallel)
					if err != nil {
						// Mark as error instead of deleting for debugging
						if markErr := MarkPendingMessageAsError(ctx, conn, userID, conversationID, messageID, fmt.Sprintf("Execution error: %v", err)); markErr != nil {
							fmt.Printf("Warning: failed to mark pending message as error: %v\n", markErr)
						}
						return QueryResponse{
							ContentChunks:  []ContentChunk{},
							Suggestions:    []string{},
							ConversationID: conversationID,
							MessageID:      messageID,
							Timestamp:      time.Now(),
						}, fmt.Errorf("error executing function calls: %w", err)
					}
					activeResults = append(activeResults, results...)
				}
				// Update query with active results for next planning iteration
				// Only pass active results to avoid context bloat
				planningPrompt, err = BuildPlanningPromptWithResultsAndConversationID(conn, userID, conversationID, query.Query, query.Context, query.ActiveChartContext, activeResults, accumulatedThoughts)
				if err != nil {
					// Mark as error instead of deleting for debugging
					if markErr := MarkPendingMessageAsError(ctx, conn, userID, conversationID, messageID, fmt.Sprintf("Failed to build prompt with results: %v", err)); markErr != nil {
						fmt.Printf("Warning: failed to mark pending message as error: %v\n", markErr)
					}
					return QueryResponse{
						ContentChunks:  []ContentChunk{},
						Suggestions:    []string{},
						ConversationID: conversationID,
						MessageID:      messageID,
						Timestamp:      time.Now(),
					}, err
				}
			case StageFinishedExecuting:
				go func() {
					var cleanedModelThoughts string
					if thoughtsValue := ctx.Value(PERIPHERAL_LATEST_MODEL_THOUGHTS_KEY); thoughtsValue != nil && thoughtsValue != ctx.Value(PERIPHERAL_ALREADY_USED_MODEL_THOUGHTS_KEY) {
						ctx = context.WithValue(ctx, PERIPHERAL_ALREADY_USED_MODEL_THOUGHTS_KEY, thoughtsValue)
						if thoughtsStr, ok := thoughtsValue.(string); ok {
							cleanedModelThoughts = cleanStatusMessage(conn, thoughtsStr)
						}
					}
					data := map[string]interface{}{
						"message":  cleanedModelThoughts,
						"headline": "Tying things together",
					}
					socket.SendAgentStatusUpdate(userID, "FunctionUpdate", data)
				}()

				// Generate final response based on active execution results
				var finalResponse *FinalResponse

				// Get the final response from the model
				finalResponse, err = GetFinalResponseGPT(ctx, conn, userID, query.Query, conversationID, messageID, activeResults, accumulatedThoughts)
				if err != nil {
					// Mark as error instead of deleting for debugging
					if markErr := MarkPendingMessageAsError(ctx, conn, userID, conversationID, messageID, fmt.Sprintf("Final response error: %v", err)); markErr != nil {
						fmt.Printf("Warning: failed to mark pending message as error: %v\n", markErr)
					}
					return QueryResponse{
						ContentChunks:  []ContentChunk{},
						Suggestions:    []string{},
						ConversationID: conversationID,
						MessageID:      messageID,
						Timestamp:      time.Now(),
					}, err
				}

				totalTokenCounts.OutputTokenCount += int64(finalResponse.TokenCounts.OutputTokenCount)
				totalTokenCounts.InputTokenCount += int64(finalResponse.TokenCounts.InputTokenCount)
				totalTokenCounts.ThoughtsTokenCount += int64(finalResponse.TokenCounts.ThoughtsTokenCount)
				totalTokenCounts.TotalTokenCount += int64(finalResponse.TokenCounts.TotalTokenCount)

				// For final response, combine all results for storage
				allResults := append(activeResults, discardedResults...)
				// process content chunks for storing in db
				chunksForDB := processContentChunksForDB(ctx, conn, userID, finalResponse.ContentChunks)
				// Update pending message to completed and get message data with timestamps
				messageData, err := UpdatePendingMessageToCompletedInConversation(ctx, conn, userID, conversationID, query.Query, chunksForDB, []FunctionCall{}, allResults, finalResponse.Suggestions, totalTokenCounts)
				if err != nil {
					return QueryResponse{
						ContentChunks:  chunksForDB,
						Suggestions:    finalResponse.Suggestions,
						ConversationID: conversationID,
						MessageID:      messageID,
						Timestamp:      time.Now(),
					}, fmt.Errorf("error updating pending message to completed: %w", err)
				}
				go func() {
					// Record usage and deduct 1 credit now that chat completed successfully
					usageMetadata := map[string]interface{}{
						"query":           query.Query,
						"conversation_id": conversationID,
						"message_id":      messageID,
						"token_count":     totalTokenCounts.TotalTokenCount,
						"result_type":     "final_response",
						"function_count":  len(allResults),
					}
					if err := limits.RecordUsage(conn, userID, limits.UsageTypeCredits, 1, usageMetadata); err != nil {
						fmt.Printf("Warning: Failed to record usage for user %d: %v\n", userID, err)
					}

				}()

				// Process any table instructions in the content chunks for frontend viewing for backtest table and backtest plot chunks
				processedChunks := processContentChunksForFrontend(ctx, conn, userID, finalResponse.ContentChunks)
				go func() {
					ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
					defer cancel()
					hasPlot, err := HasConversationPlot(ctx, conn, conversationID)
					fmt.Printf("\n\n\nHas plot: %v\n", hasPlot)
					if err != nil {
						fmt.Printf("Warning: failed to check if conversation has plot: %v\n", err)
					}
					if !hasPlot {
						for _, chunk := range processedChunks {
							if chunk.Type == "plot" {
								plotBase64, err := plotly.RenderTwitterPlotToBase64(conn, chunk.Content, false)
								if err != nil {
									fmt.Printf("Warning: failed to render plot: %v\n", err)
									continue
								}
								err = UpdateConversationPlot(ctx, conn, conversationID, plotBase64)
								fmt.Printf("\n\n\nUpdated conversation plot")
								if err != nil {
									fmt.Printf("Warning: failed to update conversation plot: %v\n", err)
									continue
								}
								break
							}
						}
					}
				}()
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
		if maxTurns <= 0 {
			// Mark as error instead of deleting for debugging
			if markErr := MarkPendingMessageAsError(ctx, conn, userID, conversationID, messageID, "Model took too many turns to run"); markErr != nil {
				fmt.Printf("Warning: failed to mark pending message as error: %v\n", markErr)
			}
			return QueryResponse{
				ContentChunks:  []ContentChunk{},
				Suggestions:    []string{},
				ConversationID: conversationID,
				MessageID:      messageID,
				Timestamp:      time.Now(),
			}, fmt.Errorf("model took too many turns to run")
		}
	}
}

// StopChatRequest cancels the active chat for this user if any
func StopChatRequest(ctx context.Context, conn *data.Conn, userID int, args json.RawMessage) (interface{}, error) {
	if userID == 0 {
		return map[string]interface{}{
			"success": false,
			"message": "No active chat found to cancel",
		}, nil
	}
	activeChatMu.Lock()
	cancel, ok := activeChatCanc[userID]
	if ok {
		cancel()
		delete(activeChatCanc, userID)
	}
	activeChatMu.Unlock()

	return map[string]interface{}{
		"success": ok,
		"message": func() string { // might want to return no message on cancellation
			if ok {
				return "Chat cancelled successfully"
			}
			return "No active chat found to cancel"
		}(),
	}, nil
}

// TableInstructionData holds the parameters for generating a table from cached data
type BacktestTableChunkData struct {
	StrategyID int         `json:"strategyID"`        // strategyId
	Version    int         `json:"version"`           // version
	Columns    interface{} `json:"columns"`           // Internal column names as either []string or string
	Caption    string      `json:"caption"`           // Table title
	NumRows    int         `json:"numRows,omitempty"` // Number of rows in the table
}
type BacktestPlotChunkData struct {
	StrategyID int    `json:"strategyID"`
	Version    int    `json:"version"`
	PlotID     int    `json:"plotID"`
	ChartType  string `json:"chartType,omitempty"`
	ChartTitle string `json:"chartTitle,omitempty"`
	Length     int    `json:"length,omitempty"`
	XAxisTitle string `json:"xAxisTitle,omitempty"`
	YAxisTitle string `json:"yAxisTitle,omitempty"`
}
type AgentPlotChunkData struct {
	ExecutionID string `json:"executionID"`
	PlotID      int    `json:"plotID"`
	ChartType   string `json:"chartType,omitempty"`
	ChartTitle  string `json:"chartTitle,omitempty"`
	Length      int    `json:"length,omitempty"`
	XAxisTitle  string `json:"xAxisTitle,omitempty"`
	YAxisTitle  string `json:"yAxisTitle,omitempty"`
}

// getBacktestKey creates a composite key for the backtest results map using strategy ID and version
func getBacktestKey(strategyID, version int) string {
	return fmt.Sprintf("%d-%d", strategyID, version)
}

func processContentChunksForDB(ctx context.Context, conn *data.Conn, userID int, inputChunks []ContentChunk) []ContentChunk {
	processedChunks := make([]ContentChunk, 0, len(inputChunks))
	var backtestResultsMap = make(map[string]*strategy.BacktestResponse)
	var agentResultsMap = make(map[string]*RunPythonAgentResponse)
	for _, chunk := range inputChunks {
		if chunk.Type == "backtest_table" {
			var backtestTableChunkContent BacktestTableChunkData
			contentBytes, err := json.Marshal(chunk.Content)
			if err != nil {
				processedChunks = append(processedChunks, ContentChunk{
					Type:    "text",
					Content: "[Internal Error: Could not marshal backtest table chunk content]",
				})
				continue
			}
			if err := json.Unmarshal(contentBytes, &backtestTableChunkContent); err != nil {
				processedChunks = append(processedChunks, ContentChunk{
					Type:    "text",
					Content: "[Internal Error: Invalid backtest table chunk format]",
				})
				continue
			}
			// Get backtest results for this strategy
			backtestKey := getBacktestKey(backtestTableChunkContent.StrategyID, backtestTableChunkContent.Version)
			if backtestResultsMap[backtestKey] == nil {
				backtestResultsMap[backtestKey], err = strategy.GetBacktestFromCache(ctx, conn, userID, backtestTableChunkContent.StrategyID, backtestTableChunkContent.Version)
				if err != nil {
					continue
				}
			}
			if backtestTableChunkContent.Columns == "all" {
				// Store all columns from the backtest results
				backtestTableChunkContent.Columns = backtestResultsMap[backtestKey].Summary.Columns
				chunk.Content = backtestTableChunkContent
			} else {
				// store only columns specified
				chunk.Content = backtestTableChunkContent
			}
			backtestTableChunkContent.NumRows = len(backtestResultsMap[backtestKey].Instances)

			processedChunks = append(processedChunks, chunk)
		} else if chunk.Type == "backtest_plot" {
			var backtestPlotChunkContent BacktestPlotChunkData
			contentBytes, err := json.Marshal(chunk.Content)
			if err != nil {
				processedChunks = append(processedChunks, ContentChunk{
					Type:    "text",
					Content: "[Internal Error: Could not marshal backtest plot chunk content]",
				})
				continue
			}
			if err := json.Unmarshal(contentBytes, &backtestPlotChunkContent); err != nil {
				processedChunks = append(processedChunks, ContentChunk{
					Type:    "text",
					Content: "[Internal Error: Invalid backtest plot chunk format]",
				})
				continue
			}
			backtestKey := getBacktestKey(backtestPlotChunkContent.StrategyID, backtestPlotChunkContent.Version)
			if backtestResultsMap[backtestKey] == nil {
				backtestResultsMap[backtestKey], err = strategy.GetBacktestFromCache(ctx, conn, userID, backtestPlotChunkContent.StrategyID, backtestPlotChunkContent.Version)
				if err != nil {
					continue
				}
			}
			strategyPlots := backtestResultsMap[backtestKey].StrategyPlots
			for _, plot := range strategyPlots {
				if plot.PlotID == backtestPlotChunkContent.PlotID {
					// Extract axis titles safely with nil checks
					var xAxisTitle, yAxisTitle string
					if xaxis, ok := plot.Layout["xaxis"].(map[string]any); ok && xaxis != nil {
						if title, titleOk := xaxis["title"].(string); titleOk {
							xAxisTitle = title
						}
					}
					if yaxis, ok := plot.Layout["yaxis"].(map[string]any); ok && yaxis != nil {
						if title, titleOk := yaxis["title"].(string); titleOk {
							yAxisTitle = title
						}
					}

					chunk.Content = BacktestPlotChunkData{
						StrategyID: backtestPlotChunkContent.StrategyID,
						Version:    backtestPlotChunkContent.Version,
						PlotID:     backtestPlotChunkContent.PlotID,
						ChartType:  plot.ChartType,
						ChartTitle: plot.Title,
						Length:     plot.Length,
						XAxisTitle: xAxisTitle,
						YAxisTitle: yAxisTitle,
					}
					processedChunks = append(processedChunks, chunk)
					break
				}
			}
		} else if chunk.Type == "agent_plot" {
			var agentPlotChunkContent AgentPlotChunkData
			contentBytes, err := json.Marshal(chunk.Content)
			if err != nil {
				processedChunks = append(processedChunks, ContentChunk{
					Type:    "text",
					Content: "[Internal Error: Could not marshal agent plot chunk content]",
				})
				continue
			}
			if err := json.Unmarshal(contentBytes, &agentPlotChunkContent); err != nil {
				processedChunks = append(processedChunks, ContentChunk{
					Type:    "text",
					Content: "[Internal Error: Invalid agent plot chunk format]",
				})
				continue
			}
			if agentResultsMap[agentPlotChunkContent.ExecutionID] == nil {
				agentResultsMap[agentPlotChunkContent.ExecutionID], err = GetPythonAgentResultFromCache(ctx, conn, agentPlotChunkContent.ExecutionID)
				if err != nil {
					continue
				}
			}
			agentPlots := agentResultsMap[agentPlotChunkContent.ExecutionID].Plots
			for _, plot := range agentPlots {
				if plot.PlotID == agentPlotChunkContent.PlotID {
					var xAxisTitle, yAxisTitle string
					if xaxis, ok := plot.Layout["xaxis"].(map[string]any); ok && xaxis != nil {
						if title, titleOk := xaxis["title"].(string); titleOk {
							xAxisTitle = title
						}
					}
					if yaxis, ok := plot.Layout["yaxis"].(map[string]any); ok && yaxis != nil {
						if title, titleOk := yaxis["title"].(string); titleOk {
							yAxisTitle = title
						}
					}
					chunk.Content = AgentPlotChunkData{
						ExecutionID: agentPlotChunkContent.ExecutionID,
						PlotID:      agentPlotChunkContent.PlotID,
						ChartType:   plot.ChartType,
						ChartTitle:  plot.Title,
						Length:      plot.Length,
						XAxisTitle:  xAxisTitle,
						YAxisTitle:  yAxisTitle,
					}
					processedChunks = append(processedChunks, chunk)
					break
				}
			}
		} else {
			processedChunks = append(processedChunks, chunk)
		}
	}
	return processedChunks
}

// processContentChunksForFrontend iterates through chunks and generates tables for "backtest_table" type.
func processContentChunksForFrontend(ctx context.Context, conn *data.Conn, userID int, inputChunks []ContentChunk) []ContentChunk {
	processedChunks := make([]ContentChunk, 0, len(inputChunks))
	var backtestResultsMap = make(map[string]*strategy.BacktestResponse)
	var agentResultsMap = make(map[string]*RunPythonAgentResponse)
	for _, chunk := range inputChunks {
		// Check for the type "backtest_table"
		if chunk.Type == "backtest_table" {
			var chunkContent BacktestTableChunkData
			contentBytes, err := json.Marshal(chunk.Content)
			if err != nil {
				processedChunks = append(processedChunks, ContentChunk{
					Type:    "text",
					Content: "[Internal Error: Could not marshal backtest table chunk content]",
				})
				continue
			}
			if err := json.Unmarshal(contentBytes, &chunkContent); err != nil {
				processedChunks = append(processedChunks, ContentChunk{
					Type:    "text",
					Content: "[Internal Error: Invalid backtest table chunk format]",
				})
				continue
			}
			backtestKey := getBacktestKey(chunkContent.StrategyID, chunkContent.Version)
			if backtestResultsMap[backtestKey] == nil {
				backtestResultsMap[backtestKey], err = strategy.GetBacktestFromCache(ctx, conn, userID, chunkContent.StrategyID, chunkContent.Version)
				if err != nil {
					processedChunks = append(processedChunks, ContentChunk{
						Type:    "text",
						Content: fmt.Sprintf("[Internal Error: Could not get backtest results: %v]", err),
					})
					continue
				}
			}
			backtestInstances := backtestResultsMap[backtestKey].Instances
			backtestColumns := backtestResultsMap[backtestKey].Summary.Columns
			// Ensure "ticker" is the first column
			tickerIdx := -1
			timestampIdx := -1
			for i, col := range backtestColumns {
				if col == "ticker" {
					tickerIdx = i
				}
				if col == "timestamp" {
					timestampIdx = i
				}
			}
			// If both ticker and timestamp exist, merge them into ticker and remove timestamp
			if tickerIdx >= 0 && timestampIdx >= 0 {
				// Remove timestamp from columns
				newColumns := make([]string, 0, len(backtestColumns)-1)
				for _, col := range backtestColumns {
					if col != "timestamp" {
						newColumns = append(newColumns, col)
					}
				}
				backtestColumns = newColumns
			}
			// Recompute tickerIdx after possible column change
			tickerIdx = -1
			for i, col := range backtestColumns {
				if col == "ticker" {
					tickerIdx = i
					break
				}
			}
			if tickerIdx > 0 {
				// Move "ticker" to the front
				backtestColumns = append([]string{"ticker"}, append(backtestColumns[:tickerIdx], backtestColumns[tickerIdx+1:]...)...)
			}
			// Get first 100 instances, or all if less than 100
			instances := backtestInstances
			/*if len(backtestInstances) > 100 {
				instances = backtestInstances[:100]
			}*/
			// flatten instances into rows using original columns
			rows := make([]map[string]any, 0, len(instances))
			for _, instance := range instances {
				row := make(map[string]any)
				// If both ticker and timestamp exist, merge them
				if tickerIdx >= 0 && timestampIdx >= 0 {
					tickerVal, okTicker := instance.Instance["ticker"].(string)
					timestampVal, okTimestamp := instance.Instance["timestamp"]
					var timestampMs string
					if okTimestamp {
						switch v := timestampVal.(type) {
						case int64:
							timestampMs = fmt.Sprintf("%d", v*1000)
						case int:
							timestampMs = fmt.Sprintf("%d", v*1000)
						case float64:
							timestampMs = fmt.Sprintf("%d", int64(v*1000))
						case float32:
							timestampMs = fmt.Sprintf("%d", int64(v*1000))
						case string:
							// Try to parse string to int
							var tsInt int64
							_, err := fmt.Sscanf(v, "%d", &tsInt)
							if err == nil {
								timestampMs = fmt.Sprintf("%d", tsInt*1000)
							} else {
								timestampMs = v
							}
						default:
							timestampMs = ""
						}
					}
					if okTicker && timestampMs != "" {
						row["ticker"] = "$$" + tickerVal + "-" + timestampMs + "$$"
					} else if okTicker {
						row["ticker"] = "$$" + tickerVal + "$$"
					}
					// Copy other columns except timestamp
					for _, column := range backtestColumns {
						if column == "ticker" {
							continue
						}
						if column == "timestamp" {
							continue
						}
						row[column] = instance.Instance[column]
					}
				} else {
					for _, column := range backtestColumns {
						row[column] = instance.Instance[column]
					}
				}
				rows = append(rows, row)
			}
			// Now, create display columns for headers and output
			displayColumns := make([]string, len(backtestColumns))
			c := cases.Title(language.Und)
			for i, column := range backtestColumns {
				words := strings.Split(column, "_")
				for j, word := range words {
					words[j] = c.String(strings.TrimSpace(word))
				}
				displayColumns[i] = strings.Join(words, " ")
			}
			// Remap row keys to display columns
			for i, row := range rows {
				newRow := make(map[string]any)
				for j, column := range backtestColumns {
					displayCol := displayColumns[j]
					newRow[displayCol] = row[column]
				}
				rows[i] = newRow
			}
			// Convert rows to [][]any for table output
			finalRows := make([][]any, len(rows))
			for i, row := range rows {
				valueRow := make([]any, len(displayColumns))
				for j, displayCol := range displayColumns {
					valueRow[j] = row[displayCol]
				}
				finalRows[i] = valueRow
			}
			chunk.Type = "table"
			chunk.Content = map[string]any{
				"strategyID": chunkContent.StrategyID,
				"caption":    chunkContent.Caption,
				"headers":    displayColumns,
				"rows":       finalRows,
			}
			processedChunks = append(processedChunks, chunk)
		} else if chunk.Type == "backtest_plot" {
			var chunkContent BacktestPlotChunkData
			contentBytes, err := json.Marshal(chunk.Content)
			if err != nil {
				processedChunks = append(processedChunks, ContentChunk{
					Type:    "text",
					Content: "[Internal Error: Could not marshal backtest plot chunk content]",
				})
				continue
			}
			if err := json.Unmarshal(contentBytes, &chunkContent); err != nil {
				processedChunks = append(processedChunks, ContentChunk{
					Type:    "text",
					Content: "[Internal Error: Invalid backtest plot chunk format]",
				})
				continue
			}
			strategyID := chunkContent.StrategyID
			plotID := chunkContent.PlotID
			backtestKey := getBacktestKey(strategyID, chunkContent.Version)
			if backtestResultsMap[backtestKey] == nil {
				backtestResultsMap[backtestKey], err = strategy.GetBacktestFromCache(ctx, conn, userID, strategyID, chunkContent.Version)
				if err != nil {
					processedChunks = append(processedChunks, ContentChunk{
						Type:    "text",
						Content: fmt.Sprintf("[Internal Error: Could not get backtest results: %v]", err),
					})
					continue
				}
			}
			strategyPlots := backtestResultsMap[backtestKey].StrategyPlots
			for _, plot := range strategyPlots {
				if plot.PlotID == plotID {
					var titleIcon string
					if plot.TitleTicker != "" {
						titleIcon, _ = helpers.GetIcon(conn, plot.TitleTicker)
					}
					processedChunks = append(processedChunks, ContentChunk{
						Type: "plot",
						Content: map[string]any{
							"chart_type": plot.ChartType,
							"data":       plot.Data,
							"title":      plot.Title,
							"titleIcon":  titleIcon,
							"layout":     plot.Layout,
						},
					})
					break
				}
			}
		} else if chunk.Type == "agent_plot" {
			var chunkContent AgentPlotChunkData
			contentBytes, err := json.Marshal(chunk.Content)
			if err != nil {
				processedChunks = append(processedChunks, ContentChunk{
					Type:    "text",
					Content: "[Internal Error: Could not marshal agent plot chunk content]",
				})
				continue
			}
			if err := json.Unmarshal(contentBytes, &chunkContent); err != nil {
				processedChunks = append(processedChunks, ContentChunk{
					Type:    "text",
					Content: "[Internal Error: Invalid agent plot chunk format]",
				})
				continue
			}
			executionID := chunkContent.ExecutionID
			plotID := chunkContent.PlotID
			if agentResultsMap[executionID] == nil {
				agentResultsMap[executionID], err = GetPythonAgentResultFromCache(ctx, conn, executionID)
				if err != nil {
					processedChunks = append(processedChunks, ContentChunk{
						Type:    "text",
						Content: fmt.Sprintf("[Internal Error: Could not get agent results: %v]", err),
					})
					continue
				}
			}
			agentPlots := agentResultsMap[executionID].Plots
			for _, plot := range agentPlots {
				if plot.PlotID == plotID {
					var titleIcon string
					if plot.TitleTicker != "" {
						titleIcon, _ = helpers.GetIcon(conn, plot.TitleTicker)
					}
					processedChunks = append(processedChunks, ContentChunk{
						Type: "plot",
						Content: map[string]any{
							"chart_type": plot.ChartType,
							"data":       plot.Data,
							"title":      plot.Title,
							"titleIcon":  titleIcon,
							"layout":     plot.Layout,
						},
					})
					break
				}
			}
		} else if chunk.Type == "plot" {
			// Handle titleTicker for plot chunks
			if contentMap, ok := chunk.Content.(map[string]any); ok {
				var titleIcon string
				if titleTicker, exists := contentMap["titleTicker"].(string); exists && titleTicker != "" {
					titleIcon, _ = helpers.GetIcon(conn, titleTicker)
				}

				// Create new content with titleIcon added
				newContent := make(map[string]any)
				for k, v := range contentMap {
					newContent[k] = v
				}
				newContent["titleIcon"] = titleIcon

				processedChunks = append(processedChunks, ContentChunk{
					Type:    chunk.Type,
					Content: newContent,
				})
			} else {
				// If content is not a map, keep the original chunk
				processedChunks = append(processedChunks, chunk)
			}
		} else {
			// Keep non-instruction chunks as they are
			processedChunks = append(processedChunks, chunk)
		}
	}
	return processedChunks
}

// formatStatusMessage replaces placeholders like {key} with values from the args map.
func formatStatusMessage(message string, argsMap map[string]interface{}) string {
	re := regexp.MustCompile(`{([^}]+)}`)
	formattedMessage := re.ReplaceAllStringFunc(message, func(match string) string {
		key := match[1 : len(match)-1] // Extract key from {key}
		if val, ok := argsMap[key]; ok {
			return fmt.Sprintf("%v", val) // Convert value to string
		}
		return match // Return original placeholder if key not found
	})
	return formattedMessage
}

func cleanStatusMessage(conn *data.Conn, message string) string {
	client := conn.OpenAIClient
	messages := []responses.ResponseInputItemUnionParam{}
	messages = append(messages, responses.ResponseInputItemUnionParam{
		OfMessage: &responses.EasyInputMessageParam{
			Role: responses.EasyInputMessageRoleUser,
			Content: responses.EasyInputMessageContentUnionParam{
				OfString: openai.String(message),
			},
		},
	})
	instructions := getCleanThinkingTracePrompt()
	res, err := client.Responses.New(context.Background(), responses.ResponseNewParams{
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: messages,
		},
		Model:        "gpt-4.1-nano",
		Instructions: openai.String(instructions),
	})
	if err != nil {
		return ""
	}
	return res.OutputText()
}

// </chat.go>
