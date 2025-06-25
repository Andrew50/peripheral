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
The system uses **Gemini 2.5 Flash** to convert descriptions to Python code using the **NEW ACCESSOR PATTERN**:

```python
def strategy():
    """Generated strategy for gap-up detection using NEW ACCESSOR PATTERN"""
    instances = []
    
    # Get recent bar data using accessor function
    bar_data = get_bar_data(
        timeframe="1d",
        columns=["ticker", "timestamp", "open", "close", "volume"],
        min_bars=2
    )
    
    if len(bar_data) == 0:
        return instances
    
    # Convert to DataFrame for processing
    import pandas as pd
    df = pd.DataFrame(bar_data, columns=["ticker", "timestamp", "open", "close", "volume"])
    df['date'] = pd.to_datetime(df['timestamp'], unit='s').dt.date
    df = df.sort_values(['ticker', 'date']).copy()
    
    # Calculate gaps for all tickers
    df['prev_close'] = df.groupby('ticker')['close'].shift(1)
    df['gap_percent'] = ((df['open'] - df['prev_close']) / df['prev_close']) * 100
    
    # Calculate volume ratio
    df['avg_volume_4d'] = df.groupby('ticker')['volume'].rolling(4).mean().reset_index(0, drop=True).shift(1)
    df['volume_ratio'] = df['volume'] / df['avg_volume_4d']
    
    # Filter for gap up with volume criteria
    gap_ups = df[(df['gap_percent'] > 3.0) & (df['volume_ratio'] > 1.5)].dropna()
    
    for _, row in gap_ups.iterrows():
        instances.append({
            'ticker': row['ticker'],
            'timestamp': str(row['date']),
            'signal': True,
            'gap_percent': round(row['gap_percent'], 2),
            'volume_ratio': round(row['volume_ratio'], 2),
            'message': f"{row['ticker']} gapped up {row['gap_percent']:.2f}% with {row['volume_ratio']:.1f}x volume"
        })
    
    return instances
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
def strategy():
    """Complete value investing strategy using NEW ACCESSOR PATTERN"""
    instances = []
    
    # Get current bar data for all securities
    bar_data = get_bar_data(
        timeframe="1d",
        columns=["ticker", "timestamp", "close"],
        min_bars=1
    )
    
    # Get fundamental data for all securities
    fundamentals = get_general_data(columns=["pe_ratio", "pb_ratio", "market_cap", "debt_to_equity"])
    
    if len(bar_data) == 0 or fundamentals.empty:
        return instances
    
    # Convert to DataFrame for processing
    import pandas as pd
    df = pd.DataFrame(bar_data, columns=["ticker", "timestamp", "close"])
    
    # Merge with fundamentals
    df = df.merge(fundamentals, left_on='ticker', right_index=True, how='inner')
    
    # Value criteria filtering
    value_stocks = df[
        (df['pe_ratio'] < 15) & 
        (df['pb_ratio'] < 1.5) & 
        (df['market_cap'] > 1000000000) & 
        (df['debt_to_equity'] < 0.5)
    ].dropna()
    
    for _, row in value_stocks.iterrows():
        instances.append({
            'ticker': row['ticker'],
            'timestamp': str(pd.to_datetime(row['timestamp'], unit='s').date()),
            'signal': True,
            'entry_price': row['close'],
            'pe_ratio': row['pe_ratio'],
            'expected_return': (20 - row['pe_ratio']) * 0.02,
            'message': f"{row['ticker']} value opportunity: PE {row['pe_ratio']:.1f}"
        })
    
    return instances
```

#### Momentum Screening Strategy
```python
def strategy():
    """Complete momentum screening using NEW ACCESSOR PATTERN"""
    instances = []
    
    # Get historical bar data for momentum calculation
    bar_data = get_bar_data(
        timeframe="1d",
        columns=["ticker", "timestamp", "close", "volume"],
        min_bars=30
    )
    
    if len(bar_data) == 0:
        return instances
    
    # Convert to DataFrame for processing
    import pandas as pd
    df = pd.DataFrame(bar_data, columns=["ticker", "timestamp", "close", "volume"])
    df['date'] = pd.to_datetime(df['timestamp'], unit='s').dt.date
    df = df.sort_values(['ticker', 'date']).copy()
    
    # Group by ticker for momentum calculations
    results = []
    for ticker, group in df.groupby('ticker'):
        if len(group) < 21:  # Need at least 21 days for 20-day return
            continue
        
        group = group.sort_values('date').reset_index(drop=True)
        
        # Momentum calculations
        current_price = group['close'].iloc[-1]
        price_5d_ago = group['close'].iloc[-6] if len(group) >= 6 else None
        price_20d_ago = group['close'].iloc[-21] if len(group) >= 21 else None
        
        if price_5d_ago is None or price_20d_ago is None:
            continue
        
        returns_5d = (current_price / price_5d_ago) - 1
        returns_20d = (current_price / price_20d_ago) - 1
        
        # Volume analysis
        recent_volumes = group['volume'].tail(10)
        avg_volume = recent_volumes.mean()
        current_volume = group['volume'].iloc[-1]
        volume_ratio = current_volume / avg_volume if avg_volume > 0 else 0
        
        # Combined momentum score
        momentum_score = (returns_5d * 2) + returns_20d + (volume_ratio * 0.1)
        
        if momentum_score > 0.05:
            instances.append({
                'ticker': ticker,
                'timestamp': str(group['date'].iloc[-1]),
                'signal': True,
                'score': momentum_score,
                'returns_5d': returns_5d,
                'returns_20d': returns_20d,
                'volume_ratio': volume_ratio,
                'message': f"{ticker} momentum score: {momentum_score:.3f}"
            })
    
    return instances
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