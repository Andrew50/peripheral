# DataFrame Strategy System

A new execution engine for trading strategies that work with pandas DataFrames, providing better performance and easier data manipulation compared to the legacy system.

## Overview

The DataFrame Strategy System allows you to write trading strategies as simple Python functions that:
1. Take a pandas DataFrame containing all market data
2. Perform vectorized operations on the data
3. Return a list of instances (signals) with ticker, date, and custom metrics

## Key Benefits

- **Vectorized Operations**: Leverage pandas/numpy for fast data processing
- **Rich Data Context**: Access to OHLCV, technical indicators, and fundamentals in one DataFrame
- **Flexible Output**: Return custom metrics and scores with each signal
- **Multi-Mode Execution**: Same strategy works for backtesting, screening, and real-time alerts
- **Memory Efficient**: Batched data loading and processing

## Strategy Function Signature

```python
def strategy_function(df: pd.DataFrame) -> List[Dict]:
    """
    Args:
        df: DataFrame with columns:
            - ticker: Stock symbol
            - date: Trading date
            - open, high, low, close, volume: OHLCV data
            - Technical indicators: sma_5, sma_20, rsi, macd, etc.
            - Fundamentals: fund_pe_ratio, fund_market_cap, etc.
    
    Returns:
        List of instances: [
            {
                'ticker': 'AAPL',
                'date': '2024-01-15',
                'signal': True,
                'score': 0.85,
                'message': 'Custom alert message',
                # ... any custom metrics
            }
        ]
    """
```

## Available Data Columns

### Price Data
- `ticker`: Stock symbol
- `date`: Trading date
- `open`, `high`, `low`, `close`, `volume`: OHLCV data

### Technical Indicators
- `returns`: Daily returns (pct_change)
- `log_returns`: Log returns
- `sma_5`, `sma_10`, `sma_20`, `sma_50`: Simple moving averages
- `ema_12`, `ema_26`: Exponential moving averages
- `macd`, `macd_signal`, `macd_histogram`: MACD indicators
- `rsi`: Relative Strength Index
- `bb_upper`, `bb_lower`, `bb_middle`: Bollinger Bands
- `bb_width`, `bb_position`: Bollinger Band metrics
- `volume_sma`, `volume_ratio`: Volume indicators
- `gap`, `gap_pct`: Price gaps
- `atr`, `true_range`: Average True Range
- `price_position`: Position within daily range

### Fundamental Data (when available)
- `fund_pe_ratio`: Price-to-earnings ratio
- `fund_market_cap`: Market capitalization
- `fund_debt_to_equity`: Debt-to-equity ratio
- `fund_sector`: Sector classification
- And other fundamental metrics...

## Example Strategies

### 1. Gap Up Strategy

```python
def gap_up_strategy(df):
    """Find stocks that gap up more than 3% with volume confirmation"""
    instances = []
    
    # Filter for stocks with gap data
    df_filtered = df[df['gap_pct'].notna() & (df['gap_pct'] > 3.0)]
    
    # Add volume confirmation
    df_filtered = df_filtered[df_filtered['volume_ratio'] > 1.5]
    
    for _, row in df_filtered.iterrows():
        instances.append({
            'ticker': row['ticker'],
            'date': str(row['date']),
            'signal': True,
            'gap_percent': round(row['gap_pct'], 2),
            'volume_ratio': round(row['volume_ratio'], 2),
            'score': min(1.0, (row['gap_pct'] / 10.0) + (row['volume_ratio'] / 5.0)),
            'message': f"{row['ticker']} gapped up {row['gap_pct']:.1f}% with {row['volume_ratio']:.1f}x volume"
        })
    
    return instances
```

### 2. RSI Oversold Strategy

```python
def rsi_oversold_strategy(df):
    """Find oversold stocks with RSI < 30"""
    instances = []
    
    # Filter for oversold conditions
    df_filtered = df[(df['rsi'] < 30) & (df['rsi'].notna())]
    
    # Additional filter: must be above 50-day SMA for trend
    df_filtered = df_filtered[(df_filtered['close'] > df_filtered['sma_50']) & (df_filtered['sma_50'].notna())]
    
    for _, row in df_filtered.iterrows():
        instances.append({
            'ticker': row['ticker'],
            'date': str(row['date']),
            'signal': True,
            'rsi': round(row['rsi'], 2),
            'price': row['close'],
            'score': (30 - row['rsi']) / 30,  # Lower RSI = higher score
            'message': f"{row['ticker']} oversold with RSI {row['rsi']:.1f}"
        })
    
    return instances
```

### 3. MACD Crossover Strategy

