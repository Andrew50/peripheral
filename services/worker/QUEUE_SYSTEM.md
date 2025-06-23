# Strategy Queue System

A Redis-based queue system for executing trading strategy backtests and screening tasks.

## Overview

The queue system consists of:
- **Redis Queue**: `strategy_queue` - holds tasks to be processed
- **Worker Process**: Continuously processes tasks from the queue
- **Result Storage**: Task results stored in Redis with expiration

## Queue Architecture

```
Backend/API → Redis Queue → Worker → Redis Results
     ↓           ↓            ↓         ↓
  Add Task → strategy_queue → Process → task_result:{id}
```

## Task Types

### 1. Backtest Task
Tests a strategy against historical data for specified symbols and date range.

```python
{
    "task_id": "uuid-string",
    "task_type": "backtest", 
    "args": {
        "strategy_code": "def my_strategy(df): return instances",
        "symbols": ["AAPL", "MSFT", "GOOGL"],
        "start_date": "2024-01-01T00:00:00",  # ISO format
        "end_date": "2024-12-31T23:59:59"     # ISO format
    }
}
```

### 2. Screening Task
Runs a strategy across a universe of symbols to find current opportunities.

```python
{
    "task_id": "uuid-string", 
    "task_type": "screening",
    "args": {
        "strategy_code": "def my_strategy(df): return instances",
        "universe": ["AAPL", "MSFT", "GOOGL", "TSLA", "NVDA"],
        "limit": 10  # Max results to return
    }
}
```

## Adding Tasks to Queue

### From Python
```python
import redis
from worker import add_backtest_task, add_screening_task

redis_client = redis.Redis(host='cache', port=6379, decode_responses=True)

# Add backtest task
add_backtest_task(
    redis_client=redis_client,
    task_id="my-backtest-123",
    strategy_code="""
def gap_up_strategy(df):
    instances = []
    df_filtered = df[(df['gap_pct'] > 3.0) & (df['volume_ratio'] > 1.5)]
    
    for _, row in df_filtered.iterrows():
        instances.append({
            'ticker': row['ticker'],
            'date': str(row['date']),
            'signal': True,
            'gap_percent': row['gap_pct'],
            'message': f"{row['ticker']} gapped up {row['gap_pct']:.1f}%"
        })
    
    return instances
""",
    symbols=['AAPL', 'MSFT', 'GOOGL'],
    start_date='2024-01-01T00:00:00',
    end_date='2024-12-31T23:59:59'
)

# Add screening task
add_screening_task(
    redis_client=redis_client,
    task_id="my-screening-456",
    strategy_code=strategy_code,
    universe=['AAPL', 'MSFT', 'GOOGL', 'TSLA', 'NVDA'],
    limit=5
)
```

### From Go Backend
```go
taskData := map[string]interface{}{
    "task_id": taskID,
    "task_type": "backtest",
    "args": map[string]interface{}{
        "strategy_code": strategyCode,
        "symbols": symbols,
        "start_date": startDate.Format(time.RFC3339),
        "end_date": endDate.Format(time.RFC3339),
    },
}

taskJSON, _ := json.Marshal(taskData)
conn.Cache.LPush(ctx, "strategy_queue", taskJSON)
```

## Getting Task Results

### From Python
```python
from worker import get_task_result

result = get_task_result(redis_client, "my-backtest-123")
if result:
    status = result["status"]  # "running", "completed", "error"
    data = result["data"]      # Task-specific result data
    
    if status == "completed":
        instances = data["instances"]
        execution_time = data["execution_time_seconds"]
        print(f"Found {len(instances)} signals in {execution_time:.2f}s")
```

### From Go Backend
```go
resultJSON, err := conn.Cache.Get(ctx, fmt.Sprintf("task_result:%s", taskID)).Result()
if err == nil {
    var result map[string]interface{}
    json.Unmarshal([]byte(resultJSON), &result)
    
    status := result["status"]
    data := result["data"]
}
```

## Result Structure

