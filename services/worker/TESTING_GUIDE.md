# Python Strategy System - Testing Guide

This guide shows you how to test and develop custom Python trading strategies using your system.

## 🚀 Quick Start

### 1. Environment Setup

```bash
cd services/worker
python3 -m venv venv
source venv/bin/activate
pip install psycopg2-binary sqlalchemy pandas numpy redis asyncio
```

### 2. Run Basic Tests

```bash
# Test core functionality with mock data
python test_simple.py

# Run strategy demos
python demo_strategy.py

# Test with real execution pipeline (requires database)
python test_execution.py
```

## 📋 Test Categories

### ✅ **Working Tests** (No Database Required)

These tests use mock data and demonstrate core functionality:

- **`test_simple.py`** - Mock data tests for basic execution
- **`demo_strategy.py`** - Interactive strategy examples
- **Individual Strategy Tests** - Custom strategy validation

### 🔧 **Database-Dependent Tests**

These require your full backend setup:

- **`test_execution.py`** - Full pipeline with real data functions
- **`test_worker_pipeline.py`** - Redis queue integration
- **`test_automated.py`** - End-to-end automation

## 📊 Strategy Development Workflow

### 1. **Write Your Strategy**

```python
# Example: Custom MACD Strategy
strategy_code = """
# Get price data (in real system)
# price_data = get_price_data(symbol, timeframe='1d', days=50)

# Mock data for testing
mock_prices = [100, 101, 99, 102, 98, 103, 97, 104, 96, 105]

def calculate_macd(prices, fast=12, slow=26, signal=9):
    \"\"\"Calculate MACD indicator from scratch\"\"\"
    # Implement EMA calculation
    def ema(data, period):
        alpha = 2 / (period + 1)
        ema_values = [data[0]]
        for price in data[1:]:
            ema_values.append(alpha * price + (1 - alpha) * ema_values[-1])
        return ema_values
    
    if len(prices) < slow:
        return {'macd': [], 'signal': [], 'histogram': []}
    
    # Calculate MACD line
    ema_fast = ema(prices, fast)
    ema_slow = ema(prices, slow)
    macd_line = [ema_fast[i] - ema_slow[i] for i in range(len(ema_slow))]
    
    # Calculate signal line
    signal_line = ema(macd_line, signal)
    
    # Calculate histogram
    histogram = [macd_line[i] - signal_line[i] for i in range(len(signal_line))]
    
    return {
        'macd': macd_line,
        'signal': signal_line,
        'histogram': histogram
    }

# Calculate MACD
macd = calculate_macd(mock_prices)

# Strategy logic
if macd['histogram']:
    current_histogram = macd['histogram'][-1]
    prev_histogram = macd['histogram'][-2] if len(macd['histogram']) > 1 else 0
    
    # MACD histogram crossing above zero
    bullish_cross = prev_histogram <= 0 and current_histogram > 0
    
    save_result('classification', bullish_cross)
    save_result('current_histogram', current_histogram)
    save_result('prev_histogram', prev_histogram)
    save_result('signal_type', 'BULLISH_CROSS' if bullish_cross else 'NO_SIGNAL')
else:
    save_result('classification', False)
    save_result('reason', 'Insufficient data')
"""
```

### 2. **Test Your Strategy**

```python
import asyncio
from src.execution_engine import PythonExecutionEngine

async def test_my_strategy():
    engine = PythonExecutionEngine()
    result = await engine.execute(strategy_code, {'symbol': 'TEST'})
    
    print("Strategy Results:")
    for key, value in result.items():
        print(f"  {key}: {value}")

# Run the test
asyncio.run(test_my_strategy())
```

### 3. **Validate Results**

Check that your strategy returns:
- ✅ `classification`: Boolean (True/False)
- ✅ Key metrics as separate fields
- ✅ `reason`: String explaining the decision

## 🎯 Testing Patterns

### Pattern 1: Simple Indicator Test

```python
# Test a single technical indicator
async def test_rsi_calculation():
    strategy_code = """
# Your RSI implementation here
def calculate_rsi(prices, period=14):
    # ... implementation
    pass

# Test with known data
test_prices = [44, 44.34, 44.09, 44.15, 43.61]  # Known RSI values
rsi_values = calculate_rsi(test_prices, 14)

save_result('rsi', rsi_values)
save_result('success', len(rsi_values) > 0)
"""
    
    engine = PythonExecutionEngine()
    result = await engine.execute(strategy_code, {})
    assert result['success'] == True
```