```python
def macd_crossover_strategy(df):
    """MACD bullish crossover strategy"""
    instances = []
    
    # Sort by ticker and date to ensure proper order
    df_sorted = df.sort_values(['ticker', 'date']).copy()
    
    # Calculate previous MACD values
    df_sorted['macd_prev'] = df_sorted.groupby('ticker')['macd'].shift(1)
    df_sorted['macd_signal_prev'] = df_sorted.groupby('ticker')['macd_signal'].shift(1)
    
    # Find bullish crossovers (MACD crosses above signal line)
    df_filtered = df_sorted[
        (df_sorted['macd'] > df_sorted['macd_signal']) &      # Current: MACD above signal
        (df_sorted['macd_prev'] <= df_sorted['macd_signal_prev']) &  # Previous: MACD below/equal signal
        df_sorted['macd'].notna() &
        df_sorted['macd_signal'].notna()
    ]
    
    for _, row in df_filtered.iterrows():
        instances.append({
            'ticker': row['ticker'],
            'date': str(row['date']),
            'signal': True,
            'macd': round(row['macd'], 4),
            'macd_signal': round(row['macd_signal'], 4),
            'price': row['close'],
            'score': min(1.0, abs(row['macd_histogram']) * 10),
            'message': f"{row['ticker']} MACD bullish crossover at ${row['close']:.2f}"
        })
    
    return instances
```

## Usage Examples

### Backtesting

```python
from dataframe_strategy_engine import DataFrameStrategyEngine
from datetime import datetime, timedelta

engine = DataFrameStrategyEngine()

# Define your strategy code
strategy_code = '''
def my_strategy(df):
    # Your strategy logic here
    return instances
'''

# Run backtest
result = await engine.execute_backtest(
    strategy_code=strategy_code,
    symbols=['AAPL', 'MSFT', 'GOOGL'],
    start_date=datetime.now() - timedelta(days=365),
    end_date=datetime.now()
)

print(f"Found {len(result['instances'])} signals")
print(f"Performance metrics: {result['performance_metrics']}")
```

### Screening

```python
# Run screening to find current opportunities
result = await engine.execute_screening(
    strategy_code=strategy_code,
    universe=['AAPL', 'MSFT', 'GOOGL', 'TSLA', 'NVDA'],
    limit=10
)

print(f"Top opportunities:")
for opportunity in result['ranked_results']:
    print(f"- {opportunity['ticker']}: {opportunity['message']}")
```

### Real-time Alerts

```python
# Monitor symbols for real-time signals
result = await engine.execute_realtime(
    strategy_code=strategy_code,
    symbols=['AAPL', 'MSFT', 'GOOGL']
)

print(f"Generated {len(result['alerts'])} alerts")
for alert in result['alerts']:
    print(f"🚨 {alert['message']}")
```

## Integration with Strategy Worker

The DataFrame engine is integrated into the existing `StrategyWorker` class. You can specify which engine to use:

```python
from strategy_worker import StrategyWorker

worker = StrategyWorker()

# Use DataFrame engine (default)
result = await worker.run_backtest(
    strategy_id=123,
    engine_type='dataframe'
)

# Use legacy engine
result = await worker.run_backtest(
    strategy_id=123,
    engine_type='legacy'
)
```

## Performance Characteristics

- **Data Loading**: Batched loading (50 symbols at a time) to manage memory
- **Technical Indicators**: Pre-computed for all symbols to avoid recalculation
- **Memory Usage**: ~1-2GB per worker for typical datasets
- **Execution Speed**: 100-1000x faster than legacy symbol-by-symbol processing
- **Scalability**: Horizontal scaling through worker queues

## Best Practices

1. **Filter Early**: Use pandas boolean indexing to filter data before loops
2. **Vectorize Operations**: Prefer pandas operations over Python loops
3. **Handle NaN Values**: Always check for `.notna()` when using indicators
4. **Group Operations**: Use `groupby()` for per-symbol calculations
5. **Custom Metrics**: Include meaningful scores and messages in instances
6. **Memory Efficiency**: Process data in chunks for very large datasets

## Testing

Run the test script to verify your setup:

```bash
cd services/worker/src
python test_dataframe_strategy.py
```

## Migration from Legacy System

To migrate existing strategies:

1. **Data Access**: Replace individual data fetching with DataFrame operations
2. **Logic**: Convert symbol-by-symbol loops to vectorized pandas operations  
3. **Output Format**: Return list of instances instead of boolean classifications
4. **Testing**: Verify results match between old and new systems

## Security

The DataFrame engine includes the same security validations as the legacy system:
- AST-based code validation
- Restricted execution environment
- No file system or network access
- Memory and execution time limits 