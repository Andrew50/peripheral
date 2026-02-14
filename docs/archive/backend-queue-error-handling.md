# Go Queue Error Handling Improvements

## Overview
Updated the Go queue system to properly handle the new structured error format from the Python worker system and provide detailed error logging with full tracebacks.

## Changes Made

### 1. New Error Structure
- **Added `ErrorDetails` struct**: Represents structured error information from Python worker
  ```go
  type ErrorDetails struct {
      Type      string `json:"type"`      // Exception class name (e.g., "ValueError")
      Message   string `json:"message"`   // Error message
      Traceback string `json:"traceback"` // Full Python traceback
  }
  ```

### 2. Updated Result Types
All result types now include both legacy string errors and new structured errors:
- **`ResultUpdate`**: Added `ErrorDetails *ErrorDetails` field
- **`BacktestResult`**: Added `Error *ErrorDetails` field
- **`ScreeningResult`**: Added `ErrorDetails *ErrorDetails` field  
- **`AlertResult`**: Added `Error *ErrorDetails` field
- **`CreateStrategyResult`**: Added `ErrorDetails *ErrorDetails` field
- **`PythonAgentResult`**: Added `ErrorDetails *ErrorDetails` field

### 3. Error Extraction and Logging
- **`extractErrorDetails()`**: Extracts structured error information from data payload
  - Handles both structured error objects and legacy string errors
  - Returns both `ErrorDetails` struct and formatted error string
  
- **`logErrorDetails()`**: Logs detailed error information with traceback
  - Logs exception type and message
  - Prints full Python traceback for debugging
  - Falls back to simple string logging for legacy errors

### 4. Enhanced Error Handling

#### `subscribeToUpdates()` Function
- **Dual Error Source Handling**: Checks both `unifiedMsg.Error` field and `unifiedMsg.Data["error"]` 
- **Structured Error Parsing**: Properly extracts error details from nested JSON structures
- **Automatic Error Logging**: Logs detailed error information when status is "error"

#### `AwaitTyped()` Function  
- **Enhanced Error Reporting**: Returns formatted error messages with exception type
- **Detailed Logging**: Calls `logErrorDetails()` to log full traceback information
- **Backward Compatibility**: Falls back to legacy string errors when structured errors not available

#### Watchdog Functions
- **Improved Failure Logging**: Enhanced logging in `markTaskAsFailed()`, `waitForAssignment()`, and retry logic
- **Detailed Retry Information**: Logs retry attempts with specific failure reasons
- **Permanent Failure Tracking**: Clear logging when max retries exceeded

### 5. Updated Message Structure
- **`UnifiedMessage`**: Added `Error interface{}` field to handle both string and structured errors from Python worker

## Error Logging Examples

### Structured Error Output
```
❌ Task abc-123 failed with ValueError: Strategy execution failed: division by zero
📋 Task abc-123 traceback:
Traceback (most recent call last):
  File "engine.py", line 123, in strategy_func
    result = calculate_ratio(x, y)
  File "engine.py", line 45, in calculate_ratio  
    return x / y
ZeroDivisionError: division by zero
```

### Legacy Error Output
```
❌ Task abc-123 failed: Task execution failed: invalid strategy code
```

### Watchdog Failures
```
⚠️ Task abc-123 assignment failed (attempt 1/4): timeout waiting for task assignment
🔄 Task abc-123 worker worker_456 failed, retrying (1/3)
❌ Task abc-123 permanently failed after 3 retries
```

## Benefits Achieved

✅ **Full Python Tracebacks**: Complete stack traces are now logged and available for debugging

✅ **Structured Error Transport**: Errors include exception type, message, and traceback separately

✅ **Enhanced Debugging**: Developers can see exactly where and why Python code failed

✅ **Backward Compatibility**: Legacy string errors still work while new structured errors provide more detail

✅ **Improved Monitoring**: Better logging for task failures, retries, and watchdog actions

✅ **Type Safety**: Go code properly handles both string and structured error formats

## Testing
- **Unit Tests**: `TestExtractErrorDetails()` verifies error extraction works correctly
- **Error Scenarios**: Tests both structured errors, string errors, and no-error cases
- **Integration Ready**: Compatible with the updated Python worker error format

The Go queue system now provides comprehensive error handling that matches the enhanced error reporting from the Python worker system. 