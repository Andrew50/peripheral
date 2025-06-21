# Trading Strategy System - Complete Infrastructure Guide

## 🎯 Overview

This document provides a comprehensive overview of the AI-powered trading strategy infrastructure. The system converts natural language descriptions into executable Python code and supports three execution modes through dedicated worker functions.

## 🏗️ System Architecture

### Core Components

1. **Strategy Creation Engine** - AI-powered natural language to Python code generation
2. **Worker Functions** - Three dedicated functions for complete strategy execution:
   - `run_backtest(strategy_id)` - Complete historical backtesting
   - `run_screener(strategy_id)` - Complete universe screening  
   - `run_alert(strategy_id)` - Complete real-time monitoring
3. **Security Layer** - Multi-layer validation and sandboxed execution
4. **Data Access Layer** - Comprehensive market data and fundamental analysis

### New Worker Architecture

The system now uses three dedicated worker functions that handle complete operations:

```python
# Complete Backtest - No More Batching
async def run_backtest(strategy_id: int, **kwargs) -> Dict[str, Any]:
    """
    Runs complete backtest across ALL historical days (default: 2 years)
    - Processes all symbols in universe
    - Calculates performance metrics
    - Returns complete results in single call
    """

# Complete Screening - Universe-Wide
async def run_screener(strategy_id: int, **kwargs) -> Dict[str, Any]:
    """
    Runs complete screening across ALL tickers in universe
    - Processes entire market (S&P 500, NASDAQ, etc.)
    - Ranks opportunities by strategy scores
    - Returns top-ranked results
    """

# Complete Alert Monitoring - Real-Time
async def run_alert(strategy_id: int, **kwargs) -> Dict[str, Any]:
    """
    Runs complete alert monitoring across ALL tickers
    - Monitors entire universe in real-time
    - Generates alerts when criteria met
    - Returns all alerts and signals
    """
```

## 🚀 Strategy Creation Process

### 1. Natural Language Input
Users describe strategies in plain English:

```
"Find stocks that gap up by more than 3% with high volume"
"Identify value stocks with P/E ratio under 15 and strong balance sheets"
"Alert me when ARM gaps up by 5% or more"
```

### 2. AI Code Generation
The system uses **Gemini 2.5 Flash** to convert descriptions to Python code:

```python
def classify_symbol(symbol):
    """Generated strategy for gap-up detection"""
    try:
        # Get current and previous day data
        price_data = get_price_data(symbol, timeframe='1d', days=2)
        if not price_data.get('close') or len(price_data['close']) < 2:
            return False
        
        current_open = price_data['open'][-1]
        prev_close = price_data['close'][-2]
        
        # Calculate gap percentage
        gap_percent = ((current_open - prev_close) / prev_close) * 100
        
        # Check volume spike
        volume_data = price_data['volume']
        avg_volume = sum(volume_data[-5:-1]) / 4  # 4-day average
        current_volume = volume_data[-1]
        volume_ratio = current_volume / avg_volume if avg_volume > 0 else 0
        
        # Gap up criteria: >3% gap with >1.5x volume
        return gap_percent > 3.0 and volume_ratio > 1.5
        
    except Exception:
        return False
```

### 3. Complete Execution Modes

#### Backtesting - Complete Historical Analysis
```python
# Execute complete backtest
result = await run_backtest(strategy_id=123)

# Returns comprehensive results
{
    "success": true,
    "strategy_id": 123,
    "execution_mode": "backtest",
    "instances": [
        {
            "ticker": "AAPL",
            "timestamp": 1672531200000,
            "classification": true,
            "entry_price": 150.25,
            "future_return": 0.08  # 8% return
        }
    ],
    "summary": {
        "total_instances": 45,
        "positive_signals": 32,
        "date_range": ["2022-01-01", "2024-01-01"],
        "symbols_processed": 500
    },
    "performance_metrics": {
        "average_return": 0.045,
        "hit_rate": 0.71,
        "sharpe_ratio": 1.23,
        "max_return": 0.34,
        "positive_return_rate": 0.68
    }
}
```