### Pattern 2: Strategy Signal Test

```python
# Test strategy signals with different market conditions
async def test_strategy_signals():
    test_cases = [
        {'prices': [100, 99, 98, 97, 96], 'expected': True},   # Downtrend
        {'prices': [100, 101, 102, 103, 104], 'expected': False}, # Uptrend
    ]
    
    for case in test_cases:
        result = await test_with_prices(case['prices'])
        assert result['classification'] == case['expected']
```

### Pattern 3: Edge Case Testing

```python
# Test with edge cases
async def test_edge_cases():
    edge_cases = [
        {'prices': [], 'name': 'empty_data'},
        {'prices': [100], 'name': 'single_point'},
        {'prices': [100] * 20, 'name': 'flat_line'},
        {'prices': [float('inf'), 100], 'name': 'infinite_value'},
    ]
    
    for case in edge_cases:
        result = await test_with_prices(case['prices'])
        # Strategy should handle gracefully
        assert 'classification' in result
        assert 'reason' in result
```

## 🔍 Debugging Tips

### 1. **Add Debug Logging**

```python
# In your strategy code
log("Starting RSI calculation", "info")
log(f"Price data length: {len(prices)}", "debug")

# Check intermediate results
save_result('debug_prices', prices[:5])  # First 5 prices
save_result('debug_deltas', deltas[:5])   # First 5 deltas
```

### 2. **Validate Calculations**

```python
# Test against known values
def test_sma_accuracy():
    test_prices = [1, 2, 3, 4, 5]
    sma_3 = calculate_sma(test_prices, 3)
    expected = [2.0, 3.0, 4.0]  # Known correct values
    
    for i, (actual, expected_val) in enumerate(zip(sma_3, expected)):
        assert abs(actual - expected_val) < 0.01, f"SMA mismatch at index {i}"
```

### 3. **Performance Testing**

```python
import time

async def test_performance():
    large_dataset = list(range(1000))  # 1000 data points
    
    start_time = time.time()
    result = await engine.execute(strategy_with_large_data, {'prices': large_dataset})
    execution_time = time.time() - start_time
    
    print(f"Execution time: {execution_time:.3f}s")
    assert execution_time < 1.0, "Strategy too slow"
```

## 📈 Advanced Testing

### Multi-Symbol Testing

```python
async def test_multiple_symbols():
    symbols = ['AAPL', 'GOOGL', 'MSFT', 'TSLA']
    
    for symbol in symbols:
        result = await engine.execute(strategy_code, {'symbol': symbol})
        
        # Validate each symbol
        assert 'classification' in result
        print(f"{symbol}: {result['classification']}")
```

### Backtesting Framework

```python
async def backtest_strategy(price_history, lookback_period=20):
    results = []
    
    for i in range(lookback_period, len(price_history)):
        # Get historical window
        window_data = price_history[i-lookback_period:i]
        
        # Run strategy
        result = await engine.execute(strategy_code, {
            'prices': window_data,
            'current_date': i
        })
        
        results.append({
            'date': i,
            'classification': result['classification'],
            'price': price_history[i]
        })
    
    return results
```

## 🚨 Common Issues & Solutions

### Issue 1: "Module not available" warnings
**Solution**: These are expected - the system only loads available modules.

### Issue 2: Strategy returns no results
**Solution**: Ensure you're using `save_result()` to store outputs.

### Issue 3: Type errors in calculations
**Solution**: Add type checking and handle edge cases:

```python
def safe_divide(a, b):
    return a / b if b != 0 else 0

def validate_prices(prices):
    return [p for p in prices if isinstance(p, (int, float)) and not math.isnan(p)]
```

### Issue 4: Performance issues
**Solution**: Use list comprehensions and avoid nested loops:

```python
# Slow
result = []
for i in range(len(prices)):
    for j in range(period):
        # calculation
    result.append(value)

# Fast
result = [calculation(prices[i:i+period]) for i in range(len(prices)-period)]
```

## 🎉 Next Steps

1. **Start with `test_simple.py`** to validate your environment
2. **Run `demo_strategy.py`** to see examples
3. **Create your own strategy** using the patterns above
4. **Test thoroughly** with different market conditions
5. **Integrate with the full system** once validated

## 📚 Key Resources

- **Data Functions**: See `services/backend/internal/app/strategy/strategies.go` for available functions
- **Examples**: Check `test_simple.py` and `demo_strategy.py`
- **Architecture**: Read `services/worker/README.md`

Happy strategy development! 🚀 