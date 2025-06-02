package agent

import (
	"backend/internal/data"
	"backend/internal/services/socket"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"google.golang.org/genai"
)

type Query struct {
	Query              string                   `json:"query"`
	Context            []map[string]interface{} `json:"context,omitempty"`
	ActiveChartContext map[string]interface{}   `json:"activeChartContext,omitempty"`
}

type QueryResponse struct {
	Type          string         `json:"type"` //"mixed_content", "function_calls", "simple_text"
	ContentChunks []ContentChunk `json:"content_chunks,omitempty"`
	Text          string         `json:"text,omitempty"`
	Citations     []Citation     `json:"citations,omitempty"`
	Suggestions   []string       `json:"suggestions,omitempty"`
}

// ThinkingResponse represents the JSON output from the thinking model with rounds
type ThinkingResponse struct {
	Rounds                  [][]FunctionCall `json:"rounds"`
	RequiresFurtherPlanning bool             `json:"requires_further_planning"`
	RequiresFinalResponse   bool             `json:"requires_final_response"`
	ContentChunks           []ContentChunk   `json:"content_chunks,omitempty"`
	PlanningContext         json.RawMessage  `json:"planning_context,omitempty"`
}

const initialQueryModel = "gemini-2.5-flash-preview-04-17"
const thinkingModel = "gemini-2.0-flash-thinking-exp-01-21"

// buildContextPrompt formats incoming chart/filing context for the model
func buildContextPrompt(contextItems []map[string]interface{}) string {
	var sb strings.Builder
	for _, item := range contextItems {
		// Treat filing contexts first
		if _, ok := item["link"]; ok {
			ticker, _ := item["ticker"].(string)
			fType, _ := item["filingType"].(string)
			link, _ := item["link"].(string)
			sb.WriteString(fmt.Sprintf("Filing - Ticker: %s, Type: %s, Link: %s\n", ticker, fType, link))
		} else if _, ok := item["timestamp"]; ok {
			// Then treat instance contexts
			ticker, _ := item["ticker"].(string)
			secID := fmt.Sprint(item["securityId"])
			tsStr := fmt.Sprint(item["timestamp"])
			sb.WriteString(fmt.Sprintf("Instance - Ticker: %s, SecurityId: %s, TimestampMs: %s\n", ticker, secID, tsStr))
		}
	}
	return sb.String()
}