### Successful Backtest Result
```json
{
    "status": "completed",
    "data": {
        "success": true,
        "execution_mode": "backtest",
        "instances": [
            {
                "ticker": "AAPL",
                "date": "2024-01-15",
                "signal": true,
                "gap_percent": 3.5,
                "message": "AAPL gapped up 3.5%"
            }
        ],
        "summary": {
            "total_instances": 42,
            "positive_signals": 38,
            "date_range": ["2024-01-01T00:00:00", "2024-12-31T23:59:59"],
            "symbols_processed": 3,
            "data_shape": [1000, 25]
        },
        "performance_metrics": {
            "total_return": 0.15,
            "win_rate": 0.68,
            "avg_return": 0.03
        },
        "execution_time_ms": 2500,
        "execution_time_seconds": 2.5,
        "worker_id": "worker-pod-abc123",
        "completed_at": "2024-01-15T10:30:00Z"
    },
    "updated_at": "2024-01-15T10:30:00Z"
}
```

### Successful Screening Result
```json
{
    "status": "completed", 
    "data": {
        "success": true,
        "execution_mode": "screening",
        "ranked_results": [
            {
                "ticker": "TSLA",
                "score": 0.95,
                "signal": true,
                "message": "TSLA momentum breakout"
            }
        ],
        "universe_size": 500,
        "results_returned": 5,
        "execution_time_seconds": 1.2,
        "worker_id": "worker-pod-def456", 
        "completed_at": "2024-01-15T10:30:00Z"
    },
    "updated_at": "2024-01-15T10:30:00Z"
}
```

### Error Result
```json
{
    "status": "error",
    "data": {
        "error": "Security validation failed: Code contains prohibited operations",
        "completed_at": "2024-01-15T10:30:00Z"
    },
    "updated_at": "2024-01-15T10:30:00Z"
}
```

## Running the Worker

### Development
```bash
cd services/worker
source venv/bin/activate
python worker.py
```

### Production (Docker)
```bash
docker run -d \
  -e REDIS_HOST=cache \
  -e REDIS_PORT=6379 \
  -e REDIS_PASSWORD=mysecret \
  worker:latest
```

### Kubernetes
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: strategy-worker
spec:
  replicas: 3
  selector:
    matchLabels:
      app: strategy-worker
  template:
    metadata:
      labels:
        app: strategy-worker
    spec:
      containers:
      - name: worker
        image: worker:latest
        env:
        - name: REDIS_HOST
          value: "cache"
        - name: REDIS_PORT  
          value: "6379"
        resources:
          requests:
            cpu: 500m
            memory: 1Gi
          limits:
            cpu: 2000m
            memory: 4Gi
```

## Testing

Run the test suite to verify the queue system:

```bash
cd services/worker
python test_queue_system.py
```

## Performance Characteristics

- **Queue Latency**: ~1-2ms to add/retrieve tasks
- **Task Processing**: Depends on strategy complexity and data size
  - Simple screening: 100-500ms
  - Small backtest (1 year, 10 symbols): 1-5 seconds  
  - Large backtest (2 years, 100 symbols): 10-30 seconds
- **Throughput**: ~10-50 tasks/minute per worker (strategy dependent)
- **Scaling**: Horizontal scaling via multiple worker replicas

## Monitoring

### Queue Metrics
- `LLEN strategy_queue` - Current queue depth
- `INFO keyspace` - Redis memory usage
- Worker logs for processing times and errors

### Autoscaling
Scale workers based on queue depth:
```yaml
# KEDA ScaledObject
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: strategy-worker-scaler
spec:
  scaleTargetRef:
    name: strategy-worker
  minReplicaCount: 1
  maxReplicaCount: 10
  triggers:
  - type: redis
    metadata:
      address: cache:6379
      listName: strategy_queue
      listLength: "5"
```

## Security

- Strategy code validation via AST parsing
- Restricted execution environment (no file/network access)
- Memory and time limits enforced
- Redis authentication and SSL in production 