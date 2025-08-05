package agent

import (
	"backend/internal/data"
	"backend/internal/queue"
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/go-redis/redis/v8"
)

// ProgressCallback is a function type for sending progress updates during Python agent execution
type ProgressCallback func(message string)

type Plot struct {
	Data        []map[string]any `json:"data"`
	PlotID      int              `json:"plotID"`
	ChartType   string           `json:"chartType,omitempty"`
	Length      int              `json:"length,omitempty"`
	Title       string           `json:"title,omitempty"`
	Layout      map[string]any   `json:"layout,omitempty"`
	TitleTicker string           `json:"titleTicker,omitempty"`
}

// PythonAgentPlotData represents the full plotly data from Python
type PythonAgentPlotData struct {
	PlotID      int            `json:"plotID"`
	Data        map[string]any `json:"data"`                  // Full plotly figure object
	TitleTicker string         `json:"titleTicker,omitempty"` // Ticker for the title
}

type RunPythonAgentArgs struct {
	Prompt string `json:"prompt"`
	Data   string `json:"data"`
}
type RunPythonAgentResponse struct {
	Result         any             `json:"result,omitempty"`
	ExecutionID    string          `json:"executionID,omitempty"`
	Prints         string          `json:"prints,omitempty"`
	Plots          []Plot          `json:"plots,omitempty"`
	ResponseImages []ResponseImage `json:"responseImages,omitempty"`
}

func RunPythonAgent(ctx context.Context, conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	return RunPythonAgentWithProgress(ctx, conn, userID, rawArgs, nil)
}

func RunPythonAgentWithProgress(ctx context.Context, conn *data.Conn, userID int, rawArgs json.RawMessage, progressCallback ProgressCallback) (any, error) {
	var args RunPythonAgentArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	// Extract context values
	messageID, _ := ctx.Value("messageID").(string)
	conversationID, _ := ctx.Value("conversationID").(string)

	// Build args map for queue
	queueArgs := map[string]interface{}{
		"user_id":         userID,
		"prompt":          args.Prompt,
		"data":            args.Data,
		"conversation_id": conversationID,
		"message_id":      messageID,
	}

	// Use standard queue with progress callback
	handle, err := queue.QueuePythonAgent(ctx, conn, queueArgs)
	if err != nil {
		return nil, fmt.Errorf("error queuing python agent: %v", err)
	}

	// Await result with progress callback
	result, err := queue.AwaitTypedResult[queue.PythonAgentResult](ctx, handle, func(update queue.ResultUpdate) {
		if progressCallback != nil && update.Status == "running" {
			if message, ok := update.Data["message"].(string); ok {
				progressCallback(message)
			}
		}
	})
	if err != nil {
		return nil, fmt.Errorf("error waiting for python agent result: %v", err)
	}

	// Check for errors in the result
	if !result.Success {
		errorMsg := result.Error
		if result.ErrorDetails != nil {
			errorMsg = result.ErrorDetails.Message
		}
		return nil, fmt.Errorf("python agent execution failed: %s", errorMsg)
	}

	// Convert queue result to API response format
	response := convertQueueResultToResponse(result)

	// Cache the result
	if err := SetPythonAgentResultToCache(ctx, conn, result.ExecutionID, &response); err != nil {
		log.Printf("Error setting python agent result to cache: %v", err)
	}

	// Clean data from plots for agent response
	for i := range response.Plots {
		response.Plots[i].Data = []map[string]any{}
	}

	return response, nil
}

func convertQueueResultToResponse(result *queue.PythonAgentResult) RunPythonAgentResponse {
	response := RunPythonAgentResponse{
		ExecutionID: result.ExecutionID,
		Prints:      result.Prints,
	}

	if result.Result != nil {
		response.Result = result.Result
	}

	// Convert plots
	if len(result.Plots) > 0 {
		plots := make([]Plot, len(result.Plots))
		for i, plotData := range result.Plots {
			plots[i] = Plot{
				PlotID:      plotData.PlotID,
				TitleTicker: plotData.TitleTicker,
			}
			extractPythonAgentPlotAttributes(&plots[i], plotData.Data)
		}
		response.Plots = plots
	}

	// Convert response images
	if len(result.ResponseImages) > 0 {
		responseImages := make([]ResponseImage, len(result.ResponseImages))
		for i, img := range result.ResponseImages {
			responseImages[i] = ResponseImage{
				Data:   img,
				Format: "png",
			}
		}
		response.ResponseImages = responseImages
	}

	return response
}