// GetQuery processes a natural language query and returns the result
func GetQuery(conn *data.Conn, userID int, args json.RawMessage) (interface{}, error) {

	// Use the standardized Redis connectivity test
	ctx := context.Background()
	success, message := conn.TestRedisConnectivity(ctx, userID)
	if !success {
		return nil, fmt.Errorf("%s", message)
		////fmt.Printf("WARNING: %s\n", message)
	}
	//else {
	////fmt.Println(message)
	//}

	var query Query
	if err := json.Unmarshal(args, &query); err != nil {
		return nil, fmt.Errorf("error parsing request: %w", err)
	}

	// Build context prompt section
	contextSection := buildContextPrompt(query.Context)
	// If userQuery is empty, error
	userQuery := query.Query
	if userQuery == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	// Check for existing conversation history
	conversationKey := fmt.Sprintf("user:%d:conversation", userID)
	conversationData, err := GetConversationFromCache(ctx, conn, userID)
	// If we have existing conversation data, append the new message
	////fmt.Println("Accessing conversation for key:", conversationKey)
	// If no conversation exists, create a new one
	if err != nil || conversationData == nil {
		////fmt.Printf("Creating new conversation. Error: %v\n", err)
		conversationData = &ConversationData{
			Messages:  []ChatMessage{},
			Timestamp: time.Now(),
		}
	}
	persistentContextData, err := getPersistentContext(ctx, conn, userID)
	if err != nil {
		// Log the error but don't fail the request, just proceed without this context
		////fmt.Printf("Warning: Failed to retrieve persistent context: %v\n", err)
		persistentContextData = &PersistentContextData{Items: make(map[string]PersistentContextItem)}
	}

	// Get function calls from the LLM with context from previous messages
	var conversationHistory string
	var persistentHistory string
	var allResults []ExecuteResult
	var allThinkingResults []ThinkingResponse
	if len(conversationData.Messages) > 0 {
		// Build context from previous messages for Gemini
		conversationHistory = _buildConversationContext(conversationData.Messages)
	} else {
		conversationHistory = ""
	}
	if len(persistentContextData.Items) > 0 {
		persistentHistory = buildPersistentHistory(persistentContextData)
	} else {
		persistentHistory = ""
	}
	maxTurns := 5
	numTurns := 0
	for numTurns < maxTurns {
		var prompt strings.Builder

		if persistentHistory != "" {
			prompt.WriteString("<PersistentContext>\n")
			prompt.WriteString(persistentHistory)
			prompt.WriteString("</PersistentContext>\n\n")
			////fmt.Println("persistentHistory ", persistentHistory)
		}
		if conversationHistory != "" {
			prompt.WriteString("<ConversationHistory>\n")
			prompt.WriteString(conversationHistory)
			prompt.WriteString("</ConversationHistory>\n\n")
		}
		// Add active chart context if present
		if query.ActiveChartContext != nil {
			ticker, _ := query.ActiveChartContext["ticker"].(string)
			secID := fmt.Sprint(query.ActiveChartContext["securityId"])
			tsStr := fmt.Sprint(query.ActiveChartContext["timestamp"])
			prompt.WriteString("<UserActiveChart>\n")
			prompt.WriteString(fmt.Sprintf("- Ticker: %s, SecurityId: %s, TimestampMs: %s\n", ticker, secID, tsStr))
			prompt.WriteString("</UserActiveChart>\n\n")
		}
		if contextSection != "" {
			prompt.WriteString("<UserContext>\n")
			prompt.WriteString(contextSection)
			prompt.WriteString("</UserContext>\n\n")
		}
		prompt.WriteString("<UserQuery>\n")
		prompt.WriteString(userQuery)
		prompt.WriteString("\n</UserQuery>\n\n")
		if len(allThinkingResults) > 0 {
			prompt.WriteString("<PreviousRoundResults>\n")
			resultsJSON, _ := json.Marshal(allResults)
			prompt.WriteString("```json\n")
			prompt.WriteString(string(resultsJSON))
			prompt.WriteString("\n```\n")
			prompt.WriteString("</PreviousRoundResults>\n\n")
		}
		////fmt.Println("prompt ", prompt.String())
		// This first passes the query to a thinking model
		geminiThinkingResponse, err := getGeminiFunctionThinking(ctx, conn, "defaultSystemPrompt", prompt.String(), initialQueryModel)
		if err != nil {
			return nil, fmt.Errorf("error getting thinking response: %w", err)
		}

		responseText := geminiThinkingResponse.Text
		// Try to parse the thinking response as JSON
		citations := geminiThinkingResponse.Citations
		var thinkingResp ThinkingResponse
		// Find the JSON block in the response
		jsonStartIdx := strings.Index(responseText, "{")
		jsonEndIdx := strings.LastIndex(responseText, "}")

		// If no valid JSON is found, just return the text response
		if jsonStartIdx == -1 || jsonEndIdx == -1 || jsonEndIdx <= jsonStartIdx {
			return QueryResponse{
				Type: "text",
				Text: responseText,
			}, nil
		}

		jsonBlock := responseText[jsonStartIdx : jsonEndIdx+1]
		_ = json.Unmarshal([]byte(jsonBlock), &thinkingResp) // Ignore error for now, as the block was empty
		////fmt.Println("thinking response ", thinkingResp)
		if len(thinkingResp.Rounds) == 0 && len(thinkingResp.ContentChunks) == 0 {
			newMessage := ChatMessage{
				Query:            query.Query,
				ResponseText:     responseText,
				FunctionCalls:    []FunctionCall{},
				ToolResults:      []ExecuteResult{},
				ContextItems:     query.Context, // Store context with the user query message
				SuggestedQueries: []string{},    // No suggestions for this response type
				Timestamp:        time.Now(),
				ExpiresAt:        time.Now().Add(24 * time.Hour),
				Citations:        citations,
			}

			// Add new message to conversation history
			conversationData.Messages = append(conversationData.Messages, newMessage)
			conversationData.Timestamp = time.Now()
			if err := saveConversationToCache(ctx, conn, userID, conversationKey, conversationData); err != nil {
				return nil, err
				////fmt.Printf("Error saving updated conversation: %v\n", err)
			}

			return QueryResponse{
				Type: "text",
				Text: responseText,
			}, nil
		}

		// If we have content chunks directly in the response, return them
		if len(thinkingResp.ContentChunks) > 0 {

			// Process potential table instructions *before* saving/returning
			processedInitialChunks := processContentChunksForTables(ctx, conn, userID, thinkingResp.ContentChunks)

			newMessage := ChatMessage{
				Query:            query.Query,
				ContentChunks:    processedInitialChunks, // Use processed chunks
				FunctionCalls:    []FunctionCall{},
				ToolResults:      []ExecuteResult{},
				ContextItems:     query.Context, // Store context with the user query message
				SuggestedQueries: []string{},    // No suggestions for this response type
				Timestamp:        time.Now(),
				ExpiresAt:        time.Now().Add(24 * time.Hour),
				Citations:        citations,
			}

			// Add new message to conversation history
			conversationData.Messages = append(conversationData.Messages, newMessage)
			conversationData.Timestamp = time.Now()
			if err := saveConversationToCache(ctx, conn, userID, conversationKey, conversationData); err != nil {
				return nil, err
				////fmt.Printf("Error saving updated conversation: %v\n", err)
			}

			// Return both the original ContentChunks for the frontend to render
			// and the text version for systems that can't handle structured content
			return QueryResponse{
				Type:          "mixed_content",
				ContentChunks: processedInitialChunks, // Return processed chunks
				Citations:     citations,
			}, nil
		}

		// Try to process the thinking response as rounds
		contentChunks, thinkingResults, err := processThinkingResponse(ctx, conn, userID, thinkingResp, query.Query)
		if err == nil && len(thinkingResults) > 0 {
			if len(contentChunks) > 0 {
				// Create new message with the content chunks response
				newMessage := ChatMessage{
					Query:            query.Query,
					ContentChunks:    contentChunks,
					FunctionCalls:    []FunctionCall{},
					ToolResults:      []ExecuteResult{},
					ContextItems:     query.Context, // Store context with the user query message
					SuggestedQueries: []string{},    // No suggestions for this response type
					Timestamp:        time.Now(),
					ExpiresAt:        time.Now().Add(24 * time.Hour),
				}
				conversationData.Messages = append(conversationData.Messages, newMessage)
				conversationData.Timestamp = time.Now()
				if err := saveConversationToCache(ctx, conn, userID, conversationKey, conversationData); err != nil {
					return nil, err
					////fmt.Printf("Error saving updated conversation: %v\n", err)
				}

				return QueryResponse{
					Type:          "mixed_content",
					ContentChunks: newMessage.ContentChunks,
				}, nil
			}
			if !thinkingResp.RequiresFurtherPlanning {
				_, callErr := GetQuery(conn, userID, json.RawMessage(fmt.Sprintf(`{"query": "%s"}`, userQuery)))
				if callErr != nil {
					log.Printf("Error in GetQuery call: %v", callErr)
				}
			}
			allResults = append(allResults, thinkingResults...)
			allThinkingResults = append(allThinkingResults, thinkingResp)
		}

		numTurns++
	}
	return nil, fmt.Errorf("error getting gemini function response: %w", err)
}

