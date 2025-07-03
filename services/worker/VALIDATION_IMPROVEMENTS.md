# Validation System Improvements: Exact min_bars Requirements

## Overview

The validation system has been enhanced to extract and use the exact `min_bars` requirements from each `get_bar_data()` call in strategy code, instead of always using a fixed number of bars or applying arbitrary caps.

## Key Changes

### 1. AST Analysis for min_bars Extraction

**File**: `services/worker/src/validator.py`

Added new methods to the `SecurityValidator` class:

- `extract_min_bars_requirements(code: str)` - Parses strategy code AST to find all `get_bar_data()` calls
- `_extract_get_bar_data_params(call_node)` - Extracts parameters from individual call nodes
- `_extract_string_value(node)` - Safely extracts string values from AST nodes
- `_extract_int_value(node)` - Safely extracts integer values from AST nodes

**Functionality**:
- Analyzes the Abstract Syntax Tree (AST) of strategy code
- Finds all `get_bar_data()` function calls
- Extracts `timeframe` and `min_bars` parameters from each call
- Supports both positional and keyword arguments
- Handles default values (timeframe='1d', min_bars=1)
- Returns detailed information including line numbers for debugging

### 2. Enhanced Validation Execution

**File**: `services/worker/src/accessor_strategy_engine.py`

Modified `execute_validation()` method:

- Now extracts min_bars requirements before execution
- Passes extracted requirements to data accessor context
- Provides detailed logging of exact requirements found
- Shows which line each requirement comes from
- Includes requirements in validation result for debugging

**Before**:
```python
# Set execution context for validation with MINIMAL data
self.data_accessor.set_execution_context(
    mode='validation',
    symbols=['AAPL']    # Just one symbol for validation
)
```

**After**:
```python
# Extract min_bars requirements from strategy code
min_bars_requirements = self.validator.extract_min_bars_requirements(strategy_code)

# Set execution context for validation with exact requirements
context_data = {
    'mode': 'validation',
    'symbols': ['AAPL'],
    'min_bars_requirements': min_bars_requirements
}
self.data_accessor.set_execution_context(**context_data)
```

### 3. Data Accessor Context Enhancement

**File**: `services/worker/src/data_accessors.py`

Updated `set_execution_context()` method:

- Added `min_bars_requirements` parameter
- Stores extracted requirements in execution context
- Makes requirements available to data fetching logic

### 4. Removed Arbitrary 100-Bar Cap

**File**: `services/worker/src/data_accessors.py`

Modified validation mode logic in `_get_bar_data_single()`:

**Before**:
```python
# Override min_bars to be reasonable for validation, but respect strategy needs
if min_bars > 100:  # Set reasonable upper limit for validation
    original_min_bars = min_bars
    min_bars = 100  # Maximum 100 bars for validation
    logger.info(f"🧪 Validation mode: limiting min_bars from {original_min_bars} to {min_bars} for performance")
```

**After**:
```python
# Check if this specific min_bars matches any requirement from the strategy code
min_bars_requirements = context.get('min_bars_requirements', [])
matching_requirement = None
for req in min_bars_requirements:
    if req.get('timeframe') == timeframe and req.get('min_bars') == min_bars:
        matching_requirement = req
        break

if matching_requirement:
    logger.info(f"🧪 Validation mode: using exact min_bars={min_bars} for {timeframe} (from line {matching_requirement['line_number']})")
else:
    logger.info(f"🧪 Validation mode: using min_bars={min_bars} for {timeframe} (no arbitrary caps applied)")

# No min_bars override - use the strategy's exact requirements
```

## Benefits

### 1. Accurate Validation
- Strategies are now validated with the exact data they need
- No more false failures due to insufficient data
- No more false passes due to overly generous caps

### 2. Better Performance
- Only fetches the exact amount of data needed
- No wasted computation on unnecessary bars
- More predictable validation times

### 3. Improved Debugging
- Clear logging shows exactly what requirements were extracted
- Line numbers help identify which calls need what data
- Validation results include extracted requirements

### 4. Strategy Integrity
- Respects the strategy author's intent
- Ensures strategies that need 200+ bars get proper validation
- Prevents arbitrary limits from masking real issues

## Example Output

When validating a strategy, you'll now see logs like:

```
📊 Extracted min_bars requirements from strategy:
   Line 15: get_bar_data(timeframe='1d', min_bars=20)
   Line 23: get_bar_data(timeframe='1h', min_bars=50)
   Line 31: get_bar_data(timeframe='1w', min_bars=10)
🎯 Validation will use exact min_bars requirements (max: 50 bars)

🧪 Validation mode: using exact min_bars=20 for 1d (from line 15)
🧪 Validation mode: using exact min_bars=50 for 1h (from line 23)
🧪 Validation mode: using exact min_bars=10 for 1w (from line 31)
```

## Supported Call Patterns

The AST analysis supports various `get_bar_data()` call patterns:

```python
# Positional arguments
data = get_bar_data("1d", None, 20)

# Keyword arguments
data = get_bar_data(timeframe="1h", min_bars=50)

# Mixed arguments
data = get_bar_data("5m", columns=["close"], min_bars=100)

# Default values
data = get_bar_data("1d")  # Uses min_bars=1

# Complex expressions (if they resolve to constants)
data = get_bar_data(timeframe="1d", min_bars=LOOKBACK_DAYS)  # Would extract variable name
```

## Backward Compatibility

- All existing functionality remains unchanged
- Strategies without `get_bar_data()` calls work as before
- Old validation behavior is preserved for non-accessor strategies
- No breaking changes to existing APIs

## Future Enhancements

This foundation enables future improvements:

- Variable resolution for dynamic min_bars values
- Cross-timeframe dependency analysis
- Memory usage optimization based on actual requirements
- Performance profiling per data accessor call 