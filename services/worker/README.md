# Python Worker Service

A high-performance Python execution environment for AI-generated trading strategies. This service provides **raw market data only** - technical indicators must be implemented by the AI-generated strategy code.

## Architecture Overview

The system is designed to force AI models to implement their own technical analysis calculations, promoting better understanding and more sophisticated strategy development.

### Core Components

- **ExecutionEngine**: Sandboxed Python execution with resource limits
- **DataProvider**: Raw market data access (OHLCV, fundamentals, volume, etc.)
- **SecurityValidator**: AST-based code validation and security enforcement
- **Worker**: Redis-based job processing and result management

## ⚠️ Critical: Component Synchronization Requirements

**ALL components must be kept in perfect synchronization at all times.** Any changes to strategy requirements, data structures, or validation rules must be propagated across all components immediately.

### Components That Must Stay in Sync:

1. **System Prompt** (`services/backend/internal/app/strategy/prompts/classifier.txt`)
   - Defines what AI models should generate
   - Function signatures, parameter names, return structures
   - Available DataFrame columns and technical indicator examples

2. **Security Validator** (`services/worker/src/validator.py`)
   - Validates generated Python code for security and compliance
   - Function signature requirements (parameter names, types)
   - Required instance fields, allowed modules, forbidden patterns

3. **Strategy Examples** (`services/worker/src/examples.py`)
   - Reference implementations showing correct patterns
   - Must match exact function signatures and return formats
   - Demonstrates proper technical indicator calculations

4. **DataFrame Engine** (`services/worker/src/engine.py`)
   - Executes strategies and processes results
   - Data loading, preprocessing, and result formatting
   - Must handle the exact DataFrame structure defined in other components

5. **Go Backend Interface** (`services/backend/internal/app/strategy/strategies.go`)
   - Calls worker service and processes results
   - Must understand the return format and field names
   - Handles strategy creation, execution, and result parsing

### Synchronization Requirements:

**Function Signatures:**
- Parameter name: `df` (pandas DataFrame)
- Return type: `List[Dict]` with instances
- No `signal` field (implicit True)
- `timestamp` field (not `date`)

**DataFrame Structure:**
- Raw market data only (ticker, date, OHLCV, volume, fund_*)
- NO pre-calculated technical indicators
- Strategies must calculate their own indicators

**Instance Fields:**
- **Required**: `ticker` (string), `timestamp` (string)
- **Optional**: `score`, `message`, custom metrics
- **Forbidden**: `signal` (redundant)

**Security Rules:**
- Pandas and numpy allowed for data processing
- No file I/O, network access, or system calls
- No dangerous built-ins or introspection

**Validation Patterns:**
- Function must be named descriptively (not generic `strategy`)
- Must import pandas (`import pandas as pd`)
- Must sort data before calculations involving time series
- Must use `.groupby('ticker')` for multi-symbol operations

### When Making Changes:

1. **Identify Impact**: Determine which components are affected
2. **Update All**: Never update just one component - update ALL affected components
3. **Test Integration**: Verify that generated strategies pass validation and execute correctly
4. **Verify Examples**: Ensure all examples in `examples.py` still work
5. **Update Documentation**: Keep README and comments current

### Common Sync Issues:

- ❌ Changing required fields in validator without updating system prompt
- ❌ Adding new DataFrame columns without updating documentation
- ❌ Modifying function signatures without updating examples
- ❌ Changing validation rules without updating AI generation prompts
- ❌ Adding new security restrictions without updating allowed operations

**Remember: The AI generates code based on the system prompt, which must pass the validator, match the examples, and execute correctly in the engine. ALL must be perfectly aligned.**

## Available Raw Data Functions

### Price & Market Data
- `get_price_data(symbol, timeframe, days)` - Raw OHLCV data
- `get_historical_data(symbol, timeframe, periods, offset)` - Historical price data with lag
- `get_security_info(symbol)` - Basic security metadata
- `get_multiple_symbols_data(symbols, timeframe, days)` - Batch price data

### Fundamental Data
- `get_fundamental_data(symbol, metrics)` - Raw financial metrics
- `get_earnings_data(symbol, quarters)` - Historical earnings data
- `get_financial_statements(symbol, statement_type, periods)` - Financial statements

### Market & Sector Data
- `get_sector_data(sector, days)` - Raw sector performance data
- `get_market_indices(indices, days)` - Index data (SPY, QQQ, etc.)
- `get_volume_data(symbol, days)` - Volume and dollar volume data

### Utility Functions
- `calculate_returns(prices, periods)` - Simple return calculation
- `calculate_log_returns(prices, periods)` - Logarithmic returns
- `rolling_window(data, window)` - Create rolling windows for calculations
- `normalize_data(data, method)` - Data normalization utilities
- `vectorized_operation(values, operation, operand)` - Fast math operations

## Technical Indicator Implementation

**Important**: The system does NOT provide pre-calculated technical indicators. AI-generated strategies must implement their own calculations using raw data.

### Example: RSI Implementation