func buildPersistentHistory(persistentContextData *PersistentContextData) string {
	var persistentHistory strings.Builder
	if persistentContextData != nil && len(persistentContextData.Items) > 0 {
		keys := make([]string, 0, len(persistentContextData.Items))
		for k := range persistentContextData.Items {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			item := persistentContextData.Items[key]
			// Attempt to pretty-print the JSON value
			var prettyValue string
			var structuredValue interface{}
			if err := json.Unmarshal(item.Value, &structuredValue); err == nil {
				prettyBytes, err := json.MarshalIndent(structuredValue, "", "  ")
				if err == nil {
					prettyValue = string(prettyBytes)
				} else {
					prettyValue = string(item.Value) // Fallback to raw JSON string
				}
			} else {
				prettyValue = string(item.Value) // Fallback if not valid JSON
			}

			persistentHistory.WriteString(fmt.Sprintf("- Key: %s (Last Updated: %s)\n",
				item.Key, item.Timestamp.Format(time.RFC1123)))
			persistentHistory.WriteString(fmt.Sprintf("  Value:\n```json\n%s\n```\n", prettyValue))
		}
		persistentHistory.WriteString("\n") // Add separation
	}
	return persistentHistory.String()
}

func getGeminiResponse(ctx context.Context, conn *data.Conn, query string) (string, error) {
	apiKey, err := conn.GetGeminiKey()
	if err != nil {
		return "", fmt.Errorf("error getting gemini key: %w", err)
	}

	// Create a new client using the API key
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", fmt.Errorf("error creating gemini client: %w", err)
	}

	systemInstruction, err := getSystemInstruction("finalResponseSystemPrompt")
	if err != nil {
		return "", fmt.Errorf("error getting system instruction: %w", err)
	}

	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: systemInstruction},
			},
		},
	}
	result, err := client.Models.GenerateContent(ctx, "gemini-2.0-flash-001", genai.Text(query), config)
	if err != nil {
		return "", fmt.Errorf("error generating content: %w", err)
	}

	// Extract the response text
	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response from Gemini")
	}

	// Get the text from the response
	text := fmt.Sprintf("%v", result.Candidates[0].Content.Parts[0].Text)
	return text, nil
}

