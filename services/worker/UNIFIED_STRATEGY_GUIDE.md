# Unified Strategy Engine Guide

## Overview

The Unified Strategy Engine allows you to write **one strategy** that automatically works across **three execution modes**:

- 🚨 **Real-time**: Live market alerts and signal generation
- 📈 **Backtest**: Historical performance analysis  
- 🔍 **Screening**: Ranking and filtering large universes of stocks

## Key Benefits

### ✅ Write Once, Run Everywhere
- Single codebase works in all three modes
- No need to maintain separate implementations
- Consistent logic across all use cases

### ⚡ Optimized Execution
- Real-time mode: Fast alerts with minimal latency
- Backtest mode: Efficient bulk data processing
- Screening mode: Parallel universe analysis

### 🔄 Automatic Mode Detection
- Engine automatically detects which mode to use
- Optimizes data loading and execution accordingly
- Fallback mechanisms for compatibility

## Strategy Structure

### Core Functions

Your strategy can implement one or more of these functions:

```python
def run_realtime_scan(symbols):
    """Real-time mode: Generate live alerts"""
    alerts = []
    signals = {}
    # Your real-time logic here
    return {'alerts': alerts, 'signals': signals}

def run_batch_backtest(start_date, end_date, symbols):
    """Backtest mode: Analyze historical performance"""
    instances = []
    performance_metrics = {}
    # Your backtest logic here
    return {'instances': instances, 'performance_metrics': performance_metrics}

def run_screening(universe, limit):
    """Screening mode: Rank and filter stocks"""
    ranked_results = []
    scores = {}
    # Your screening logic here
    return {'ranked_results': ranked_results, 'scores': scores}

def classify_symbol(symbol):
    """Fallback: Simple true/false classification"""
    # Your classification logic here
    return True  # or False

def score_symbol(symbol):
    """Fallback: Numerical scoring for ranking"""
    # Your scoring logic here
    return 0.5  # Any numerical score
```

### Available Data Functions

All modes have access to these data functions:

```python
# Price and market data
price_data = get_price_data(symbol, timeframe='1d', days=30)
fundamentals = get_fundamental_data(symbol)
universe_data = scan_universe(filters={'min_market_cap': 1000000000})

# Utility functions
log("Strategy message")  # Logging
save_result("key", value)  # Save custom results

# Advanced functions (see execution_engine.py for full list)
similar_stocks = find_pairs(symbol, "sector")
correlation = check_correlation("AAPL", "MSFT", 30)
```

## Execution Modes

### 1. Real-time Mode 🚨

**Purpose**: Generate live trading alerts and signals

**Input**: List of symbols to monitor
**Output**: Alerts and current signals

**Example**:
```python
def run_realtime_scan(symbols):
    alerts = []
    signals = {}
    
    for symbol in symbols:
        price_data = get_price_data(symbol, timeframe='1h', days=1)
        
        # Check for momentum breakout
        if detect_momentum_breakout(price_data):
            alerts.append({
                'symbol': symbol,
                'type': 'momentum_breakout',
                'message': f'{symbol} showing strong momentum',
                'strength': calculate_signal_strength(price_data)
            })
            
            signals[symbol] = {
                'signal': True,
                'timestamp': datetime.now().isoformat()
            }
    
    return {'alerts': alerts, 'signals': signals}
```

**Best Practices**:
- Keep execution fast (< 30 seconds)
- Limit to essential symbols only
- Use shorter timeframes for responsiveness
- Focus on actionable signals

### 2. Backtest Mode 📈

**Purpose**: Analyze historical performance and validate strategies

**Input**: Date range and symbol universe
**Output**: Historical instances and performance metrics

**Example**:
```python
def run_batch_backtest(start_date, end_date, symbols):
    instances = []
    
    # Get universe with filters
    universe_data = scan_universe(
        filters={'min_market_cap': 1000000000},
        limit=500
    )
    
    for symbol_data in universe_data['data']:
        symbol = symbol_data['ticker']
        
        # Apply strategy logic
        fundamentals = get_fundamental_data(symbol)
        if meets_value_criteria(fundamentals):
            instances.append({
                'ticker': symbol,
                'timestamp': int(datetime.now().timestamp() * 1000),
                'classification': True,
                'entry_criteria': analyze_entry_point(symbol),
                'expected_performance': estimate_returns(fundamentals)
            })
    
    # Calculate performance metrics
    performance_metrics = {
        'total_picks': len(instances),
        'win_rate': calculate_win_rate(instances),
        'average_return': calculate_average_return(instances)
    }
    
    return {
        'instances': instances,
        'performance_metrics': performance_metrics
    }
```

