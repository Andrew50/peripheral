// <prompt.go>
package agent

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

func BuildPlanningPrompt(conn *data.Conn, userID int, query string, contextItems []map[string]interface{}, activeChartContext map[string]interface{}, includeUserChart bool) (string, error) {
	ctx := context.Background()
	var sb strings.Builder

	// Get the active conversation using the new system
	activeConversationID, err := GetActiveConversationIDCached(ctx, conn, userID)
	if err == nil && activeConversationID != "" {
		// Load conversation messages from database
		messagesInterface, err := GetConversationMessages(ctx, conn, activeConversationID, userID)
		if err == nil && messagesInterface != nil {
			// Type assert to get the actual messages
			if dbMessages, ok := messagesInterface.([]DBConversationMessage); ok && len(dbMessages) > 0 {
				// Convert DB messages to ChatMessage format for context building
				chatMessages := make([]ChatMessage, len(dbMessages))
				for i, msg := range dbMessages {
					chatMessages[i] = ChatMessage{
						Query:            msg.Query,
						ContentChunks:    msg.ContentChunks,
						ResponseText:     msg.ResponseText,
						FunctionCalls:    msg.FunctionCalls,
						ToolResults:      msg.ToolResults,
						ContextItems:     msg.ContextItems,
						SuggestedQueries: msg.SuggestedQueries,
						Citations:        msg.Citations,
						Timestamp:        msg.CreatedAt,
						TokenCount:       msg.TokenCount,
						Status:           msg.Status,
					}
					if msg.CompletedAt != nil {
						chatMessages[i].CompletedAt = *msg.CompletedAt
					}
				}

				conversationContext := _buildConversationContext(chatMessages)
				if conversationContext != "" {
					sb.WriteString("<ConversationHistory>\n")
					sb.WriteString(conversationContext)
					sb.WriteString("\n</ConversationHistory>\n")
				}
			}
		}
	}

	if len(contextItems) > 0 {
		sb.WriteString("<ContextItems>\n")
		sb.WriteString(_buildContextItems(contextItems))
		sb.WriteString("\n</ContextItems>\n")
	}
	if activeChartContext != nil && includeUserChart {
		sb.WriteString("<UserChart>\n")
		ticker, _ := activeChartContext["ticker"].(string)
		secID := fmt.Sprint(activeChartContext["securityId"])
		tsStr := fmt.Sprint(activeChartContext["timestamp"])
		sb.WriteString(fmt.Sprintf("%s, SecurityId: %s, TimestampMs: %s", ticker, secID, tsStr))
		sb.WriteString("\n</UserChart>\n")
	}
	sb.WriteString("<UserQuery>\n")
	sb.WriteString(query)
	sb.WriteString("\n</UserQuery>\n")

	return sb.String(), nil
}

func BuildPlanningPromptWithResults(conn *data.Conn, userID int, query string, contextItems []map[string]interface{}, activeChartContext map[string]interface{}, results []ExecuteResult) (string, error) {
	// Start with the basic planning prompt
	sb := strings.Builder{}
	planningPrompt, err := BuildPlanningPrompt(conn, userID, query, contextItems, activeChartContext, false)
	if err != nil {
		return "", err
	}
	sb.WriteString(planningPrompt)

	// Add execution results
	if len(results) > 0 {
		sb.WriteString("\n<ExecutionResults>\n")
		resultsJSON, err := json.Marshal(results)
		if err != nil {
			sb.WriteString(fmt.Sprintf("Error marshaling results: %v\n", err))
		} else {
			sb.WriteString("```json\n")
			sb.WriteString(string(resultsJSON))
			sb.WriteString("\n```\n")
		}
		sb.WriteString("</ExecutionResults>\n")
	}

	return sb.String(), nil
}

func _buildContextItems(contextItems []map[string]interface{}) string {
	var context strings.Builder
	for _, item := range contextItems {
		// Treat filing contexts first
		if _, ok := item["link"]; ok {
			ticker, _ := item["ticker"].(string)
			fType, _ := item["filingType"].(string)
			link, _ := item["link"].(string)
			context.WriteString(fmt.Sprintf("SEC Filing - Ticker: %s, Type: %s, Link: %s\n", ticker, fType, link))
		} else if _, ok := item["timestamp"]; ok {
			// Then treat instance contexts
			ticker, _ := item["ticker"].(string)
			secID := fmt.Sprint(item["securityId"])
			tsStr := fmt.Sprint(item["timestamp"])
			context.WriteString(fmt.Sprintf("%s, SecurityId: %s, TimestampMs: %s\n", ticker, secID, tsStr))
		}
	}
	return context.String()
}

