# Friday Afternoon Gap Analysis Integration Test

## Overview

This integration test analyzes Friday afternoon movements in large-cap stocks (market cap > $50 billion) and tracks whether the subsequent gap on the next market session is in the same direction as the Friday afternoon "imbalance" move.

## Test Query

The test analyzes:
- **Stocks**: Market cap > $50 billion
- **Time Period**: Last 3 months
- **Movement Threshold**: > 2% in either direction
- **Time Window**: Friday afternoon from 3:49 PM to close (4:00 PM Eastern Time)
- **Gap Analysis**: Direction of gap on next market session vs Friday move direction

## Test Components

### 1. Integration Test (`test_friday_afternoon_gaps.py`)
- Creates a strategy from natural language query
- Executes backtest over 3-month period
- Analyzes results for direction match patterns
- Provides comprehensive reporting

### 2. Direct Strategy Test (`test_friday_strategy_execution.py`)
- Tests the strategy implementation directly
- Verifies data access and processing logic
- Includes mock data testing for CI environments

### 3. Test Suite Script (`test-friday-gaps.sh`)
- Runs both integration and direct tests
- Provides comprehensive reporting
- Handles both CI and development environments

## Expected Results

The test is designed to:
- **Return > 0 instances** when real data is available
- **Demonstrate gap direction correlation** analysis
- **Provide statistical match rate** between Friday moves and subsequent gaps
- **Handle graceful fallback** to mock data in CI environments

### Sample Output

```
📊 Total Friday Moves Found: 47
🎯 Matching Gap Directions: 28
📈 Direction Match Rate: 59.6%
⏱️ Execution Time: 3.2s
```

## CI Integration

The test is integrated into GitHub Actions workflow and runs:
- **On every push** to main branches (via matrix strategy)
- **On pull requests** targeting main branches  
- **In full pipeline tests** when explicitly requested

### Environment Variables

- `SKIP_INTEGRATION_TESTS`: When `true`, runs mock tests only
- `BACKEND_URL`: Backend server URL for integration testing
- `RUN_FRIDAY_GAP_TEST`: Enable/disable the Friday gap test

## Market Analysis

This test validates a real market analysis hypothesis:
- **Friday afternoon imbalances** (large moves in final minutes) may predict
- **Gap direction** on the next trading session (typically Monday)
- **Market makers** and institutional activity patterns

The test framework can be extended to analyze:
- Different time windows (not just 3:49-4:00 PM)
- Various market cap ranges
- Different movement thresholds
- Seasonal or event-driven patterns

## Running the Test

### Local Development
```bash
./test-friday-gaps.sh
```

### CI Environment
```bash
SKIP_INTEGRATION_TESTS=true ./test-friday-gaps.sh
```

### With Real Backend
```bash
BACKEND_URL=http://your-backend:8080 ./test-friday-gaps.sh
```

## Success Criteria

The test passes if:
1. **Integration test** completes without errors, OR
2. **Direct strategy test** demonstrates framework functionality
3. **Mock data validation** confirms logic correctness

The test is designed to be resilient and provide valuable insights whether running with real market data or mock data for development/CI purposes. 