// this is a general agent that works with the underlying infra of chat.go for other internal purposes
// like for example, crafting tweets for our twitter account

package agent

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/invopop/jsonschema"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/responses"
	"github.com/openai/openai-go/shared"
	"go.uber.org/zap"
	"google.golang.org/genai"
)

type ExecutionPlan struct {
	Stage          Stage   `json:"stage"`
	Rounds         []Round `json:"rounds,omitempty"`
	Thoughts       string  `json:"thoughts,omitempty"`
	DiscardResults []int64 `json:"discard_results,omitempty"`
}

func RunGeneralAgent[T any](conn *data.Conn, userID int, additionalSystemPromptFile string, finalSystemPromptFile string, prompt string, finalModel string, finalModelThinkingEffort string) (T, error) {
	var zeroResult T
	systemPrompt, err := getSystemInstruction("generalAgentSystemPrompt")
	if err != nil {
		return zeroResult, fmt.Errorf("error getting system instruction: %w", err)
	}
	var additionalSystemPrompt string
	if additionalSystemPromptFile != "" {
		additionalSystemPrompt, err = getSystemInstruction(additionalSystemPromptFile)
		if err != nil {
			return zeroResult, fmt.Errorf("error getting additional system instruction: %w", err)
		}
	}
	systemPrompt = systemPrompt + "\n" + additionalSystemPrompt

	var executor *Executor
	var activeResults []ExecuteResult
	var discardedResults []ExecuteResult
	var accumulatedThoughts []string

	var modelExecutionPlan ExecutionPlan
	planningPrompt := ""
	maxTurns := 15
	for {
		var err error
		if planningPrompt == "" {
			modelExecutionPlan, err = _generalGeminiGenerateExecutionPlan(context.Background(), conn, systemPrompt, prompt)
			if err != nil {
				return zeroResult, fmt.Errorf("error generating execution plan: %w", err)
			}
		} else {
			modelExecutionPlan, err = _generalGeminiGenerateExecutionPlan(context.Background(), conn, systemPrompt, planningPrompt)
			if err != nil {
				return zeroResult, fmt.Errorf("error generating execution plan: %w", err)
			}
		}

		if modelExecutionPlan.Thoughts != "" {
			accumulatedThoughts = append(accumulatedThoughts, modelExecutionPlan.Thoughts)
		}
		// Handle result discarding if specified in the plan
		if len(modelExecutionPlan.DiscardResults) > 0 && modelExecutionPlan.Stage != StageFinishedExecuting {
			// Create a map for quick lookup of IDs to discard
			discardMap := make(map[int64]bool)
			for _, id := range modelExecutionPlan.DiscardResults {
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
		switch modelExecutionPlan.Stage {
		case StageExecute:
			logger, _ := zap.NewProduction()
			if executor == nil {
				executor = NewExecutor(conn, 0, 5, logger, "", "")
			}
			for _, round := range modelExecutionPlan.Rounds {
				results, err := executor.Execute(context.Background(), round.Calls, round.Parallel)
				if err != nil {
					return zeroResult, fmt.Errorf("error executing plan: %w", err)
				}
				activeResults = append(activeResults, results...)
			}
			planningPrompt, err = _buildGeneralAgentPlanningPromptWithResults(prompt, activeResults, accumulatedThoughts)
			if err != nil {
				return zeroResult, fmt.Errorf("error building planning prompt: %w", err)
			}
		case StageFinishedExecuting:
			finalResultContext, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			finalModelResult, err := _generalAgentGenerateFinalResponse[T](finalResultContext, conn, userID, prompt, finalSystemPromptFile, activeResults, accumulatedThoughts, finalModel, finalModelThinkingEffort)
			if err != nil {
				return zeroResult, fmt.Errorf("error generating final response: %w", err)
			}
			return finalModelResult, nil
		}
		maxTurns--
		if maxTurns <= 0 {
			return zeroResult, fmt.Errorf("max turns reached")
		}
	}

}

func _buildGeneralAgentPlanningPromptWithResults(query string, activeResults []ExecuteResult, accumulatedThoughts []string) (string, error) {
	sb := strings.Builder{}
	sb.WriteString(query)
	if len(accumulatedThoughts) > 0 {
		sb.WriteString("\n<PreviousThoughts>\n")
		for i, thought := range accumulatedThoughts {
			sb.WriteString(fmt.Sprintf("Turn %d: %s\n", i+1, thought))
		}
		sb.WriteString("</PreviousThoughts>\n")
	}
	if len(activeResults) > 0 {
		sb.WriteString("\n<ExecutionResults>\n")
		cleanedResults := cleanExecuteResultsForPrompt(activeResults)
		resultsJSON, err := json.Marshal(cleanedResults)
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

func _generalAgentGenerateFinalResponse[T any](ctx context.Context, conn *data.Conn, userID int, query string, systemPromptFile string, activeResults []ExecuteResult, accumulatedThoughts []string, finalModel string, finalModelThinkingEffort string) (T, error) {
	var zeroResult T
	client := conn.OpenAIClient
	systemPrompt, err := getSystemInstruction(systemPromptFile)
	if err != nil {
		return zeroResult, fmt.Errorf("error getting system instruction: %w", err)
	}
	messages, err := _buildGeneralAgentOpenAIFinalResponseInput(query, activeResults, accumulatedThoughts)
	if err != nil {
		return zeroResult, fmt.Errorf("error building OpenAI messages: %w", err)
	}
	ref := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	model := finalModel

	var zero T
	rawSchema := ref.Reflect(zero)
	b, _ := json.Marshal(rawSchema)
	var oaSchema map[string]any
	_ = json.Unmarshal(b, &oaSchema)

	textConfig := responses.ResponseTextConfigParam{
		Format: responses.ResponseFormatTextConfigUnionParam{
			OfJSONSchema: &responses.ResponseFormatTextJSONSchemaConfigParam{
				Name:   "generalAgentResponse",
				Schema: oaSchema,
				Strict: openai.Bool(true),
			},
		},
	}
	res, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: messages,
		},
		Model:        model,
		Instructions: openai.String(systemPrompt),
		User:         openai.String("user:0"),
		Text:         textConfig,
		Reasoning: responses.ReasoningParam{
			Effort: responses.ReasoningEffort(finalModelThinkingEffort),
		},
		Metadata: shared.Metadata{"userID": strconv.Itoa(userID), "env": conn.ExecutionEnvironment},
	})
	if err != nil {
		return zeroResult, fmt.Errorf("error generating final response: %w", err)
	}
	raw := res.OutputText()
	var finalResp T
	if err := json.Unmarshal([]byte(raw), &finalResp); err != nil {
		return zeroResult, fmt.Errorf("error unmarshalling final response: %w", err)
	}
	return finalResp, nil
}
func _buildGeneralAgentOpenAIFinalResponseInput(query string, executionResults []ExecuteResult, thoughts []string) (responses.ResponseInputParam, error) {
	var messages []responses.ResponseInputItemUnionParam
	messages = append(messages, responses.ResponseInputItemUnionParam{
		OfMessage: &responses.EasyInputMessageParam{
			Role: responses.EasyInputMessageRoleUser,
			Content: responses.EasyInputMessageContentUnionParam{
				OfString: openai.String(query),
			},
		},
	})
	if len(thoughts) > 0 {
		messages = append(messages, responses.ResponseInputItemUnionParam{
			OfMessage: &responses.EasyInputMessageParam{
				Role: responses.EasyInputMessageRoleSystem,
				Content: responses.EasyInputMessageContentUnionParam{
					OfString: openai.String(strings.Join(thoughts, "\n")),
				},
			},
		})
	}
	if len(executionResults) > 0 {
		var allResults []map[string]interface{}
		var allImages []ResponseImage
		for _, result := range executionResults {
			// Skip results that had errors
			if result.Error != nil {
				continue
			}

			// Create a cleaned result without responseImages
			var resultsWithoutResponseImages map[string]interface{}

			// Check if result contains responseImages
			// First try direct cast to map
			var resultMap map[string]interface{}
			var ok bool

			if resultMap, ok = result.Result.(map[string]interface{}); ok {
				// Result is already a map, use it directly
			} else {
				// If direct cast fails, convert any type to map through JSON marshaling
				// Marshal the result to JSON
				jsonBytes, err := json.Marshal(result.Result)
				if err != nil {
					fmt.Printf("Failed to marshal result to JSON: %v\n", err)
					// Keep original result if marshaling fails
					resultData := map[string]interface{}{
						"fn":   result.FunctionName,
						"res":  result.Result,
						"args": result.Args,
					}
					allResults = append(allResults, resultData)
					continue
				}

				// Unmarshal back to map[string]interface{}
				if err := json.Unmarshal(jsonBytes, &resultMap); err != nil {
					fmt.Printf("Failed to unmarshal JSON to map: %v\n", err)
					// Keep original result if unmarshaling fails
					resultData := map[string]interface{}{
						"fn":   result.FunctionName,
						"res":  result.Result,
						"args": result.Args,
					}
					allResults = append(allResults, resultData)
					continue
				}
			}

			// Now check for responseImages in the converted map
			if responseImages, hasImages := resultMap["responseImages"]; hasImages {
				// Try to extract and append images to allImages with multiple type assertions
				// First try []ResponseImage
				if imageList, ok := responseImages.([]ResponseImage); ok {
					allImages = append(allImages, imageList...)
				} else if imageList, ok := responseImages.([]interface{}); ok {
					// Try []interface{} (common after JSON unmarshaling)
					for _, img := range imageList {
						imgBytes, err := json.Marshal(img)
						if err == nil {
							var responseImg ResponseImage
							if err := json.Unmarshal(imgBytes, &responseImg); err == nil {
								allImages = append(allImages, responseImg)
							}
						}
					}
				}

				// Always create a copy of the result without responseImages (regardless of processing success)
				cleanedResultMap := make(map[string]interface{})
				for k, v := range resultMap {
					if k != "responseImages" {
						cleanedResultMap[k] = v
					}
				}
				resultsWithoutResponseImages = cleanedResultMap
			} else {
				resultsWithoutResponseImages = resultMap
			}

			resultData := map[string]interface{}{
				"fn":   result.FunctionName,
				"res":  resultsWithoutResponseImages,
				"args": result.Args,
			}

			allResults = append(allResults, resultData)
		}

		// Only add execution results message if we have successful results
		if len(allResults) > 0 {
			combinedContent, err := json.Marshal(map[string]interface{}{
				"execution_results": allResults,
			})
			if err != nil {
				combinedContent = []byte(fmt.Sprintf("Error marshaling execution results: %v", err))
			}
			messages = append(messages, responses.ResponseInputItemUnionParam{
				OfMessage: &responses.EasyInputMessageParam{
					Role: responses.EasyInputMessageRoleSystem,
					Content: responses.EasyInputMessageContentUnionParam{
						OfString: openai.String(string(combinedContent)),
					},
				},
			})
		}
		if len(allImages) > 0 {
			var imageContent []responses.ResponseInputContentUnionParam
			for _, img := range allImages {
				// Format as data URL: data:image/png;base64,{base64_data}
				dataURL := fmt.Sprintf("data:image/%s;base64,%s", img.Format, img.Data)
				imageContent = append(imageContent, responses.ResponseInputContentUnionParam{
					OfInputImage: &responses.ResponseInputImageParam{
						ImageURL: openai.String(dataURL),
					},
				})
			}
			// Add images as a system message with mixed content
			messages = append(messages, responses.ResponseInputItemUnionParam{
				OfMessage: &responses.EasyInputMessageParam{
					Role: responses.EasyInputMessageRoleUser,
					Content: responses.EasyInputMessageContentUnionParam{
						OfInputItemContentList: imageContent,
					},
				},
			})
		}
	}
	est, _ := time.LoadLocation("America/New_York")
	messages = append(messages, responses.ResponseInputItemUnionParam{
		OfMessage: &responses.EasyInputMessageParam{
			Role: responses.EasyInputMessageRoleSystem,
			Content: responses.EasyInputMessageContentUnionParam{
				OfString: openai.String(fmt.Sprintf("CURRENT DATE (EST/Market Time): %s\n CURRENT TIME IN SECONDS: %d", time.Now().In(est).Format("2006-01-02 15:04:05"), time.Now().In(est).Unix())),
			},
		},
	})

	return messages, nil
}

func _generalGeminiGenerateExecutionPlan(ctx context.Context, conn *data.Conn, systemPrompt string, prompt string) (ExecutionPlan, error) {

	geminiClient := conn.GeminiClient
	thinkingBudget := int32(5000)
	enhancedSystemInstruction := enhanceSystemPromptWithTools(systemPrompt, false)
	geminiConfig := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: enhancedSystemInstruction},
			},
		},
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
			ThinkingBudget:  &thinkingBudget,
		},
		ResponseMIMEType: "application/json",
	}
	prompt = appendCurrentTimeToPrompt(prompt)
	geminiResult, err := geminiClient.Models.GenerateContent(ctx, planningModel, genai.Text(prompt), geminiConfig)
	if err != nil {
		return ExecutionPlan{}, fmt.Errorf("gemini had an error generating execution plan: %w", err)
	}

	if len(geminiResult.Candidates) == 0 {
		return ExecutionPlan{}, fmt.Errorf("no candidates returned from gemini")
	}
	var sb strings.Builder
	candidate := geminiResult.Candidates[0]
	if candidate.Content != nil {
		for _, part := range candidate.Content.Parts {
			if part.Thought {
				continue
			}
			if part.Text != "" {
				sb.WriteString(part.Text)
			}
		}
	}
	geminiResultText := strings.TrimSpace(sb.String())
	fmt.Println("geminiResultText", geminiResultText)
	var executionPlan ExecutionPlan
	planParseErr := json.Unmarshal([]byte(geminiResultText), &executionPlan)
	if planParseErr == nil && executionPlan.Stage != "" {
		return executionPlan, nil
	}
	// If no markdown code block found, try to extract JSON block using { } method
	jsonBlock := ""
	jsonStartIdx := strings.Index(geminiResultText, "{")

	if jsonStartIdx != -1 {
		// Try to find the matching closing brace by counting braces
		braceCount := 0
		jsonEndIdx := -1

		for i := jsonStartIdx; i < len(geminiResultText); i++ {
			if geminiResultText[i] == '{' {
				braceCount++
			} else if geminiResultText[i] == '}' {
				braceCount--
				if braceCount == 0 {
					jsonEndIdx = i
					break
				}
			}
		}

		if jsonEndIdx != -1 {
			jsonBlock = geminiResultText[jsonStartIdx : jsonEndIdx+1]
			jsonBlock = strings.TrimSpace(jsonBlock)
		}
	}

	executionPlan = ExecutionPlan{} // Reset the struct
	// Try unmarshalling the extracted block if it's not empty
	if jsonBlock != "" {
		blockPlanParseErr := json.Unmarshal([]byte(jsonBlock), &executionPlan)
		if blockPlanParseErr == nil && executionPlan.Stage != "" {
			return executionPlan, nil
		}
	}
	return ExecutionPlan{}, fmt.Errorf("no valid execution plan found")
}