func _buildConversationContext(messages []ChatMessage) string {
	var context strings.Builder

	// Include up to last 10 messages for context
	startIdx := 0
	if len(messages) > 10 {
		startIdx = len(messages) - 10
	}
	if len(messages) == 0 {
		return ""
	}
	for i := startIdx; i < len(messages); i++ {
		// Skip pending messages to avoid empty Assistant responses
		if messages[i].Status == "pending" {
			continue
		}

		context.WriteString("User: ")
		context.WriteString(messages[i].Query)
		context.WriteString("\n")
		// Include context items if they exist for the user message
		if len(messages[i].ContextItems) > 0 {
			context.WriteString("User Context:\n")
			context.WriteString(_buildContextItems(messages[i].ContextItems)) // Reuse existing formatting function
			context.WriteString("\n")
		}

		context.WriteString("Assistant: ")
		if len(messages[i].ContentChunks) > 0 {
			for _, chunk := range messages[i].ContentChunks {
				// Safely handle different content types
				switch v := chunk.Content.(type) {
				case string:
					context.WriteString(v)
				case map[string]interface{}:
					// For table data or other structured content, convert to a simple text representation
					jsonData, err := json.Marshal(v)
					if err == nil {
						context.WriteString(fmt.Sprintf("[Table data: %s]", string(jsonData)))
					} else {
						context.WriteString("[Table data]")
					}
				default:
					// Handle any other type by converting to string
					context.WriteString(fmt.Sprintf("%v", v))
				}
			}
		} else {
			context.WriteString(messages[i].ResponseText)
		}
		context.WriteString("\n\n")
	}
	return context.String()
}

func BuildFinalResponsePrompt(conn *data.Conn, userID int, query string, contextItems []map[string]interface{}, activeChartContext map[string]interface{}, allResults []ExecuteResult) (string, error) {
	// Start with the basic planning prompt
	sb := strings.Builder{}
	planningPrompt, err := BuildPlanningPrompt(conn, userID, query, contextItems, activeChartContext, false)
	if err != nil {
		return "", err
	}
	sb.WriteString(planningPrompt)

	// Add execution results
	if len(allResults) > 0 {
		sb.WriteString("\n<ExecRes>\n")
		resultsJSON, err := json.Marshal(allResults)
		if err != nil {
			sb.WriteString(fmt.Sprintf("Error marshaling results: %v\n", err))
		} else {
			sb.WriteString("```json\n")
			sb.WriteString(string(resultsJSON))
			sb.WriteString("\n```\n")
		}
		sb.WriteString("</ExecRes>\n")
	}
	return sb.String(), nil
}

// BuildFinalResponsePromptWithConversationID builds a final response prompt for a specific conversation ID
func BuildFinalResponsePromptWithConversationID(conn *data.Conn, userID int, conversationID string, query string, contextItems []map[string]interface{}, activeChartContext map[string]interface{}, allResults []ExecuteResult, thoughts []string) (string, error) {
	// Start with the basic planning prompt
	sb := strings.Builder{}
	planningPrompt, err := BuildPlanningPromptWithConversationID(conn, userID, conversationID, query, contextItems, activeChartContext)
	if err != nil {
		return "", err
	}
	sb.WriteString(planningPrompt)

	// Add previous thoughts if any
	if len(thoughts) > 0 {
		sb.WriteString("\n<PreviousThoughts>\n")
		for i, thought := range thoughts {
			sb.WriteString(fmt.Sprintf("Turn %d: %s\n", i+1, thought))
		}
		sb.WriteString("</PreviousThoughts>\n")
	}
	// Add execution results
	if len(allResults) > 0 {
		sb.WriteString("\n<ExecRes>\n")
		resultsJSON, err := json.Marshal(allResults)
		if err != nil {
			sb.WriteString(fmt.Sprintf("Error marshaling results: %v\n", err))
		} else {
			sb.WriteString("```json\n")
			sb.WriteString(string(resultsJSON))
			sb.WriteString("\n```\n")
		}
		sb.WriteString("</ExecRes>\n")
	}
	return sb.String(), nil
}

