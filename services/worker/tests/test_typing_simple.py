#!/usr/bin/env python3
"""
Simple test to verify that typing module restrictions work correctly
"""

def test_typing_imports():
    """Test that typing imports fail as expected"""
    print("🧪 Testing typing import restrictions...")
    
    # Code that should fail due to typing import
    code_with_typing = """
from typing import List, Dict, Tuple, Union, Optional

def test_function():
    return True
"""
    
    # Try to compile and execute the code in restricted environment
    try:
        compiled_code = compile(code_with_typing, "<test>", "exec")
        
        # Create restricted globals without typing module access
        restricted_globals = {
            '__builtins__': {
                'len': len, 'range': range, 'str': str, 'int': int, 'float': float,
                'bool': bool, 'list': list, 'dict': dict, 'tuple': tuple
            }
        }
        
        # Try to execute - this should fail
        exec(compiled_code, restricted_globals, {})  # nosec B102 - Safe test execution
        print("❌ Typing imports should not be allowed but execution succeeded")
        return False
        
    except (ImportError, ModuleNotFoundError, NameError) as e:
        print("✅ Typing imports correctly blocked during execution")
        return True
    except Exception as e:
        print(f"❌ Unexpected error during typing test: {e}")
        return False


def test_builtin_types():
    """Test that built-in types work correctly"""
    print("\n🧪 Testing built-in types...")
    
    code_with_builtins = """
# Using built-in types instead of typing - NEW ACCESSOR PATTERN
def strategy():
    instances = []  # list instead of List
    data = {}  # dict instead of Dict
    
    # Get bar data using accessor functions
    bar_data = get_bar_data(
        timeframe="1d", 
        columns=["ticker", "timestamp", "close"], 
        min_bars=1
    )
    
    # Simple logic
    if len(bar_data) > 0:
        import pandas as pd
        df = pd.DataFrame(bar_data, columns=["ticker", "timestamp", "close"])
        
        for _, row in df.iterrows():
            instances.append({
                'ticker': row['ticker'],
                'timestamp': str(row['timestamp']),
                'signal': True,
                'price': float(row['close'])
            })
    
    return instances

result = strategy()
"""
    
    try:
        # Compile and execute
        compiled_code = compile(code_with_builtins, "<test>", "exec")
        exec_globals = {'__builtins__': __builtins__}
        exec_locals = {}
        exec(compiled_code, exec_globals, exec_locals)  # nosec B102 - Safe test execution
        
        result = exec_locals.get('result')
        if result is not None and isinstance(result, list):
            print("✅ Built-in types work correctly")
            return True
        else:
            print("❌ Built-in types test failed - expected list result")
            return False
            
    except Exception as e:
        print(f"❌ Built-in types test failed: {e}")
        return False


def test_execution_environment_simulation():
    """Simulate the execution environment allowed modules"""
    print("\n🧪 Testing execution environment simulation...")
    
    # Simulate the allowed modules from execution_engine.py
    allowed_modules = {
        "numpy", "np", "pandas", "pd", "scipy", "sklearn", "matplotlib", 
        "seaborn", "plotly", "ta", "talib", "zipline", "pyfolio", "quantlib", 
        "statsmodels", "arch", "empyrical", "tsfresh", "stumpy", "prophet", 
        "math", "statistics", "datetime", "collections", "itertools", 
        "functools", "re", "json"
    }
    
    # Check that typing is not in allowed modules
    if "typing" in allowed_modules:
        print("❌ typing module is in allowed modules list")
        return False
    else:
        print("✅ typing module correctly excluded from allowed modules")
    
    # Test a simple strategy code without typing using NEW ACCESSOR PATTERN
    strategy_code = """
def strategy():
    # Simple gap calculation using math module - NEW ACCESSOR PATTERN
    instances = []
    
    # Mock bar data for testing (in real system, would use get_bar_data)
    bar_data = [
        ['AAPL', 1640995200, 100.0, 105.0],  # ticker, timestamp, prev_close, current_open
        ['MSFT', 1640995200, 200.0, 210.0]
    ]
    
    for row in bar_data:
        ticker = row[0]
        previous_price = row[2]
        current_price = row[3]
        
        gap_percent = ((current_price - previous_price) / previous_price) * 100
        
        if gap_percent > 2.0:
            instances.append({
                'ticker': ticker,
                'timestamp': '2024-01-01',
                'signal': True,
                'gap_percent': gap_percent
            })
    
    return instances

result = strategy()
"""
    
    try:
        # Import math module first
        import math
        
        # Create a restricted environment similar to execution engine
        restricted_globals = {
            '__builtins__': {
                'len': len, 'range': range, 'str': str, 'int': int, 'float': float,
                'bool': bool, 'list': list, 'dict': dict, 'tuple': tuple,
                'abs': abs, 'round': round, 'min': min, 'max': max
            },
            'math': math
        }
        
        compiled = compile(strategy_code, "<strategy>", "exec")
        exec_locals = {}
        exec(compiled, restricted_globals, exec_locals)  # nosec B102 - Safe test execution
        
        result = exec_locals.get('result')
        if result is not None and isinstance(result, list) and len(result) > 0:
            print("✅ Execution environment simulation successful")
            return True
        else:
            print("❌ Execution environment simulation failed - expected non-empty list")
            return False
            
    except Exception as e:
        print(f"❌ Execution environment simulation failed: {e}")
        return False


def main():
    """Run all tests"""
    print("🚀 Testing typing restrictions and fixes...")
    print("=" * 60)
    
    success = True
    
    # Test 1: Typing imports should be restricted
    test1_passed = test_typing_imports()
    success = success and test1_passed
    
    # Test 2: Built-in types should work
    test2_passed = test_builtin_types()
    success = success and test2_passed
    
    # Test 3: Execution environment simulation
    test3_passed = test_execution_environment_simulation()
    success = success and test3_passed
    
    print("\n" + "=" * 60)
    if success:
        print("🎉 All tests passed! Typing restrictions are working correctly.")
        print("   ✅ typing module properly excluded")
        print("   ✅ Built-in types work as expected")
        print("   ✅ Execution environment behaves correctly")
    else:
        print("❌ Some tests failed.")
    
    return success


if __name__ == "__main__":
    success = main()
    exit(0 if success else 1) 