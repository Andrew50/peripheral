# Queue Package

This package provides a self-contained task queuing system that replaces the global worker monitor approach. Each queued task gets its own monitoring goroutine that handles retries, worker failure detection, and result streaming using a unified update channel.

## Features

- **Per-task monitoring**: Each task has its own watchdog goroutine
- **Unified update channel**: Single channel for both task updates and heartbeats using `update_id`
- **Heartbeat monitoring**: Real-time worker health monitoring via heartbeat messages
- **Automatic retries**: Failed tasks are automatically retried up to a configurable limit
- **Worker failure detection**: Detects dead workers via missed heartbeats
- **Progress streaming**: Real-time progress updates via channels
- **Cancellation support**: Tasks can be cancelled by callers
- **Timeout handling**: Tasks that run too long are automatically retried

## Architecture Changes

### Unified Update Channel

The system now uses a unified channel approach:

- **Channel Format**: `task_updates:{update_id}` 
- **Update ID**: Each task gets a unique `update_id` for all communications
- **Message Types**: 
  - Task status updates (`status: "running"`, `"completed"`, `"error"`)
  - Heartbeat messages (`type: "heartbeat"`)
  - Progress updates (`status: "progress"`)

### Heartbeat Monitoring

- **Worker Integration**: Python workers publish heartbeats to the unified channel
- **Interval Configuration**: Heartbeat interval is configurable per task (default: 5 seconds)
- **Failure Detection**: Missing 3 consecutive heartbeats triggers task retry
- **Task Lifecycle**: Heartbeat monitoring starts when task status changes to "running"

## Usage

### Basic Usage

```go
import "backend/internal/queue"

// Queue a task
handle, err := queue.Task(ctx, conn, "backtest", args, false, 3, 10*time.Minute)
if err != nil {
    return err
}

// Wait for completion
result, err := handle.Await(ctx)
if err != nil {
    return err
}

// Handle result
if result.Status == "completed" {
    // Process successful result
    fmt.Printf("Task completed: %+v\n", result.Data)
} else if result.Status == "error" {
    // Handle error
    fmt.Printf("Task failed: %s\n", result.Error)
}
```

### Typed Results (Recommended)

The queue system provides strongly-typed result handling to eliminate manual parsing:

```go
import "backend/internal/queue"

// Queue a backtest with typed result
result, err := queue.BacktestTyped(ctx, conn, args)
if err != nil {
    return err
}

// Handle typed result - no manual parsing needed!
// The result is directly typed, no need to check status
fmt.Printf("Backtest completed successfully!\n")
fmt.Printf("Strategy ID: %d\n", result.StrategyID)
fmt.Printf("Version: %d\n", result.Version) 
fmt.Printf("Total Instances: %d\n", result.Summary.TotalInstances)
fmt.Printf("Success Rate: %.2f%%\n", 
    float64(result.Summary.PositiveInstances)/float64(result.Summary.TotalInstances)*100)
```

### Progress Monitoring

```go
// Monitor progress updates (heartbeats are filtered out automatically)
go func() {
    for update := range handle.Updates {
        fmt.Printf("Progress: %s - %s\n", update.Status, update.Data)
        if update.Status == "completed" || update.Status == "error" {
            break
        }
    }
}()

// Wait for final result
result, err := handle.Await(ctx)
```

### Cancellation

```go
// Cancel a running task
err := handle.Cancel()
if err != nil {
    log.Printf("Failed to cancel task: %v", err)
}
```

### Convenience Functions

The package provides both untyped and typed convenience functions:

#### Untyped Functions (Return Handle)
```go
// Queue a backtest (10 minute timeout, 3 retries)
handle, err := queue.Backtest(ctx, conn, args)

// Queue a strategy creation (15 minute timeout, 2 retries, high priority)
handle, err := queue.CreateStrategy(ctx, conn, args)

// Queue a screening task (5 minute timeout, 3 retries)
handle, err := queue.Screening(ctx, conn, args)

// Queue an alert task (2 minute timeout, 3 retries)
handle, err := queue.Alert(ctx, conn, args)

// Queue a Python agent task (8 minute timeout, 3 retries)
handle, err := queue.PythonAgent(ctx, conn, args)
```