**Best Practices**:
- Use larger time windows for statistical significance
- Include comprehensive performance metrics
- Test multiple time periods
- Validate with out-of-sample data

### 3. Screening Mode 🔍

**Purpose**: Rank and filter large universes of stocks

**Input**: Universe of symbols and ranking limit
**Output**: Ranked results with scores

**Example**:
```python
def run_screening(universe, limit):
    scored_symbols = []
    scores = {}
    
    for symbol in universe:
        # Multi-factor scoring
        price_data = get_price_data(symbol, timeframe='1d', days=60)
        fundamentals = get_fundamental_data(symbol)
        
        # Calculate composite score
        momentum_score = calculate_momentum(price_data)
        value_score = calculate_value_metrics(fundamentals)
        quality_score = calculate_quality_metrics(fundamentals)
        
        composite_score = (
            momentum_score * 0.4 +
            value_score * 0.3 +
            quality_score * 0.3
        )
        
        if composite_score > 0.5:  # Minimum threshold
            scored_symbols.append({
                'symbol': symbol,
                'score': composite_score,
                'momentum': momentum_score,
                'value': value_score,
                'quality': quality_score
            })
            scores[symbol] = composite_score
    
    # Sort by score and return top results
    scored_symbols.sort(key=lambda x: x['score'], reverse=True)
    ranked_results = scored_symbols[:limit]
    
    return {
        'ranked_results': ranked_results,
        'scores': scores
    }
```

**Best Practices**:
- Use multi-factor scoring for robustness
- Apply quality filters to reduce noise
- Return comprehensive ranking metadata
- Consider sector/industry balancing

## Example Strategies

### Momentum Strategy

```python
def run_realtime_scan(symbols):
    """Real-time momentum alerts"""
    alerts = []
    for symbol in symbols:
        price_data = get_price_data(symbol, '1h', 2)
        if detect_breakout(price_data):
            alerts.append(create_momentum_alert(symbol, price_data))
    return {'alerts': alerts}

def run_batch_backtest(start_date, end_date, symbols):
    """Momentum backtest"""
    instances = []
    for symbol in symbols:
        if momentum_entry_signal(symbol, start_date):
            instances.append(create_backtest_instance(symbol))
    return {'instances': instances}

def run_screening(universe, limit):
    """Momentum screening"""
    results = []
    for symbol in universe:
        score = calculate_momentum_score(symbol)
        if score > 0.7:
            results.append({'symbol': symbol, 'score': score})
    results.sort(key=lambda x: x['score'], reverse=True)
    return {'ranked_results': results[:limit]}
```

### Value Strategy

```python
def run_realtime_scan(symbols):
    """Value opportunity alerts"""
    alerts = []
    for symbol in symbols:
        if value_opportunity_detected(symbol):
            alerts.append(create_value_alert(symbol))
    return {'alerts': alerts}

def run_batch_backtest(start_date, end_date, symbols):
    """Value investing backtest"""
    instances = []
    universe = scan_universe({'min_market_cap': 1000000000})
    
    for symbol_data in universe['data']:
        if meets_value_criteria(symbol_data):
            instances.append(create_value_instance(symbol_data))
    
    return {'instances': instances}

def run_screening(universe, limit):
    """Value screening"""
    results = []
    for symbol in universe:
        value_score = calculate_value_score(symbol)
        if value_score > 0.6:
            results.append({'symbol': symbol, 'score': value_score})
    results.sort(key=lambda x: x['score'], reverse=True)
    return {'ranked_results': results[:limit]}
```

## Integration with Backend

### Go Handler Usage

