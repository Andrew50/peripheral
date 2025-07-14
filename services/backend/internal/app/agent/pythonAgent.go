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

type RunPythonAgentArgs struct {
	Prompt string `json:"prompt"`
}
type RunPythonAgentResponse struct {
	Result         any             `json:"result,omitempty"`
	ExecutionID    string          `json:"execution_id,omitempty"`
	Prints         string          `json:"prints,omitempty"`
	Plots          []Plot          `json:"plots,omitempty"`
	ResponseImages []ResponseImage `json:"response_images,omitempty"`
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
	fmt.Println("workerResult", workerResult)
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
		response.Plots = workerResult.Plots
	}
	if len(workerResult.ResponseImages) > 0 {
		responseImages := make([]ResponseImage, len(workerResult.ResponseImages))
		for i, img := range workerResult.ResponseImages {
			responseImages[i] = ResponseImage{
				Data:   img.Data,
				Format: "png",
			}
		}
		response.ResponseImages = responseImages
	}
	err = SetPythonAgentResultToCache(ctx, conn, workerResult.ExecutionID, workerResult)
	if err != nil {
		log.Printf("Error setting python agent result to cache: %v", err)
	}
	for i := range response.Plots {
		response.Plots[i].Data = []map[string]any{} //Cleaning data so that the agent doesn't see it
	}
	return response, nil
}

type WorkerPythonAgentResult struct {
	Success        bool            `json:"success"`
	Result         any             `json:"result,omitempty"`
	Prints         string          `json:"prints,omitempty"`
	Plots          []Plot          `json:"plots,omitempty"`
	ResponseImages []ResponseImage `json:"response_images,omitempty"`
	ExecutionID    string          `json:"execution_id,omitempty"`
	ErrorMessage   string          `json:"error,omitempty"`
}

func callWorkerPythonAgentWithProgress(ctx context.Context, conn *data.Conn, userID int, args RunPythonAgentArgs, progressCallback ProgressCallback) (*WorkerPythonAgentResult, error) {
	taskID := fmt.Sprintf("pythonAgent_%d_%d", userID, time.Now().UnixNano())
	task := map[string]interface{}{
		"task_id":   taskID,
		"task_type": "general_python_agent",
		"args": map[string]interface{}{
			"user_id": userID,
			"prompt":  args.Prompt,
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
						fmt.Println("resultJSON", string(resultJSON))
						err = json.Unmarshal(resultJSON, &result)
						if err != nil {
							return nil, fmt.Errorf("error unmarshaling backtest result: %v", err)
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
func SetPythonAgentResultToCache(ctx context.Context, conn *data.Conn, executionID string, result *WorkerPythonAgentResult) error {
	cacheKey := fmt.Sprintf("python_agent_result_%s", executionID)
	cacheValue, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("error marshaling result: %v", err)
	}
	conn.Cache.Set(ctx, cacheKey, string(cacheValue), 0)
	return nil
}
func GetPythonAgentResultFromCache(ctx context.Context, conn *data.Conn, executionID string) (*WorkerPythonAgentResult, error) {
	cacheKey := fmt.Sprintf("python_agent_result_%s", executionID)
	cacheValue, err := conn.Cache.Get(ctx, cacheKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // NEED TO IMPLEMENT THIS
		}
	}
	var result WorkerPythonAgentResult
	if err := json.Unmarshal([]byte(cacheValue), &result); err != nil {
		return nil, fmt.Errorf("error unmarshaling result: %v", err)
	}
	return &result, nil
}

func InvalidatePythonAgentResultCache(ctx context.Context, conn *data.Conn, executionID string) error {
	cacheKey := fmt.Sprintf("python_agent_result_%s", executionID)

	return conn.Cache.Del(ctx, cacheKey).Err()
}
