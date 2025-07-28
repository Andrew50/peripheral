package twitter

import (
	"backend/internal/app/agent"
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/invopop/jsonschema"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/responses"
	"github.com/openai/openai-go/shared"
)

func GenerateAskPeripheralTweet(conn *data.Conn, tweet ExtractedTweetData) error {
	tweetText := tweet.Text
	fmt.Println("Tweet text", tweetText)
	// Remove @askperipheral mentions (case insensitive)
	tweetText = strings.ReplaceAll(tweetText, "@AskPeripheral ", "")
	tweetText = strings.ReplaceAll(tweetText, "@askPeripheral ", "")
	tweetText = strings.ReplaceAll(tweetText, "@PeripheralIO ", "")
	tweetText = strings.ReplaceAll(tweetText, "@peripheralio ", "")

	// Clean up extra whitespace
	tweetText = strings.TrimSpace(tweetText)

	chatRequest := agent.ChatRequest{
		Query: tweetText,
	}
	chatRequestJSON, err := json.Marshal(chatRequest)
	if err != nil {
		fmt.Printf("Error marshalling chat request: %v", err)
		return err
	}
	res, err := agent.GetChatRequest(context.Background(), conn, 0, chatRequestJSON)
	if err != nil {
		fmt.Printf("Error generating Ask Peripheral tweet: %v", err)
		return err
	}
	chatResult, ok := res.(agent.QueryResponse)
	if !ok {
		fmt.Printf("Error casting chat result to ChatResponse: %v", err)
		return err
	}
	conversationID := chatResult.ConversationID
	_, err = conn.DB.Exec(context.Background(), "UPDATE conversations SET is_public = true WHERE conversation_id = $1", conversationID)
	if err != nil {
		fmt.Printf("Error updating conversation to public: %v", err)
		return err
	}

	chatResultText := ""
	var availablePlots []map[string]interface{}

	for _, chunk := range chatResult.ContentChunks {
		if chunk.Type == "text" {
			chatResultText += chunk.Content.(string)
		} else if chunk.Type == "table" {
			tableData := chunk.Content.(map[string]interface{})
			chatResultText += "\n Table: " + tableData["caption"].(string) + "\n"

			// Append column headers if they exist
			if headers, ok := tableData["headers"].([]interface{}); ok {
				chatResultText += "Columns: "
				for i, header := range headers {
					if i > 0 {
						chatResultText += ", "
					}
					chatResultText += fmt.Sprintf("%v", header)
				}
				chatResultText += "\n"
			}
		} else if chunk.Type == "plot" {
			if plotData, ok := chunk.Content.(map[string]interface{}); ok {
				plotInfo := map[string]interface{}{
					"index":        len(availablePlots),
					"originalPlot": plotData,
				}
				if title, exists := plotData["title"].(string); exists {
					plotInfo["title"] = title
				} else {
					plotInfo["title"] = fmt.Sprintf("Plot %d", len(availablePlots)+1)
				}
				if chartType, exists := plotData["chart_type"].(string); exists {
					plotInfo["chart_type"] = chartType
				}
				availablePlots = append(availablePlots, plotInfo)
			}
		}
	}
	// Build comprehensive prompt with plot information
	promptText := fmt.Sprintf("Query: %s\n Response: %s\n\n", tweetText, chatResultText)
	// append information about plots
	if len(availablePlots) > 0 {
		promptText += "Plots:\n"
		for i, plot := range availablePlots {
			plotTitle := plot["title"].(string)
			plotType := ""
			if ct, ok := plot["chart_type"].(string); ok {
				plotType = fmt.Sprintf(" (%s)", ct)
			}
			promptText += fmt.Sprintf("- Plot %d: %s%s\n", i, plotTitle, plotType)
		}
		promptText += "\n"
	}

	openAIClient := conn.OpenAIClient
	var messages []responses.ResponseInputItemUnionParam
	messages = append(messages, responses.ResponseInputItemUnionParam{
		OfMessage: &responses.EasyInputMessageParam{
			Role: responses.EasyInputMessageRoleUser,
			Content: responses.EasyInputMessageContentUnionParam{
				OfString: openai.String(promptText),
			},
		},
	})

	ref := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	model := "o4-mini"
	thinkingEffort := "medium"
	rawSchema := ref.Reflect(AgentReplyTweet{})
	b, _ := json.Marshal(rawSchema)
	var oaSchema map[string]any
	_ = json.Unmarshal(b, &oaSchema)

	textConfig := responses.ResponseTextConfigParam{
		Format: responses.ResponseFormatTextConfigUnionParam{
			OfJSONSchema: &responses.ResponseFormatTextJSONSchemaConfigParam{
				Name:   "agentTweet",
				Schema: oaSchema,
				Strict: openai.Bool(true),
			},
		},
	}
	instructions, err := agent.GetSystemInstruction("AskPeripheralPrompt")
	if err != nil {
		fmt.Printf("Error getting system instruction: %v", err)
		return err
	}

	openAIRes, err := openAIClient.Responses.New(context.Background(), responses.ResponseNewParams{
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: messages,
		},
		Model:        model,
		Instructions: openai.String(instructions),
		User:         openai.String("user:0"),
		Text:         textConfig,
		Reasoning: responses.ReasoningParam{
			Effort: responses.ReasoningEffort(thinkingEffort),
		},
		Metadata: shared.Metadata{"userID": "0", "env": conn.ExecutionEnvironment, "convID": conversationID},
	})
	if err != nil {
		fmt.Printf("Error generating tweet response: %v", err)
		return err
	}

	// Parse the response
	var tweetResponse AgentReplyTweet
	raw := openAIRes.OutputText()
	if err := json.Unmarshal([]byte(raw), &tweetResponse); err != nil {
		fmt.Printf("Error parsing tweet response: %v", err)
		return err
	}

	tweetResponse.Text = strings.ReplaceAll(tweetResponse.Text, "{{CHAT_LINK}}", "https://peripheral.io/share/"+conversationID)

	// If model selected a plot by index, get the actual plot data
	if tweetResponse.Plot != nil {
		if plotIndex, ok := tweetResponse.Plot.(float64); ok && int(plotIndex) < len(availablePlots) {
			// Model returned an index, replace with actual plot data
			selectedPlot := availablePlots[int(plotIndex)]
			tweetResponse.Plot = selectedPlot["originalPlot"]
		}
	}

	// Render the plot to base64
	base64PNG, err := RenderTwitterPlotToBase64(conn, tweetResponse.Plot)
	if err != nil {
		fmt.Printf("Error rendering plot: %v", err)
		return err
	}

	formattedAskPeripheralTweet := FormattedPeripheralTweet{
		Text:  tweetResponse.Text,
		Image: base64PNG,
	}
	fmt.Println("Tweet response", formattedAskPeripheralTweet)
	//SendTweetToPeripheralTwitterAccount(conn, formattedAskPeripheralTweet)

	return nil
}