#### Typed Functions (Return Direct Result)
```go
// Typed functions automatically wait and return strongly-typed results
result, err := queue.BacktestTyped(ctx, conn, args)      // *BacktestResult
result, err := queue.CreateStrategyTyped(ctx, conn, args) // *CreateStrategyResult
result, err := queue.ScreeningTyped(ctx, conn, args)     // *ScreeningResult
result, err := queue.AlertTyped(ctx, conn, args)         // *AlertResult
result, err := queue.PythonAgentTyped(ctx, conn, args)   // *PythonAgentResult
```

## Typed Result Structures

### BacktestResult
```go
type BacktestResult struct {
    Success        bool                   `json:"success"`
    StrategyID     int                    `json:"strategy_id"`
    Version        int                    `json:"version"`
    Instances      []map[string]any       `json:"instances"`
    Summary        BacktestSummary        `json:"summary"`
    StrategyPrints string                 `json:"strategy_prints,omitempty"`
    StrategyPlots  []StrategyPlotData     `json:"strategy_plots,omitempty"`
    ResponseImages []string               `json:"response_images,omitempty"`
    ErrorMessage   string                 `json:"error_message,omitempty"`
}
```

### CreateStrategyResult
```go
type CreateStrategyResult struct {
    Success  bool      `json:"success"`
    Strategy *Strategy `json:"strategy,omitempty"`
    Error    string    `json:"error,omitempty"`
}
```

### Manual Typed Await
If you need more control over the waiting process, you can use the generic `AwaitTypedResult` function:

```go
// Queue a task and get a handle
handle, err := queue.Backtest(ctx, conn, args)
if err != nil {
    return err
}

// Monitor progress if needed
go func() {
    for update := range handle.Updates {
        fmt.Printf("Progress: %s\n", update.Status)
        if update.Status == "completed" || update.Status == "error" {
            break
        }
    }
}()

// Wait for typed result
result, err := queue.AwaitTypedResult[queue.BacktestResult](ctx, handle, nil)
if err != nil {
    return err
}

// Use the typed result directly
fmt.Printf("Backtest completed with %d instances\n", result.Summary.TotalInstances)
```

### ScreeningResult
```go
type ScreeningResult struct {
    Success       bool                     `json:"success"`
    RankedResults []map[string]interface{} `json:"ranked_results"`
    ErrorMessage  string                   `json:"error_message,omitempty"`
}
```

### AlertResult
```go
type AlertResult struct {
    Success      bool   `json:"success"`
    ErrorMessage string `json:"error_message,omitempty"`
}
```

### PythonAgentResult
```go
type PythonAgentResult struct {
    Success        bool     `json:"success"`
    Result         []any    `json:"result"`
    Prints         string   `json:"prints"`
    Plots          []any    `json:"plots"`
    ResponseImages []string `json:"responseImages"`
    ExecutionID    string   `json:"executionID"`
    Error          string   `json:"error,omitempty"`
}
```

## Manual Type Conversion

If you need to convert an untyped result to a typed one:

```go
// Get untyped result
handle, err := queue.Backtest(ctx, conn, args)
result, err := handle.Await(ctx)

// Convert to typed result
typedResult, err := queue.UnmarshalTypedResult[queue.BacktestResult](result)
if err != nil {
    return fmt.Errorf("failed to unmarshal result: %w", err)
}

// Now use strongly-typed data
fmt.Printf("Strategy ID: %d\n", typedResult.Result.StrategyID)
```

## Task Data Structure

Tasks now include additional fields for unified monitoring:

```json
{
  "task_id": "uuid-v4",
  "task_type": "backtest",
  "args": {...},
  "created_at": "2024-01-01T00:00:00Z",
  "priority": "normal",
  "update_id": "uuid-v4",
  "heartbeat_interval": 5
}
```

## Message Formats

