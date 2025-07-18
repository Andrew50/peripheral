package agent

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

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
	workerResult, err := callWorkerPythonAgentWithProgress(ctx, conn, userID, args, progressCallback)
	if err != nil {
		return nil, fmt.Errorf("error executing worker python agent: %v", err)
	}
	var response RunPythonAgentResponse
	if !workerResult.Success {
		return nil, fmt.Errorf("python agent execution failed: %s", workerResult.ErrorMessage)
	}
	if workerResult.Result != nil {
		response.Result = workerResult.Result
	}
	if workerResult.ExecutionID != "" {
		response.ExecutionID = workerResult.ExecutionID
	}
	if workerResult.Prints != "" {
		response.Prints = workerResult.Prints
	}
	if len(workerResult.Plots) > 0 {
		// Convert PythonAgentPlotData to Plot with extraction
		plots := make([]Plot, len(workerResult.Plots))
		for i, plotData := range workerResult.Plots {
			plots[i] = Plot{
				PlotID:      plotData.PlotID,
				TitleTicker: plotData.TitleTicker,
			}
			// Extract plot attributes from the full plotly data
			extractPythonAgentPlotAttributes(&plots[i], plotData.Data)
		}
		response.Plots = plots
	}
	if len(workerResult.ResponseImages) > 0 {
		responseImages := make([]ResponseImage, len(workerResult.ResponseImages))
		for i, img := range workerResult.ResponseImages {
			responseImages[i] = ResponseImage{
				Data:   img, // img is now a string, not a struct
				Format: "png",
			}
		}
		response.ResponseImages = responseImages
	}
	if err := SetPythonAgentResultToCache(ctx, conn, workerResult.ExecutionID, &response); err != nil {
		log.Printf("Error setting python agent result to cache: %v", err)
	}
	for i := range response.Plots {
		response.Plots[i].Data = []map[string]any{} //Cleaning data so that the agent doesn't see it
	}
	return response, nil
}

type WorkerPythonAgentResult struct {
	Success        bool                  `json:"success"`
	Result         any                   `json:"result,omitempty"`
	Prints         string                `json:"prints,omitempty"`
	Plots          []PythonAgentPlotData `json:"plots,omitempty"`
	ResponseImages []string              `json:"responseImages,omitempty"`
	ExecutionID    string                `json:"executionID,omitempty"`
	ErrorMessage   string                `json:"error,omitempty"`
}

func callWorkerPythonAgentWithProgress(ctx context.Context, conn *data.Conn, userID int, args RunPythonAgentArgs, progressCallback ProgressCallback) (*WorkerPythonAgentResult, error) {
	taskID := fmt.Sprintf("pythonAgent_%d_%d", userID, time.Now().UnixNano())
	task := map[string]interface{}{
		"task_id":   taskID,
		"task_type": "general_python_agent",
		"args": map[string]interface{}{
			"user_id": userID,
			"prompt":  args.Prompt,
			"data":    args.Data,
		},
		"created_at": time.Now().UTC().Format(time.RFC3339),
	}
	taskJSON, err := json.Marshal(task)
	if err != nil {
		return nil, fmt.Errorf("error marshaling task: %v", err)
	}
	conn.Cache.RPush(ctx, "strategy_queue", string(taskJSON))
	workerResult, err := waitForPythonAgentResultWithProgress(ctx, conn, taskID, 4*time.Minute, progressCallback)
	if err != nil {
		return nil, fmt.Errorf("error waiting for python agent result: %v", err)
	}
	return workerResult, nil
}

func waitForPythonAgentResultWithProgress(ctx context.Context, conn *data.Conn, taskID string, timeout time.Duration, progressCallback ProgressCallback) (*WorkerPythonAgentResult, error) {
	pubsub := conn.Cache.Subscribe(ctx, "worker_task_updates")
	defer pubsub.Close()

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ch := pubsub.Channel()

	for {
		select {
		case <-timeoutCtx.Done():
			return nil, fmt.Errorf("timeout waiting for python agent result")
		case msg := <-ch:
			if msg == nil {
				continue
			}
			var taskUpdate map[string]interface{}
			if err := json.Unmarshal([]byte(msg.Payload), &taskUpdate); err != nil {
				log.Printf("Failed to unmarshal task update: %v", err)
				continue
			}
			if taskUpdate["task_id"] == taskID {
				status, _ := taskUpdate["status"].(string)
				if status == "progress" {
					// Handle progress updates
					stage, _ := taskUpdate["stage"].(string)
					message, _ := taskUpdate["message"].(string)
					log.Printf("Python agent progress [%s]: %s", stage, message)

					// Call progress callback if provided
					if progressCallback != nil {
						progressCallback(message)
					}
					continue
				}
				if status == "completed" || status == "failed" {
					// Convert task result to WorkerBacktestResult
					var result WorkerPythonAgentResult
					if resultData, exists := taskUpdate["result"]; exists {
						resultJSON, err := json.Marshal(resultData)
						if err != nil {
							return nil, fmt.Errorf("error marshaling task result: %v", err)
						}
						err = json.Unmarshal(resultJSON, &result)
						if err != nil {
							return nil, fmt.Errorf("error unmarshaling python agent result: %v", err)
						}
					}

					if status == "failed" {
						errorMsg, _ := taskUpdate["error_message"].(string)
						result.Success = false
						result.ErrorMessage = errorMsg
					} else {
						result.Success = true
					}

					return &result, nil
				}
			}
		}
	}
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