```python
def classify_symbol(symbol):
    # Get raw price data
    price_data = get_price_data(symbol, timeframe='1d', days=50)
    
    # Implement RSI calculation
    def calculate_rsi(prices, period=14):
        if len(prices) < period + 1:
            return []
        
        # Calculate price changes
        deltas = [prices[i] - prices[i-1] for i in range(1, len(prices))]
        
        # Separate gains and losses
        gains = [delta if delta > 0 else 0 for delta in deltas]
        losses = [-delta if delta < 0 else 0 for delta in deltas]
        
        # Calculate initial averages
        avg_gain = sum(gains[:period]) / period
        avg_loss = sum(losses[:period]) / period
        
        rsi = []
        for i in range(period, len(gains)):
            if avg_loss == 0:
                rsi.append(100)
            else:
                rs = avg_gain / avg_loss
                rsi.append(100 - (100 / (1 + rs)))
            
            # Update averages using Wilder's smoothing
            avg_gain = (avg_gain * (period - 1) + gains[i]) / period
            avg_loss = (avg_loss * (period - 1) + losses[i]) / period
        
        return rsi
    
    # Use the RSI in strategy logic
    closes = price_data['close']
    rsi_values = calculate_rsi(closes, 14)
    
    if not rsi_values:
        return False
    
    # Strategy: RSI oversold condition
    return rsi_values[-1] < 30
```

### Example: Bollinger Bands Implementation

```python
def classify_symbol(symbol):
    price_data = get_price_data(symbol, timeframe='1d', days=50)
    
    def calculate_bollinger_bands(prices, period=20, std_dev=2.0):
        if len(prices) < period:
            return {'upper': [], 'middle': [], 'lower': []}
        
        # Calculate SMA (middle band)
        middle = []
        for i in range(period - 1, len(prices)):
            avg = sum(prices[i - period + 1:i + 1]) / period
            middle.append(avg)
        
        # Calculate standard deviation and bands
        upper = []
        lower = []
        
        for i in range(len(middle)):
            data_slice = prices[i:i + period]
            mean_val = sum(data_slice) / len(data_slice)
            variance = sum((x - mean_val) ** 2 for x in data_slice) / len(data_slice)
            std = variance ** 0.5
            
            upper.append(middle[i] + (std_dev * std))
            lower.append(middle[i] - (std_dev * std))
        
        return {'upper': upper, 'middle': middle, 'lower': lower}
    
    # Use Bollinger Bands in strategy
    closes = price_data['close']
    bb = calculate_bollinger_bands(closes, 20, 2.0)
    
    if not bb['lower']:
        return False
    
    # Strategy: Price near lower band
    current_price = closes[-1]
    lower_band = bb['lower'][-1]
    
    return current_price <= lower_band * 1.02
```

## Performance Characteristics

- **Execution Speed**: Sub-millisecond strategy execution
- **Memory Usage**: < 50MB per strategy execution
- **Concurrency**: Handles 100+ concurrent strategy executions
- **Data Access**: Direct PostgreSQL queries with connection pooling
- **Security**: Comprehensive sandboxing with AST validation

## Security Features

- **AST Validation**: Code analysis before execution
- **Module Restrictions**: Limited import capabilities
- **Resource Limits**: CPU time, memory, and execution time constraints
- **Sandboxed Environment**: Isolated execution context
- **Input Validation**: All parameters validated and sanitized

## Installation & Setup

```bash
# Install dependencies
pip install -r requirements.txt

# Set environment variables
export DATABASE_URL="postgresql://user:pass@localhost/trading_db"
export REDIS_URL="redis://localhost:6379"

# Run tests
python test_execution.py

# Start worker
python worker.py
```

## Environment Variables

- `DATABASE_URL`: PostgreSQL connection string
- `REDIS_URL`: Redis connection string for job queue
- `LOG_LEVEL`: Logging level (DEBUG, INFO, WARNING, ERROR)
- `MAX_EXECUTION_TIME`: Maximum strategy execution time (default: 30s)
- `MAX_MEMORY_MB`: Maximum memory per execution (default: 100MB)

## Testing

The test suite demonstrates various strategy implementations:

```bash
python test_execution.py
```

Tests include:
- Basic strategy execution with custom SMA
- Raw data accessor function validation
- Custom RSI implementation from scratch
- Bollinger Bands calculation example
- Security validation and sandboxing
- Data provider functionality

## Integration with Go Backend

The worker service integrates with the Go backend through:

1. **Strategy Generation**: Go service generates Python code using AI
2. **Job Queue**: Strategies queued via Redis for execution
3. **Result Processing**: Results returned through Redis channels
4. **Data Consistency**: Shared PostgreSQL database for market data

## Benefits of Raw Data Approach

1. **Educational**: AI learns to implement technical analysis
2. **Flexibility**: Custom indicators and novel calculations
3. **Performance**: Optimized calculations for specific use cases
4. **Transparency**: Clear understanding of calculation methods
5. **Innovation**: Encourages development of new indicators

## Deployment

For production deployment:

1. Use PyPy for 10-100x performance improvement
2. Configure resource limits based on hardware
3. Set up monitoring and alerting
4. Use Redis Cluster for high availability
5. Implement proper logging and error tracking 