package agent

import (
	"backend/internal/data"
	"context"

	"encoding/json"
	"fmt"
	"regexp"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// ExecuteResult represents the result of executing a function
type ExecuteResult struct {
	FunctionName string      `json:"function_name"`
	Result       interface{} `json:"result"`
	Error        string      `json:"error,omitempty"`
	Args         interface{} `json:"args,omitempty"`
}

type Executor struct {
	conn       *data.Conn
	userId     int
	tools      map[string]Tool
	log        *zap.Logger
	tracer     trace.Tracer
	maxWorkers int
}

func NewExecutor(conn *data.Conn, userId int, maxWorkers int, lg *zap.Logger) *Executor {
	if maxWorkers <= 0 {
		maxWorkers = 3
	}
	return &Executor{
		conn:       conn,
		userId:     userId,
		tools:      Tools,
		log:        lg,
		tracer:     otel.Tracer("agent-executor"),
		maxWorkers: maxWorkers,
	}
}
func (e *Executor) Execute(ctx context.Context, functionCalls []FunctionCall, parallel bool) ([]ExecuteResult, error) {

	if !parallel || len(functionCalls) == 1 {
		var result []ExecuteResult
		for _, fc := range functionCalls {
			res, _ := e.executeFunction(ctx, fc)
			result = append(result, res)
		}
		return result, nil
	}
	resCh := make(chan ExecuteResult, len(functionCalls))
	g, ctx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, e.maxWorkers)

	for _, fc := range functionCalls {
		fc := fc
		g.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := e.executeFunction(ctx, fc)

			select {
			case resCh <- result:
			case <-ctx.Done():
			}
			return err
		})
	}
	go func() {
		_ = g.Wait()
		close(resCh)
	}()
	var results []ExecuteResult
	for r := range resCh {
		results = append(results, r)
	}
	return results, g.Wait()

}

func (e *Executor) executeFunction(ctx context.Context, fc FunctionCall) (ExecuteResult, error) {
	tool, exists := e.tools[fc.Name]
	if !exists {
		return ExecuteResult{FunctionName: fc.Name, Error: fmt.Sprintf("function '%s' not found", fc.Name), Args: fc.Args}, nil
	}
	var argsMap map[string]interface{}
	_ = json.Unmarshal(fc.Args, &argsMap)
	if tool.StatusMessage != "" {
		server.SendFunctionStatus(e.userId, formatStatusMessage(tool.StatusMessage, argsMap))
	}
	_, span := e.tracer.Start(ctx, fc.Name, trace.WithAttributes(attribute.String("agent.tool", fc.Name)))
	defer span.End()

	result, err := tool.Function(e.conn, e.userId, fc.Args)
	if err != nil {
		span.RecordError(err)
		e.log.Warn("Error executing function", zap.String("function", fc.Name), zap.Error(err))
		return ExecuteResult{FunctionName: fc.Name, Error: err.Error(), Args: argsMap}, nil
	}
	return ExecuteResult{FunctionName: fc.Name, Result: result, Args: argsMap}, nil

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
