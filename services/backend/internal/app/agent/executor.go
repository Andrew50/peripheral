// <executor.go>
package agent

import (
	"backend/internal/data"
	"backend/internal/services/socket"
	"context"

	"encoding/json"
	"fmt"
	"regexp"
	"sync"
	"sync/atomic"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/responses"
)

// ExecuteResult represents the result of executing a function
type ExecuteResult struct {
	FunctionID   int64       `json:"fn_id"`
	FunctionName string      `json:"fn"`
	Result       interface{} `json:"res"`
	Error        *string     `json:"err,omitempty"`
	Args         interface{} `json:"args,omitempty"`
}

// Executor manages the execution of tasks in a queue
type Executor struct {
	conn            *data.Conn
	userID          int
	tools           map[string]Tool
	log             *zap.Logger
	tracer          trace.Tracer
	maxWorkers      int
	limiter         chan struct{}
	functionCounter int64     // Thread-safe counter for result IDs
	resultPool      sync.Pool // Pool for ExecuteResult slices
	conversationID  string
	messageID       string
}

// NewExecutor creates a new Executor
func NewExecutor(conn *data.Conn, userID int, maxWorkers int, lg *zap.Logger, conversationID string, messageID string) *Executor {
	if maxWorkers <= 0 {
		maxWorkers = 5
	}

	executor := &Executor{
		conn:            conn,
		userID:          userID,
		tools:           Tools,
		log:             lg,
		tracer:          otel.Tracer("agent-executor"),
		maxWorkers:      maxWorkers,
		limiter:         make(chan struct{}, maxWorkers),
		functionCounter: 0,
		conversationID:  conversationID,
		messageID:       messageID,
	}

	// Initialize result pool for memory optimization
	executor.resultPool = sync.Pool{
		New: func() interface{} {
			// Pre-allocate slice with common batch size
			return make([]ExecuteResult, 0, 10)
		},
	}

	return executor
}

func (e *Executor) Execute(ctx context.Context, functionCalls []FunctionCall, parallel bool) ([]ExecuteResult, error) {
	// Pre-allocate results slice with exact capacity
	results := make([]ExecuteResult, len(functionCalls))

	if !parallel || len(functionCalls) == 1 {
		// Sequential execution - direct slice assignment
		for i, fc := range functionCalls {
			results[i], _ = e.executeFunction(ctx, fc)
		}
		return results, nil
	}

	// Parallel execution with optimized goroutine pool
	g, ctx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, e.maxWorkers)

	for i, fc := range functionCalls {
		i, fc := i, fc // Capture loop variables
		g.Go(func() error {
			// Add panic recovery for each goroutine
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("Panic recovered in executor goroutine: %v\n", r)
					// Create an error result for the panic case
					errorStr := fmt.Sprintf("panic occurred during execution: %v", r)
					results[i] = ExecuteResult{
						FunctionID:   atomic.AddInt64(&e.functionCounter, 1),
						FunctionName: fc.Name,
						Error:        &errorStr,
						Args:         fc.Args,
					}
				}
			}()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Execute function and assign directly to pre-allocated slice
			result, _ := e.executeFunction(ctx, fc)
			results[i] = result

			// Return error to errgroup (but don't fail the whole batch)
			// Individual function errors are captured in ExecuteResult.Error
			return nil
		})
	}

	// Wait for all goroutines to complete
	if err := g.Wait(); err != nil {
		return results, err
	}

	return results, nil
}

// ExecuteOptimized provides enhanced execution with intelligent batching
func (e *Executor) ExecuteOptimized(ctx context.Context, functionCalls []FunctionCall, parallel bool) ([]ExecuteResult, error) {
	if len(functionCalls) == 0 {
		return []ExecuteResult{}, nil
	}

	// Pre-allocate results slice with exact capacity
	results := make([]ExecuteResult, len(functionCalls))

	if !parallel || len(functionCalls) == 1 {
		// Sequential execution - direct slice assignment
		for i, fc := range functionCalls {
			results[i], _ = e.executeFunction(ctx, fc)
		}
		return results, nil
	}

	// Intelligent batching: group similar functions together
	batches := e.createOptimalBatches(functionCalls)

	// Execute batches with optimal concurrency
	return e.executeBatches(ctx, batches, results, functionCalls)
}