### Task Status Update
```json
{
  "task_id": "task-uuid",
  "update_id": "update-uuid", 
  "status": "running",
  "result": {...},
  "updated_at": "2024-01-01T00:00:00Z",
  "worker_id": "worker_123"
}
```

### Heartbeat Message
```json
{
  "task_id": "task-uuid",
  "update_id": "update-uuid",
  "type": "heartbeat",
  "worker_id": "worker_123",
  "timestamp": "2024-01-01T00:00:00Z",
  "status": "alive",
  "heartbeat_interval": 5
}
```

### Progress Update
```json
{
  "task_id": "task-uuid",
  "update_id": "update-uuid",
  "status": "progress",
  "stage": "initialization",
  "message": "Fetching strategy code...",
  "data": {...},
  "updated_at": "2024-01-01T00:00:00Z"
}
```

## Components

### Go Queue System

1. **Task**: Main entry point that creates tasks and returns handles
2. **Handle**: Provides control over individual tasks (Updates channel, Cancel, Await)
3. **Watchdog**: Per-task monitoring goroutine that handles retries and heartbeat monitoring
4. **Progress Subscriber**: Subscribes to unified channel for real-time updates

### Python Worker Integration

1. **Task Parsing**: Extracts `update_id` and `heartbeat_interval` from task data
2. **Heartbeat Publishing**: Publishes heartbeats to `task_updates:{update_id}` channel
3. **Progress Updates**: All progress and status updates use the unified channel
4. **Backward Compatibility**: Also publishes to legacy `worker_task_updates` channel

## Redis Keys & Channels

- `strategy_queue`: Normal priority task queue
- `strategy_queue_priority`: High priority task queue  
- `task_result:{taskID}`: Task status and result storage
- `task_assignment:{taskID}`: Worker assignment tracking
- `worker_heartbeat:{workerID}`: Worker health monitoring (legacy)
- `task_updates:{updateID}`: **NEW** - Unified channel for task-specific updates and heartbeats

## Migration from Global Worker Monitor

The new system provides several improvements:

**Before:**
```go
// Manual task creation and Redis operations
taskJSON, _ := json.Marshal(task)
conn.Cache.RPush(ctx, "strategy_queue", taskJSON)

// Manual result polling with separate heartbeat monitoring
result, err := waitForResult(ctx, conn, taskID, timeout)
```

**After:**
```go
// Simplified task queuing with built-in unified monitoring
handle, err := queue.Backtest(ctx, conn, args)
result, err := handle.Await(ctx)
```

**Key Improvements:**
- **Unified monitoring**: Single channel for all task communications
- **Real-time heartbeats**: Instant detection of worker failures
- **Per-task isolation**: Each task has dedicated monitoring
- **Configurable intervals**: Heartbeat frequency can be tuned per task
- **Better error detection**: Faster failure detection and recovery

## Error Handling

The system handles various failure scenarios:

- **Worker crashes**: Detected via missing heartbeats within 3 intervals
- **Task timeouts**: Long-running tasks are retried with fresh workers
- **Network issues**: Connection problems trigger retries
- **Max retries exceeded**: Tasks are marked as permanently failed
- **Heartbeat failures**: Tasks that don't start sending heartbeats are retried

## Configuration

Task parameters can be customized:

- **maxRetries**: Number of retry attempts (default: 3)
- **timeout**: Maximum task execution time (varies by task type)
- **priority**: Whether to use priority queue (default: false)
- **heartbeatInterval**: Heartbeat frequency in seconds (default: 5)

## Testing

Run tests with:
```bash
go test ./internal/queue
```

Note: Full integration tests require a Redis instance.

## Monitoring & Debugging

The unified channel approach provides better observability:

```bash
# Monitor all updates for a specific task
redis-cli SUBSCRIBE task_updates:your-update-id

# Monitor legacy channel for backward compatibility
redis-cli SUBSCRIBE worker_task_updates
```

Heartbeat messages are automatically filtered from the user-facing Updates channel but are logged for debugging purposes. 