#### Screening - Complete Universe Analysis
```python
# Execute complete screening
result = await run_screener(strategy_id=123, limit=50)

# Returns ranked opportunities
{
    "success": true,
    "strategy_id": 123,
    "execution_mode": "screening",
    "ranked_results": [
        {
            "symbol": "NVDA",
            "score": 0.89,
            "current_price": 875.30,
            "sector": "Technology",
            "data": {
                "gap_percent": 4.2,
                "volume_ratio": 2.1
            }
        }
    ],
    "scores": {"NVDA": 0.89, "AAPL": 0.76},
    "universe_size": 1000
}
```

#### Alerts - Complete Real-Time Monitoring
```python
# Execute complete alert monitoring
result = await run_alert(strategy_id=123)

# Returns all alerts and signals
{
    "success": true,
    "strategy_id": 123,
    "execution_mode": "alert",
    "alerts": [
        {
            "symbol": "ARM",
            "type": "strategy_signal",
            "message": "ARM triggered strategy alert",
            "timestamp": "2024-01-15T09:35:00Z",
            "priority": "high",
            "data": {
                "current_price": 142.50,
                "daily_change": 5.2
            }
        }
    ],
    "signals": {
        "ARM": {
            "signal": true,
            "timestamp": "2024-01-15T09:35:00Z",
            "current_price": 142.50
        }
    },
    "symbols_monitored": 1000
}
```

## 📊 Data Access Layer

### Comprehensive Market Data
```python
# Price data with flexible timeframes
get_price_data(symbol, timeframe='1d', days=30)
# Returns: timestamps, open, high, low, close, volume

# Multiple symbols efficiently  
get_multiple_symbols_data(symbols, timeframe='1d', days=30)

# Fundamental data
get_fundamental_data(symbol)
# Returns: market_cap, pe_ratio, eps, revenue, etc.

# Universe screening
scan_universe(filters={'min_market_cap': 1000000000}, limit=100)
```

### Advanced Strategy Examples

#### Multi-Factor Value Strategy
```python
def run_batch_backtest(start_date, end_date, symbols):
    """Complete value investing backtest"""
    instances = []
    
    # Screen entire universe for value criteria
    for symbol in symbols:
        fundamentals = get_fundamental_data(symbol)
        price_data = get_price_data(symbol, timeframe='1d', days=1)
        
        if not fundamentals or not price_data.get('close'):
            continue
            
        # Value metrics
        pe_ratio = fundamentals.get('pe_ratio', float('inf'))
        pb_ratio = fundamentals.get('pb_ratio', float('inf'))
        market_cap = fundamentals.get('market_cap', 0)
        debt_to_equity = fundamentals.get('debt_to_equity', float('inf'))
        
        # Value criteria
        if (pe_ratio < 15 and pb_ratio < 1.5 and 
            market_cap > 1000000000 and debt_to_equity < 0.5):
            
            instances.append({
                'ticker': symbol,
                'timestamp': int(datetime.utcnow().timestamp() * 1000),
                'classification': True,
                'entry_price': price_data['close'][-1],
                'pe_ratio': pe_ratio,
                'expected_return': (20 - pe_ratio) * 0.02
            })
    
    return {
        'instances': instances,
        'performance_metrics': {
            'total_picks': len(instances),
            'avg_pe': sum(i['pe_ratio'] for i in instances) / len(instances)
        }
    }
```

#### Momentum Screening Strategy
```python
def run_screening(universe, limit):
    """Complete momentum screening"""
    scored_symbols = []
    
    for symbol in universe:
        price_data = get_price_data(symbol, timeframe='1d', days=30)
        if not price_data.get('close') or len(price_data['close']) < 20:
            continue
        
        # Momentum calculations
        prices = price_data['close']
        volumes = price_data['volume']
        
        returns_5d = (prices[-1] / prices[-6]) - 1
        returns_20d = (prices[-1] / prices[-21]) - 1
        
        # Volume analysis
        avg_volume = sum(volumes[-10:]) / 10
        volume_ratio = volumes[-1] / avg_volume
        
        # Combined momentum score
        momentum_score = (returns_5d * 2) + returns_20d + (volume_ratio * 0.1)
        
        if momentum_score > 0.05:
            scored_symbols.append({
                'symbol': symbol,
                'score': momentum_score,
                'returns_5d': returns_5d,
                'returns_20d': returns_20d,
                'volume_ratio': volume_ratio
            })
    
    # Sort and return top results
    scored_symbols.sort(key=lambda x: x['score'], reverse=True)
    return {'ranked_results': scored_symbols[:limit]}
```

