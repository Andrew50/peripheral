// <chat.go>
package agent

import (
	"backend/internal/app/helpers"
	"backend/internal/app/limits"
	"backend/internal/app/strategy"
	"backend/internal/data"
	"backend/internal/services/socket"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
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
		return QueryResponse{
			ContentChunks: []ContentChunk{{
				Type:    "text",
				Content: "You have reached your query limit. Please add more credits to your account to continue.",
			}},
			Suggestions:    []string{"View Pricing Plans", "Add Credits"},
			ConversationID: query.ConversationID,
			MessageID:      "", // Will be generated
			Timestamp:      time.Now(),
		}, nil
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
	socket.SendChatInitializationUpdate(userID, messageID, conversationID)
	// ----- Acquire per-user chat lock ------------------------------------
	lockKey := fmt.Sprintf("chat_lock:%d", userID)
	locked, lockErr := conn.Cache.SetNX(ctx, lockKey, "1", 1*time.Minute).Result() // 1 min for testing lol
	if lockErr != nil {
		return nil, fmt.Errorf("error acquiring chat lock: %v", lockErr)
	}
	if !locked {
		// User already has an active chat
		return QueryResponse{
			ContentChunks: []ContentChunk{{
				Type:    "text",
				Content: "You already have a chat in progress. Please wait for it to finish before starting a new one.",
			}},
			ConversationID: conversationID,
			MessageID:      messageID,
			Timestamp:      time.Now(),
		}, nil
	}
	// Ensure lock is released
	defer func() {
		_ = conn.Cache.Del(context.Background(), lockKey).Err()
	}()

	// no credit deducted yet – will deduct on success

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
		var firstRound bool
		var result interface{}
		var err error
		if planningPrompt == "" {
			firstRound = true
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
			result, err = RunPlanner(ctx, conn, conversationID, userID, planningPrompt, firstRound, activeResults, accumulatedThoughts)
		} else {
			result, err = RunPlanner(ctx, conn, conversationID, userID, planningPrompt, firstRound, activeResults, accumulatedThoughts)
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
					executor = NewExecutor(conn, userID, 5, logger)
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
				// Generate final response based on active execution results
				var finalResponse *FinalResponse

				// Get the final response from the model
				finalResponse, err = GetFinalResponseGPT(ctx, conn, userID, query.Query, conversationID, activeResults, accumulatedThoughts)
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

				// Process any table instructions in the content chunks for frontend viewing for backtest table and backtest plot chunks
				processedChunks := processContentChunksForFrontend(ctx, conn, userID, finalResponse.ContentChunks)
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

// TableInstructionData holds the parameters for generating a table from cached data
type BacktestTableChunkData struct {
	StrategyID int         `json:"strategyID"`        // strategyId
	Columns    interface{} `json:"columns"`           // Internal column names as either []string or string
	Caption    string      `json:"caption"`           // Table title
	NumRows    int         `json:"numRows,omitempty"` // Number of rows in the table
}
type BacktestPlotChunkData struct {
	StrategyID int    `json:"strategyID"`
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

func processContentChunksForDB(ctx context.Context, conn *data.Conn, userID int, inputChunks []ContentChunk) []ContentChunk {
	processedChunks := make([]ContentChunk, 0, len(inputChunks))
	var backtestResultsMap = make(map[int]*strategy.BacktestResponse)
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
			if backtestResultsMap[backtestTableChunkContent.StrategyID] == nil {
				backtestResultsMap[backtestTableChunkContent.StrategyID], err = strategy.GetBacktestFromCache(ctx, conn, userID, backtestTableChunkContent.StrategyID)
				if err != nil {
					continue
				}
			}
			if backtestTableChunkContent.Columns == "all" {
				// Store all columns from the backtest results
				backtestTableChunkContent.Columns = backtestResultsMap[backtestTableChunkContent.StrategyID].Summary.Columns
				chunk.Content = backtestTableChunkContent
			} else {
				// store only columns specified
				chunk.Content = backtestTableChunkContent
			}
			backtestTableChunkContent.NumRows = len(backtestResultsMap[backtestTableChunkContent.StrategyID].Instances)

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
			if backtestResultsMap[backtestPlotChunkContent.StrategyID] == nil {
				backtestResultsMap[backtestPlotChunkContent.StrategyID], err = strategy.GetBacktestFromCache(ctx, conn, userID, backtestPlotChunkContent.StrategyID)
				if err != nil {
					continue
				}
			}
			strategyPlots := backtestResultsMap[backtestPlotChunkContent.StrategyID].StrategyPlots
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
	var backtestResultsMap = make(map[int]*strategy.BacktestResponse)
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
			if backtestResultsMap[chunkContent.StrategyID] == nil {
				backtestResultsMap[chunkContent.StrategyID], err = strategy.GetBacktestFromCache(ctx, conn, userID, chunkContent.StrategyID)
				if err != nil {
					processedChunks = append(processedChunks, ContentChunk{
						Type:    "text",
						Content: fmt.Sprintf("[Internal Error: Could not get backtest results: %v]", err),
					})
					continue
				}
			}
			backtestInstances := backtestResultsMap[chunkContent.StrategyID].Instances
			backtestColumns := backtestResultsMap[chunkContent.StrategyID].Summary.Columns
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
			if backtestResultsMap[strategyID] == nil {
				backtestResultsMap[strategyID], err = strategy.GetBacktestFromCache(ctx, conn, userID, strategyID)
				if err != nil {
					processedChunks = append(processedChunks, ContentChunk{
						Type:    "text",
						Content: fmt.Sprintf("[Internal Error: Could not get backtest results: %v]", err),
					})
					continue
				}
			}
			strategyPlots := backtestResultsMap[strategyID].StrategyPlots
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

// </chat.go>