// createOptimalBatches groups related function calls for better performance
func (e *Executor) createOptimalBatches(functionCalls []FunctionCall) [][]int {
	// Group functions by type for potential optimizations
	funcGroups := make(map[string][]int)

	for i, fc := range functionCalls {
		funcGroups[fc.Name] = append(funcGroups[fc.Name], i)
	}

	// Create batches, prioritizing similar functions
	var batches [][]int
	batchSize := e.maxWorkers

	for _, indices := range funcGroups {
		for len(indices) > 0 {
			end := batchSize
			if end > len(indices) {
				end = len(indices)
			}
			batches = append(batches, indices[:end])
			indices = indices[end:]
		}
	}

	return batches
}

// executeBatches executes function call batches with optimal concurrency
func (e *Executor) executeBatches(ctx context.Context, batches [][]int, results []ExecuteResult, functionCalls []FunctionCall) ([]ExecuteResult, error) {
	for _, batch := range batches {
		if err := e.executeBatch(ctx, batch, results, functionCalls); err != nil {
			return results, err
		}
	}
	return results, nil
}

// executeBatch executes a single batch of function calls
func (e *Executor) executeBatch(ctx context.Context, indices []int, results []ExecuteResult, functionCalls []FunctionCall) error {
	g, ctx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, len(indices)) // Use batch size for semaphore

	for _, idx := range indices {
		idx := idx // Capture loop variable
		g.Go(func() error {
			// Add panic recovery for each goroutine
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("Panic recovered in executor batch goroutine: %v\n", r)
					// Create an error result for the panic case
					errorStr := fmt.Sprintf("panic occurred during batch execution: %v", r)
					results[idx] = ExecuteResult{
						FunctionID:   atomic.AddInt64(&e.functionCounter, 1),
						FunctionName: functionCalls[idx].Name,
						Error:        &errorStr,
						Args:         functionCalls[idx].Args,
					}
				}
			}()

			sem <- struct{}{}
			defer func() { <-sem }()

			// Execute function and assign directly to results slice
			result, _ := e.executeFunction(ctx, functionCalls[idx])
			results[idx] = result
			return nil
		})
	}

	return g.Wait()
}

func (e *Executor) executeFunction(ctx context.Context, fc FunctionCall) (ExecuteResult, error) {
	// Generate unique result ID using atomic increment
	functionID := atomic.AddInt64(&e.functionCounter, 1)

	tool, exists := e.tools[fc.Name]
	if !exists {
		errorStr := fmt.Sprintf("function '%s' not found", fc.Name)
		return ExecuteResult{
			FunctionID:   functionID,
			FunctionName: fc.Name,
			Error:        &errorStr,
			Args:         fc.Args,
		}, nil
	}

	// Check if context is cancelled before executing
	if ctx.Err() != nil {
		errorStr := "request was cancelled"
		return ExecuteResult{
			FunctionID:   functionID,
			FunctionName: fc.Name,
			Error:        &errorStr,
			Args:         fc.Args,
		}, nil
	}

	var argsMap map[string]interface{}
	_ = json.Unmarshal(fc.Args, &argsMap)
	go func() {
		var cleanedMessage string
		if thoughtsValue := ctx.Value("peripheralLatestModelThoughts"); thoughtsValue != nil {
			if thoughtsStr, ok := thoughtsValue.(string); ok {
				cleanedMessage = cleanStatusMessage(e.conn, thoughtsStr)
			}
		}
		data := map[string]interface{}{
			"message":  cleanedMessage,
			"headline": formatStatusMessage(tool.StatusMessage, argsMap),
		}
		if tool.StatusMessage != "" {
			socket.SendAgentStatusUpdate(e.userID, "FunctionUpdate", data)
		}
	}()
	_, span := e.tracer.Start(ctx, fc.Name, trace.WithAttributes(attribute.String("agent.tool", fc.Name)))
	defer span.End()
	result, err := tool.Function(ctx, e.conn, e.userID, fc.Args)
	if err != nil {
		span.RecordError(err)
		e.log.Warn("Error executing function", zap.String("function", fc.Name), zap.Error(err))
		errorStr := err.Error()
		return ExecuteResult{
			FunctionID:   functionID,
			FunctionName: fc.Name,
			Error:        &errorStr,
			Args:         argsMap,
		}, nil
	}
	return ExecuteResult{
		FunctionID:   functionID,
		FunctionName: fc.Name,
		Result:       result,
		Args:         argsMap,
	}, nil
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
	apiKey := conn.OpenAIKey

	client := openai.NewClient(option.WithAPIKey(apiKey))
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

// </executor.go>