## 🔒 Security Implementation

### Multi-Layer Validation
1. **AST Analysis** - Parses Python code for dangerous constructs
2. **Pattern Matching** - Blocks file system, network, and subprocess operations
3. **Execution Sandboxing** - Isolated environment with resource limits
4. **Resource Limits** - 128MB memory, 30-second CPU timeout

### Code Validation Example
```python
# ❌ Blocked - File system access
open('/etc/passwd', 'r')

# ❌ Blocked - Network operations  
import requests

# ❌ Blocked - Subprocess execution
subprocess.call(['rm', '-rf', '/'])

# ✅ Allowed - Market data access
price_data = get_price_data('AAPL', timeframe='1d', days=30)
```

## 🚀 Performance Optimizations

### High-Performance Execution
- **PyPy Compatibility** - 10-100x faster than CPython
- **Native Data Structures** - Lists/dicts instead of pandas DataFrames
- **Multi-level Caching** - Memory + Redis for data persistence
- **Batch Processing** - Efficient bulk data operations

### Expected Performance
- **Sub-millisecond** strategy execution
- **Thousands of symbols** processed simultaneously  
- **Real-time** market data processing
- **Complete operations** instead of batched execution

## 🎛️ API Integration

### Backend Integration
```go
// Go backend handlers
func RunBacktest(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
    // Call worker's run_backtest function
    result, err := callWorkerBacktest(args.StrategyID)
    return convertToBacktestResponse(result), err
}

func RunScreening(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
    // Call worker's run_screener function  
    result, err := callWorkerScreening(args.StrategyID, args.Universe, args.Limit)
    return convertToScreeningResponse(result), err
}

func RunAlert(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
    // Call worker's run_alert function
    result, err := callWorkerAlert(args.StrategyID, args.Symbols)
    return convertToAlertResponse(result), err
}
```

### HTTP Worker Server
```python
# Worker HTTP endpoints
POST /execute
{
    "function": "run_backtest",
    "strategy_id": 123,
    "start_date": "2022-01-01",
    "end_date": "2024-01-01"
}

POST /execute  
{
    "function": "run_screener",
    "strategy_id": 123,
    "universe": ["AAPL", "MSFT", "GOOGL"],
    "limit": 50
}

POST /execute
{
    "function": "run_alert", 
    "strategy_id": 123,
    "symbols": ["ARM", "NVDA", "AAPL"]
}
```

## 🔄 System Flow

### Complete Strategy Execution Flow
1. **Strategy Creation** - Natural language → AI → Python code → Database
2. **Function Selection** - Choose run_backtest, run_screener, or run_alert
3. **Complete Execution** - Process ALL data (no batching)
4. **Result Processing** - Comprehensive analysis and metrics
5. **Response Delivery** - Complete results in single response

### Key Improvements
- ✅ **No More Batching** - Complete operations in single calls
- ✅ **Unified Interface** - Three simple functions handle all complexity
- ✅ **Complete Results** - Full historical data, entire universe coverage
- ✅ **Real-Time Processing** - Instant alerts across all monitored symbols
- ✅ **Database Integration** - Direct strategy and universe retrieval
- ✅ **Performance Optimized** - Efficient bulk processing

## 🎯 Usage Examples

### Create and Backtest Strategy
```python
# 1. Create strategy from natural language
strategy = await create_strategy_from_prompt(
    "Find stocks with RSI below 30 and volume spike above 2x average"
)

# 2. Run complete backtest (2 years of data)
backtest_result = await run_backtest(strategy.strategy_id)
print(f"Found {len(backtest_result['instances'])} opportunities")
print(f"Average return: {backtest_result['performance_metrics']['average_return']:.2%}")

# 3. Screen current market
screening_result = await run_screener(strategy.strategy_id, limit=20)
print(f"Top opportunities: {[r['symbol'] for r in screening_result['ranked_results'][:5]]}")

# 4. Set up real-time alerts
alert_result = await run_alert(strategy.strategy_id)
print(f"Monitoring {alert_result['symbols_monitored']} symbols")
```

This new architecture provides a complete, unified approach to strategy execution with maximum performance and simplicity. 