```go
// Execute unified strategy
result, err := strategy.RunUnifiedStrategy(ctx, conn, userID, json.RawMessage(`{
    "strategyId": 123,
    "executionMode": "realtime",
    "symbols": ["AAPL", "MSFT", "GOOGL"]
}`))

// Different modes
backtestArgs := strategy.UnifiedStrategyArgs{
    StrategyID:    123,
    ExecutionMode: "backtest",
    StartDate:     &startDate,
    EndDate:       &endDate,
    Symbols:       []string{"AAPL", "MSFT"},
}

screeningArgs := strategy.UnifiedStrategyArgs{
    StrategyID:    123,
    ExecutionMode: "screening",
    Universe:      []string{"AAPL", "MSFT", "GOOGL", "AMZN", "TSLA"},
    Limit:         10,
}
```

### Frontend Integration

```typescript
// Real-time alerts
const realtimeResult = await executeUnifiedStrategy({
  strategyId: 123,
  executionMode: 'realtime',
  symbols: ['AAPL', 'MSFT', 'GOOGL']
});

// Backtest analysis
const backtestResult = await executeUnifiedStrategy({
  strategyId: 123,
  executionMode: 'backtest',
  startDate: '2023-01-01',
  endDate: '2024-01-01',
  symbols: ['AAPL', 'MSFT']
});

// Screening
const screeningResult = await executeUnifiedStrategy({
  strategyId: 123,
  executionMode: 'screening',
  universe: sp500Symbols,
  limit: 20
});
```

## Performance Optimization

### Real-time Mode
- ⚡ 30-second timeout
- 🎯 Focus on speed over completeness
- 📊 Minimal data requirements
- 🔄 Efficient caching

### Backtest Mode  
- ⏱️ 5-minute timeout
- 📈 Comprehensive historical analysis
- 💾 Bulk data loading
- 📊 Detailed performance metrics

### Screening Mode
- ⏱️ 2-minute timeout
- 🔍 Parallel universe processing
- 📋 Efficient ranking algorithms
- 🎯 Quality filtering

## Error Handling

The engine provides automatic fallbacks:

1. **Mode-specific functions not found**: Falls back to `classify_symbol()`
2. **Data unavailable**: Graceful degradation with logging
3. **Execution timeout**: Returns partial results with error status
4. **Memory limits**: Automatic garbage collection and limits

## Migration Guide

### From Legacy Strategies

1. **Real-time strategies**: Wrap in `run_realtime_scan()`
2. **Backtest strategies**: Wrap in `run_batch_backtest()`
3. **Screening strategies**: Wrap in `run_screening()`
4. **Keep fallback functions**: Maintain `classify_symbol()` for compatibility

### Best Practices

1. **Start simple**: Begin with one mode, expand to others
2. **Use logging**: Help with debugging and monitoring
3. **Test all modes**: Ensure consistent logic across modes
4. **Optimize for mode**: Tailor performance to mode requirements
5. **Handle errors gracefully**: Use try/catch blocks

## Troubleshooting

### Common Issues

**Strategy not executing in desired mode**:
- Check function names (`run_realtime_scan`, `run_batch_backtest`, `run_screening`)
- Verify function signatures match expected parameters
- Ensure return format matches mode requirements

**Performance issues**:
- Real-time: Reduce symbol count, use shorter timeframes
- Backtest: Limit universe size, optimize data queries
- Screening: Use efficient filtering, parallel processing

**Data access errors**:
- Check symbol validity with `validate_symbol()`
- Handle missing data gracefully
- Use appropriate timeframes for analysis

### Debug Tips

```python
# Add comprehensive logging
log(f"Processing {len(symbols)} symbols in {execution_mode} mode")
log(f"Data loaded for {symbol}: {len(price_data.get('close', []))} periods")

# Validate inputs
if not symbols:
    log("Warning: No symbols provided")
    return {'alerts': [], 'signals': {}}

# Handle exceptions gracefully
try:
    result = complex_calculation(data)
except Exception as e:
    log(f"Error in calculation: {e}")
    result = fallback_value
```

## Future Enhancements

- 🔄 **Streaming mode**: Continuous real-time processing
- 🌐 **Multi-asset support**: Crypto, forex, options
- 📊 **Advanced analytics**: ML-powered insights
- 🔗 **Strategy chaining**: Combine multiple strategies
- 📱 **Mobile optimization**: Lightweight mobile execution

---

The Unified Strategy Engine represents the future of algorithmic trading strategy development - **write once, run everywhere, optimize automatically**. 