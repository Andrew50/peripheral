# Strategy Worker Architecture - Three Function System

## 🎯 Overview

The strategy infrastructure has been completely redesigned around three dedicated worker functions that handle complete operations without batching. This new architecture eliminates the complexity of batched processing and provides unified interfaces for all strategy execution modes.

## 🏗️ New Architecture

### Three Core Functions

```python
# 1. Complete Backtesting
async def run_backtest(strategy_id: int, **kwargs) -> Dict[str, Any]:
    """
    Runs complete backtest across ALL historical days (default: 2 years)
    - No more batching - processes all data in one call
    - Covers entire date range and all symbols
    - Returns comprehensive performance metrics
    """

# 2. Complete Screening  
async def run_screener(strategy_id: int, **kwargs) -> Dict[str, Any]:
    """
    Runs complete screening across ALL tickers in universe
    - Processes entire market (S&P 500, NASDAQ, etc.)
    - Ranks all opportunities by strategy scores
    - Returns top-ranked results
    """

# 3. Complete Alert Monitoring
async def run_alert(strategy_id: int, **kwargs) -> Dict[str, Any]:
    """
    Runs complete alert monitoring across ALL tickers
    - Monitors entire universe in real-time
    - Generates alerts when criteria met across all symbols
    - Returns all alerts and signals in one response
    """
```

## 🚀 Key Improvements

### ✅ No More Batching
- **Before**: Backtests split into multiple batches, requiring complex coordination
- **After**: Single function call processes all historical data at once

### ✅ Complete Operations
- **Before**: Partial results requiring multiple API calls
- **After**: Comprehensive results in single response

### ✅ Unified Interface
- **Before**: Complex batching logic scattered across backend
- **After**: Three simple functions handle all complexity internally

### ✅ Database Integration
- **Before**: Mock data and hardcoded universes
- **After**: Direct database connectivity for strategies and universes

## 📋 Function Specifications

### run_backtest(strategy_id, **kwargs)

**Purpose**: Complete historical backtesting across all days

**Parameters**:
- `strategy_id` (int): Database ID of strategy to backtest
- `start_date` (optional): Start date for backtest (default: 2 years ago)
- `end_date` (optional): End date for backtest (default: today)
- `symbols` (optional): Symbol list (default: all active securities)

**Returns**:
```json
{
    "success": true,
    "strategy_id": 123,
    "execution_mode": "backtest",
    "instances": [...],  // All backtest instances
    "summary": {
        "total_instances": 45,
        "positive_signals": 32,
        "symbols_processed": 500,
        "date_range": ["2022-01-01", "2024-01-01"]
    },
    "performance_metrics": {
        "average_return": 0.045,
        "hit_rate": 0.71,
        "sharpe_ratio": 1.23
    }
}
```

### run_screener(strategy_id, **kwargs)

**Purpose**: Complete universe screening and ranking

**Parameters**:
- `strategy_id` (int): Database ID of strategy to screen
- `universe` (optional): Symbol list (default: all active securities)
- `limit` (optional): Max results to return (default: 100)

**Returns**:
```json
{
    "success": true,
    "strategy_id": 123,
    "execution_mode": "screening",
    "ranked_results": [
        {
            "symbol": "NVDA",
            "score": 0.89,
            "current_price": 875.30,
            "sector": "Technology"
        }
    ],
    "scores": {"NVDA": 0.89, "AAPL": 0.76},
    "universe_size": 1000
}
```

### run_alert(strategy_id, **kwargs)

**Purpose**: Complete real-time alert monitoring

**Parameters**:
- `strategy_id` (int): Database ID of strategy to monitor
- `symbols` (optional): Symbol list (default: all active securities)
- `alert_threshold` (optional): Minimum score for alerts

**Returns**:
```json
{
    "success": true,
    "strategy_id": 123,
    "execution_mode": "alert",
    "alerts": [
        {
            "symbol": "ARM",
            "type": "strategy_signal",
            "message": "ARM triggered strategy alert",
            "priority": "high"
        }
    ],
    "signals": {
        "ARM": {"signal": true, "timestamp": "2024-01-15T09:35:00Z"}
    },
    "symbols_monitored": 1000
}
```

## 🔧 Implementation Details

### Worker HTTP Server
```python
# services/worker/src/worker_server.py
class WorkerServer:
    """HTTP server exposing the three worker functions"""
    
    def start(self, host='localhost', port=8080):
        """Start HTTP server on specified host/port"""
        
    # Endpoints:
    # POST /execute - Execute any of the three functions
    # GET /health - Health check
```

### Backend Integration
```go
// services/backend/internal/app/strategy/strategies.go

// New function handlers that call worker
func RunBacktest(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error)
func RunScreening(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) 
func RunAlert(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error)

// HTTP routing in services/backend/internal/server/http.go
"run_backtest":  wrapContextFunc(strategy.RunBacktest),
"run_screening": strategy.RunScreening,
"run_alert":     strategy.RunAlert,
```

### Database Connectivity
```python
# services/worker/src/strategy_worker.py
class StrategyWorker:
    async def _get_strategy(self, strategy_id: int) -> Optional[Dict]:
        """Retrieve strategy from database"""
        query = "SELECT * FROM strategies WHERE strategyid = %s"
        result = await self.data_provider.execute_sql_parameterized(query, [strategy_id])
        
    async def _get_universe(self, universe_type: str = 'default') -> List[str]:
        """Retrieve active securities from database"""
        query = "SELECT ticker FROM securities WHERE active = true"
        result = await self.data_provider.execute_sql_parameterized(query, [])
```

## 🎛️ Usage Examples

### Frontend Integration
```javascript
// Complete backtest
const backtestResult = await queueRequest('run_backtest', {
    strategyId: 123
});

// Complete screening  
const screeningResult = await queueRequest('run_screening', {
    strategyId: 123,
    limit: 50
});

// Complete alert monitoring
const alertResult = await queueRequest('run_alert', {
    strategyId: 123
});
```

### Python Direct Usage
```python
# Import worker functions
from strategy_worker import run_backtest, run_screener, run_alert

# Execute complete operations
backtest_result = await run_backtest(strategy_id=123)
screening_result = await run_screener(strategy_id=123, limit=50)
alert_result = await run_alert(strategy_id=123)
```

## 🚀 Performance Benefits

### Before (Batched Architecture)
- Multiple API calls required for complete results
- Complex batch coordination logic
- Partial results requiring aggregation
- Higher latency due to multiple round trips

### After (Three-Function Architecture)
- Single API call for complete results
- Simplified execution logic
- Comprehensive results in one response
- Lower latency with bulk processing

## 🔄 Migration Path

### Existing Code Compatibility
The new functions maintain backward compatibility while providing enhanced functionality:

1. **Existing `run_backtest`** - Now calls worker's `run_backtest` internally
2. **New `run_screening`** - Replaces manual screening implementations
3. **New `run_alert`** - Consolidates alert monitoring logic

### Deployment Steps
1. Deploy worker with HTTP server
2. Update backend to call worker functions
3. Update frontend to use new endpoints
4. Remove old batching logic

This architecture provides a clean, unified approach to strategy execution with maximum performance and simplicity. 