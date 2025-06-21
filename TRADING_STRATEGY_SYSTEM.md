# AI Trading Strategy Infrastructure

## Overview

This system transforms natural language descriptions into executable Python trading strategies for backtesting, screening, and real-time alerting.

## Architecture

```
User Input → Gemini AI → Python Code → Validation → Database
                                                        ↓
Real-time Alerts ← Execution Engine ← Strategy Storage
Backtesting     ←                  ←
Screening       ←                  ←
```

## Strategy Creation Process

### 1. Natural Language Input
Users describe strategies in plain English:
- "Find stocks that gap up by more than 3% with high volume"
- "Identify oversold technology stocks with RSI below 30"

### 2. AI Code Generation
Gemini AI converts descriptions to Python:

```python
def classify_symbol(symbol):
    """Generated gap-up strategy"""
    try:
        price_data = get_price_data(symbol, timeframe='1d', days=5)
        current_open = price_data['open'][-1]
        previous_close = price_data['close'][-2]
        gap_percent = ((current_open - previous_close) / previous_close) * 100
        return gap_percent > 3.0
    except Exception:
        return False
```

### 3. Security Validation
- AST analysis for dangerous operations
- Pattern matching for prohibited code
- Execution sandboxing
- Resource limits (128MB memory, 30s CPU)

## Execution Modes

### Real-time Alerting
Continuous monitoring with instant notifications:
- Executes every second on active symbols
- Sends alerts via Telegram and WebSocket
- Logs all activity to database

### Backtesting
Historical performance analysis:
- Tests strategies against 2 years of data
- Calculates performance metrics
- Provides entry points and returns

### Screening
Universe-wide opportunity detection:
- Scans entire markets (S&P 500, NASDAQ, etc.)
- Ranks opportunities by strategy scores
- Updates results in real-time

## Data Access

Comprehensive market data through standardized functions:

```python
# Price data
get_price_data(symbol, timeframe='1d', days=30)

# Fundamentals
get_fundamental_data(symbol)

# Universe screening
scan_universe(filters={'min_market_cap': 1000000000})
```

## Performance Optimization

- PyPy-compatible code for maximum speed
- Multi-level caching (memory + Redis)
- Batch processing for multiple symbols
- Connection pooling for database access

## Security Features

- No file system access
- No network operations
- Restricted imports
- Resource monitoring
- Code validation

## API Examples

```http
# Create Strategy
POST /api/createStrategyFromPrompt
{
    "query": "Find momentum stocks with volume spikes",
    "strategyId": -1
}

# Run Backtest
POST /api/runBacktest
{
    "strategyId": 123,
    "start": 1640995200000
}

# Enable Alerts
POST /api/setAlert
{
    "strategyId": 123,
    "active": true
}
```

## Example Strategies

### Gap Detection
```python
def classify_symbol(symbol):
    price_data = get_price_data(symbol, timeframe='1d', days=5)
    current_open = price_data['open'][-1]
    previous_close = price_data['close'][-2]
    gap_percent = ((current_open - previous_close) / previous_close) * 100
    return gap_percent > 3.0
```

### Value Investing
```python
def classify_symbol(symbol):
    fundamentals = get_fundamental_data(symbol)
    pe_ratio = fundamentals.get('pe_ratio', float('inf'))
    market_cap = fundamentals.get('market_cap', 0)
    return pe_ratio < 15 and market_cap > 1000000000
```

## Key Benefits

1. **Natural Language Interface** - No coding required
2. **Multi-Mode Execution** - Alerts, backtests, screening
3. **High Performance** - Sub-millisecond execution
4. **Comprehensive Security** - Multiple validation layers
5. **Real-time Alerts** - Instant notifications
6. **Historical Analysis** - Complete backtesting
7. **Universe Screening** - Market-wide opportunity detection

This infrastructure provides a complete solution for AI-powered trading strategy development and execution. 