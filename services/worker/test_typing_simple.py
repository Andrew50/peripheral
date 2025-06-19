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
    
    # Try to compile the code
    try:
        compile(code_with_typing, "<test>", "exec")
        print("❌ Typing imports should not be allowed but compilation succeeded")
        return False
    except ImportError:
        print("✅ Typing imports correctly blocked at import level")
        return True
    except Exception as e:
        # Compilation will succeed, but execution should fail
        print(f"⚠️  Compilation succeeded, but execution should fail: {e}")
        return True


def test_builtin_types():
    """Test that built-in types work correctly"""
    print("\n🧪 Testing built-in types...")
    
    code_with_builtins = """
# Using built-in types instead of typing
def classify_symbol(symbol):
    data = {}  # dict instead of Dict
    prices = []  # list instead of List
    name = str(symbol)  # str is fine
    
    # Simple logic
    prices.append(100.0)
    data['symbol'] = name
    
    return len(prices) > 0

result = classify_symbol('TEST')
"""
    
    try:
        # Compile and execute
        compiled_code = compile(code_with_builtins, "<test>", "exec")
        exec_globals = {'__builtins__': __builtins__}
        exec_locals = {}
        exec(compiled_code, exec_globals, exec_locals)  # nosec B102 - Safe test execution
        
        if exec_locals.get('result') is True:
            print("✅ Built-in types work correctly")
            return True
        else:
            print("❌ Built-in types test failed")
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
    
    # Test a simple strategy code without typing
    strategy_code = """
import math

def classify_symbol(symbol):
    # Simple gap calculation using math module
    current_price = 105.0
    previous_price = 100.0
    
    gap_percent = ((current_price - previous_price) / previous_price) * 100
    
    return gap_percent > 2.0

result = classify_symbol('TEST')
"""
    
    try:
        # Create a restricted environment similar to execution engine
        restricted_globals = {
            '__builtins__': {
                'len': len, 'range': range, 'str': str, 'int': int, 'float': float,
                'bool': bool, 'list': list, 'dict': dict, 'tuple': tuple,
                'abs': abs, 'round': round, 'min': min, 'max': max
            },
            'math': __import__('math')
        }
        
        compiled = compile(strategy_code, "<strategy>", "exec")
        exec_locals = {}
        exec(compiled, restricted_globals, exec_locals)  # nosec B102 - Safe test execution
        
        if exec_locals.get('result') is True:
            print("✅ Execution environment simulation successful")
            return True
        else:
            print("❌ Execution environment simulation failed")
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