func getDefaultSystemPromptTokenCount(conn *data.Conn) {
	apiKey, err := conn.GetGeminiKey()
	if err != nil {
		return
	}

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return
	}
	systemPrompt, err := getSystemInstruction("defaultSystemPrompt")
	if err != nil {
		return
	}
	enhancedSystemPrompt := enhanceSystemPromptWithTools(systemPrompt)

	CountTokensResponse, err := client.Models.CountTokens(context.Background(), planningModel, genai.Text(enhancedSystemPrompt), &genai.CountTokensConfig{})
	if err != nil {
		return
	}
	if CountTokensResponse != nil {
		defaultSystemPromptTokenCount = int(CountTokensResponse.TotalTokens)
	}
}

// BuildPlanningPromptWithConversationID builds a planning prompt for a specific conversation ID
func BuildPlanningPromptWithConversationID(conn *data.Conn, userID int, conversationID string, query string, contextItems []map[string]interface{}, activeChartContext map[string]interface{}) (string, error) {
	ctx := context.Background()
	var sb strings.Builder

	// Load conversation messages from database if conversationID is provided
	if conversationID != "" {
		messagesInterface, err := GetConversationMessages(ctx, conn, conversationID, userID)
		if err == nil && messagesInterface != nil {
			// Type assert to get the actual messages
			if dbMessages, ok := messagesInterface.([]DBConversationMessage); ok && len(dbMessages) > 0 {
				// Convert DB messages to ChatMessage format for context building
				chatMessages := make([]ChatMessage, len(dbMessages))
				for i, msg := range dbMessages {
					chatMessages[i] = ChatMessage{
						Query:            msg.Query,
						ContentChunks:    msg.ContentChunks,
						ResponseText:     msg.ResponseText,
						FunctionCalls:    msg.FunctionCalls,
						ToolResults:      msg.ToolResults,
						ContextItems:     msg.ContextItems,
						SuggestedQueries: msg.SuggestedQueries,
						Citations:        msg.Citations,
						Timestamp:        msg.CreatedAt,
						TokenCount:       msg.TokenCount,
						Status:           msg.Status,
					}
					if msg.CompletedAt != nil {
						chatMessages[i].CompletedAt = *msg.CompletedAt
					}
				}

				conversationContext := _buildConversationContext(chatMessages)
				sb.WriteString("<ConversationHistory>\n")
				sb.WriteString(conversationContext)
				sb.WriteString("\n</ConversationHistory>\n")
			}
		}
	}

	if len(contextItems) > 0 {
		sb.WriteString("<ContextItems>\n")
		sb.WriteString(_buildContextItems(contextItems))
		sb.WriteString("\n</ContextItems>\n")
	}
	if activeChartContext != nil {
		sb.WriteString("<UserActiveChart>\n")
		ticker, _ := activeChartContext["ticker"].(string)
		secID := fmt.Sprint(activeChartContext["securityId"])
		tsStr := fmt.Sprint(activeChartContext["timestamp"])
		sb.WriteString(fmt.Sprintf("Instance - Ticker: %s, SecurityId: %s, TimestampMs: %s", ticker, secID, tsStr))
		sb.WriteString("\n</UserActiveChart>\n")
	}
	sb.WriteString("<UserQuery>\n")
	sb.WriteString(query)
	sb.WriteString("\n</UserQuery>\n")

	return sb.String(), nil
}

// BuildPlanningPromptWithResultsAndConversationID builds a planning prompt with results for a specific conversation ID
func BuildPlanningPromptWithResultsAndConversationID(conn *data.Conn, userID int, conversationID string, query string, contextItems []map[string]interface{}, activeChartContext map[string]interface{}, results []ExecuteResult, thoughts []string) (string, error) {
	// Start with the basic planning prompt
	sb := strings.Builder{}
	planningPrompt, err := BuildPlanningPromptWithConversationID(conn, userID, conversationID, query, contextItems, activeChartContext)
	if err != nil {
		return "", err
	}
	sb.WriteString(planningPrompt)
	// Add previous thoughts if any
	if len(thoughts) > 0 {
		sb.WriteString("\n<PreviousThoughts>\n")
		for i, thought := range thoughts {
			sb.WriteString(fmt.Sprintf("Turn %d: %s\n", i+1, thought))
		}
		sb.WriteString("</PreviousThoughts>\n")
	}
	// Add execution results
	if len(results) > 0 {
		sb.WriteString("\n<ExecutionResults>\n")
		resultsJSON, err := json.Marshal(results)
		if err != nil {
			sb.WriteString(fmt.Sprintf("Error marshaling results: %v\n", err))
		} else {
			sb.WriteString("```json\n")
			sb.WriteString(string(resultsJSON))
			sb.WriteString("\n```\n")
		}
		sb.WriteString("</ExecutionResults>\n")
	}

	return sb.String(), nil
}

// </prompt.go>