// RoundResult stores the results of a round's function calls
type RoundResult struct {
	Results map[string]interface{} `json:"results"`
}

// processThinkingResponse attempts to parse and execute the thinking model's rounds
// and modifies the final response string *before* parsing into chunks.
func processThinkingResponse(ctx context.Context, conn *data.Conn, userID int, thinkingResp ThinkingResponse, originalQuery string) ([]ContentChunk, []ExecuteResult, error) {

	var finalContentChunks []ContentChunk
	var allResults []ExecuteResult

	// Check if this is an immediate content_chunks response (no rounds executed)
	if len(thinkingResp.Rounds) == 0 && len(thinkingResp.ContentChunks) > 0 {
		finalContentChunks = thinkingResp.ContentChunks
		// Fall through to process table instructions
	} else {
		// --- Round Execution Logic --- (Execute rounds if present)
		var allPreviousRoundResults []ExecuteResult
		for _, round := range thinkingResp.Rounds {
			// ... (existing round processing logic: build prompt, call Gemini, execute functions) ...

			// First, convert the round to JSON
			roundJSON, err := json.Marshal(round)
			if err != nil {
				////fmt.Printf("Error marshaling round to JSON: %v\n", err)
				continue
			}

			// Create a prompt that includes the round and previous results
			var prompt strings.Builder
			prompt.WriteString("Process this round of function calls:\n\n")
			prompt.WriteString("```json\n")
			prompt.WriteString(string(roundJSON))
			prompt.WriteString("\n```\n")

			// Include ALL previous round results if available
			if len(allPreviousRoundResults) > 0 {
				prompt.WriteString("<PreviousRoundResults>\n")
				resultsJSON, _ := json.Marshal(allPreviousRoundResults)
				prompt.WriteString("```json\n")
				prompt.WriteString(string(resultsJSON))
				prompt.WriteString("\n```\n")
				prompt.WriteString("</PreviousRoundResults>\n")
			}

			prompt.WriteString("Please process this round of function calls.\n")
			// Send to Gemini for processing
			////fmt.Printf("Sending round to Gemini for processing:\n%s\n", prompt.String())
			processedRound, err := processRoundWithGemini(ctx, conn, prompt.String())
			if err != nil {
				////fmt.Printf("Error processing round with Gemini: %v\n", err)

				continue
			}

			// Execute the functions returned by Gemini
			roundResults, err := executeGeminiFunctionCalls(ctx, conn, userID, processedRound)
			if err != nil {
				////fmt.Printf("Error executing functions: %v\n", err)
				continue
			}

			// Add this round's results to the combined results
			allResults = append(allResults, roundResults...)
			// Accumulate results for the next round
			allPreviousRoundResults = append(allPreviousRoundResults, roundResults...)
		}
		if thinkingResp.RequiresFurtherPlanning {
			return []ContentChunk{}, allResults, nil // Return intermediate results
		}

		if thinkingResp.RequiresFinalResponse {
			// 1. Generate the final response text using accumulated results
			var finalPrompt strings.Builder
			finalPrompt.WriteString("<OriginalUserQuery>")
			finalPrompt.WriteString(originalQuery)
			finalPrompt.WriteString("</OriginalUserQuery>\n")
			finalPrompt.WriteString("<FunctionCallResults>")
			resultsJSON, _ := json.Marshal(allResults)
			finalPrompt.WriteString(string(resultsJSON))
			finalPrompt.WriteString("\n</FunctionCallResults>\n")
			finalPrompt.WriteString("Please provide a final response to the original query based on the results from the function calls.")

			processedText, err := getGeminiResponse(ctx, conn, finalPrompt.String())
			if err != nil {
				return nil, nil, fmt.Errorf("error getting final gemini response: %w", err)
			}
			processedText = strings.TrimSpace(processedText)
			////fmt.Printf("Raw final response text from Gemini:\\n%s\\n", processedText)

			// 2. Parse the text into ContentChunks
			var contentChunksResponse struct {
				ContentChunks []ContentChunk `json:"content_chunks"`
			}

			// Print the response before parsing
			////fmt.Printf("Raw LLM text before parsing content chunks:\\n---\\n%s\\n---\\n", processedText)
			if err := json.Unmarshal([]byte(processedText), &contentChunksResponse); err == nil && len(contentChunksResponse.ContentChunks) > 0 {
				finalContentChunks = contentChunksResponse.ContentChunks
				// Fall through to process table instructions
			} else {
				// Try finding a JSON block within the text
				jsonStartIdx := strings.Index(processedText, "{")
				jsonEndIdx := strings.LastIndex(processedText, "}")
				if jsonStartIdx != -1 && jsonEndIdx != -1 && jsonEndIdx > jsonStartIdx {
					jsonBlock := processedText[jsonStartIdx : jsonEndIdx+1]
					if err := json.Unmarshal([]byte(jsonBlock), &contentChunksResponse); err == nil && len(contentChunksResponse.ContentChunks) > 0 {
						finalContentChunks = contentChunksResponse.ContentChunks
						// Fall through to process table instructions
					} else {
						// Fallback: Treat the text as a single text chunk
						finalContentChunks = []ContentChunk{{Type: "text", Content: processedText}}
					}
				} else {
					// Fallback: Treat the text as a single text chunk
					finalContentChunks = []ContentChunk{{Type: "text", Content: processedText}}
				}
			}
		} else {
			// If no final response is needed (but maybe rounds were run)
			// We might still have tool results to return, but no content chunks yet.
			finalContentChunks = []ContentChunk{}
		}
	}
	processedChunks := processContentChunksForTables(ctx, conn, userID, finalContentChunks)

	return processedChunks, allResults, nil
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

// processRoundWithGemini sends a round to Gemini for processing and gets back the functions to execute
func processRoundWithGemini(ctx context.Context, conn *data.Conn, prompt string) ([]FunctionCall, error) {
	// Get a response from Gemini with the processed functions
	response, err := getGeminiFunctionResponse(ctx, conn, prompt)
	if err != nil {
		return nil, fmt.Errorf("error getting function response from Gemini: %w", err)
	}

	// Return the function calls from the response
	return response.FunctionCalls, nil
}

// executeGeminiFunctionCalls executes the function calls returned by Gemini
func executeGeminiFunctionCalls(_ context.Context, conn *data.Conn, userID int, functionCalls []FunctionCall) ([]ExecuteResult, error) {
	var results []ExecuteResult

	for _, fc := range functionCalls {
		////fmt.Printf("Executing function %s with args: %s\n", fc.Name, string(fc.Args))

		// Parse arguments into a map for storage and formatting
		var argsMap map[string]interface{}
		if err := json.Unmarshal(fc.Args, &argsMap); err != nil {
			// If unmarshalling into a map fails, try interface{} for logging purposes
			var argsForLog interface{}
			_ = json.Unmarshal(fc.Args, &argsForLog) // Ignore error here
			////fmt.Printf("Warning: Could not parse args into map for function %s: %v. Args: %s\n", fc.Name, err, string(fc.Args))
			argsMap = make(map[string]interface{}) // Use an empty map if parsing failed
		} else {
			// If parsing succeeds, also keep the raw interface{} form for ExecuteResult
			var argsForLog interface{}
			_ = json.Unmarshal(fc.Args, &argsForLog)
		}

		// Check if the function exists in Tools map
		tool, exists := Tools[fc.Name]
		if !exists {
			results = append(results, ExecuteResult{
				FunctionName: fc.Name,
				Error:        fmt.Sprintf("function '%s' not found", fc.Name),
				Args:         argsMap, // Log the parsed map if available
			})
			continue
		}

		// ---> Format and send status update to the client <---
		formattedMsg := formatStatusMessage(tool.StatusMessage, argsMap)
		socket.SendFunctionStatus(userID, formattedMsg)

		// Execute the function
		result, err := tool.Function(conn, userID, fc.Args)
		if err != nil {
			////fmt.Printf("Function %s execution error: %v\n", fc.Name, err)
			results = append(results, ExecuteResult{
				FunctionName: fc.Name,
				Error:        err.Error(),
				Args:         argsMap, // Log the parsed map
			})
		} else {
			////fmt.Printf("Function %s executed successfully\n", fc.Name)
			results = append(results, ExecuteResult{
				FunctionName: fc.Name,
				Result:       result,
				Args:         argsMap, // Log the parsed map
			})
		}
	}

	return results, nil
}