func SetPythonAgentResultToCache(ctx context.Context, conn *data.Conn, executionID string, result *RunPythonAgentResponse) error {
	cacheKey := fmt.Sprintf("python_agent_result_%s", executionID)
	cacheValue, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("error marshaling result: %v", err)
	}
	conn.Cache.Set(ctx, cacheKey, string(cacheValue), 0)
	return nil
}
func GetPythonAgentResultFromCache(ctx context.Context, conn *data.Conn, executionID string) (*RunPythonAgentResponse, error) {
	cacheKey := fmt.Sprintf("python_agent_result_%s", executionID)
	cacheValue, err := conn.Cache.Get(ctx, cacheKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // NEED TO IMPLEMENT THIS
		}
	}
	var result RunPythonAgentResponse
	if err := json.Unmarshal([]byte(cacheValue), &result); err != nil {
		return nil, fmt.Errorf("error unmarshaling result: %v", err)
	}
	return &result, nil
}

func InvalidatePythonAgentResultCache(ctx context.Context, conn *data.Conn, executionID string) error {
	cacheKey := fmt.Sprintf("python_agent_result_%s", executionID)

	return conn.Cache.Del(ctx, cacheKey).Err()
}

// extractPythonAgentPlotAttributes extracts chart attributes from plotly JSON data (similar to backtest)
func extractPythonAgentPlotAttributes(plot *Plot, plotData map[string]any) {
	if plotData == nil {
		return
	}

	// Safely convert plotData["data"] to []map[string]any
	if dataSlice, ok := plotData["data"].([]interface{}); ok {
		converted := make([]map[string]any, len(dataSlice))
		for i, v := range dataSlice {
			if m, ok := v.(map[string]any); ok {
				converted[i] = m
			} else {
				converted[i] = nil // or handle error as needed
			}
		}
		plot.Data = converted
	}

	// Extract chart title
	if layout, ok := plotData["layout"].(map[string]any); ok {
		if title, ok := layout["title"].(map[string]any); ok {
			if titleText, ok := title["text"].(string); ok {
				plot.Title = titleText
			}
		} else if titleStr, ok := layout["title"].(string); ok {
			plot.Title = titleStr
		}
	}

	// Extract chart type and length from data traces
	if dataTraces, ok := plotData["data"].([]interface{}); ok && len(dataTraces) > 0 {
		plot.Length = len(dataTraces)

		// Get chart type from first trace
		if firstTrace, ok := dataTraces[0].(map[string]any); ok {
			if traceType, ok := firstTrace["type"].(string); ok {
				plot.ChartType = mapPythonAgentPlotlyTypeToChartType(traceType, firstTrace)
			}
		}
	}

	// Extract minimal layout information
	if layout, ok := plotData["layout"].(map[string]any); ok {
		minimalLayout := make(map[string]any)

		// Extract axis titles
		if xaxis, ok := layout["xaxis"].(map[string]any); ok {
			if xaxisTitle, ok := xaxis["title"].(map[string]any); ok {
				if titleText, ok := xaxisTitle["text"].(string); ok {
					minimalLayout["xaxis"] = map[string]any{"title": titleText}
				}
			} else if titleStr, ok := xaxis["title"].(string); ok {
				minimalLayout["xaxis"] = map[string]any{"title": titleStr}
			}
		}

		if yaxis, ok := layout["yaxis"].(map[string]any); ok {
			if yaxisTitle, ok := yaxis["title"].(map[string]any); ok {
				if titleText, ok := yaxisTitle["text"].(string); ok {
					minimalLayout["yaxis"] = map[string]any{"title": titleText}
				}
			} else if titleStr, ok := yaxis["title"].(string); ok {
				minimalLayout["yaxis"] = map[string]any{"title": titleStr}
			}
		}

		// Extract dimensions
		if width, ok := layout["width"]; ok {
			minimalLayout["width"] = width
		}
		if height, ok := layout["height"]; ok {
			minimalLayout["height"] = height
		}

		plot.Layout = minimalLayout
	}
}

// mapPythonAgentPlotlyTypeToChartType converts plotly trace types to standard chart types
func mapPythonAgentPlotlyTypeToChartType(traceType string, trace map[string]any) string {
	switch traceType {
	case "scatter":
		if mode, ok := trace["mode"].(string); ok {
			if mode == "lines" {
				return "line"
			}
		}
		return "scatter"
	case "line":
		return "line"
	case "bar":
		return "bar"
	case "histogram":
		return "histogram"
	case "heatmap":
		return "heatmap"
	case "box":
		return "bar" // Fallback
	case "violin":
		return "bar" // Fallback
	case "pie":
		return "bar" // Fallback
	case "candlestick":
		return "line" // Fallback
	case "ohlc":
		return "line" // Fallback
	default:
		return "line"
	}
}
