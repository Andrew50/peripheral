#!/usr/bin/env python3
"""
Test script to verify that typing imports are no longer an issue
"""

import asyncio
import sys
import os

# Add the src directory to Python path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'src'))

from execution_engine import PythonExecutionEngine


async def test_typing_fix():
    """Test that we can execute strategy code without typing imports"""
    print("🧪 Testing typing fix...")
    
    # Strategy code that previously would have had typing imports
    strategy_code = """
# Simple strategy without typing imports
def classify_symbol(symbol):
    '''
    Simple gap-up strategy for testing
    '''
    try:
        # Mock some data since we don't have real data access
        mock_data = {
            'open': [100, 105],      # Today opened at 105
            'close': [98, 0]         # Yesterday closed at 98, today not closed yet
        }
        
        if len(mock_data['open']) >= 2 and len(mock_data['close']) >= 1:
            current_open = mock_data['open'][-1]     # 105
            previous_close = mock_data['close'][-2]  # 98
            
            # Calculate gap percentage  
            gap_percent = ((current_open - previous_close) / previous_close) * 100
            
            # Return True if gap is greater than 2%
            return gap_percent > 2.0
        
        return False
        
    except Exception as e:
        return False

# Test the function
result = classify_symbol('TEST')
save_result('classification', result)
save_result('gap_detected', result)
save_result('test_symbol', 'TEST')
"""

    engine = PythonExecutionEngine()
    
    try:
        result = await engine.execute(strategy_code, {"symbol": "TEST"})
        
        if 'classification' in result:
            print("✅ Strategy executed successfully!")
            print(f"   Classification: {result['classification']}")
            print(f"   Gap detected: {result.get('gap_detected', 'N/A')}")
            print(f"   Test symbol: {result.get('test_symbol', 'N/A')}")
            return True
        else:
            print("❌ Strategy executed but no classification result")
            print(f"   Results: {result}")
            return False
            
    except ImportError as e:
        if 'typing' in str(e).lower():
            print(f"❌ Typing import error still present: {e}")
            return False
        else:
            print(f"❌ Other import error: {e}")
            return False
    except Exception as e:
        print(f"❌ Execution failed: {e}")
        print(f"   Error type: {type(e).__name__}")
        return False


async def test_no_typing_annotations():
    """Test that we can execute code with built-in types instead of typing"""
    print("\n🧪 Testing built-in types...")
    
    strategy_code = """
# Strategy using built-in types (no typing module)
def classify_symbol(symbol):
    '''
    Test strategy using built-in Python types
    '''
    # Using built-in types
    data_dict = {}  # dict instead of Dict
    price_list = []  # list instead of List
    symbol_str = str(symbol)  # str instead of str annotation
    
    # Mock some processing
    data_dict['symbol'] = symbol_str
    price_list.extend([100.0, 105.0, 102.0])
    
    # Simple logic
    if len(price_list) > 0:
        latest_price = price_list[-1] 
        result = latest_price > 101.0
    else:
        result = False
    
    return result

# Execute the function
classification = classify_symbol('BUILTIN_TEST')
save_result('classification', classification)
save_result('built_in_types_test', True)
"""

    engine = PythonExecutionEngine()
    
    try:
        result = await engine.execute(strategy_code, {"symbol": "BUILTIN_TEST"})
        
        if result.get('built_in_types_test'):
            print("✅ Built-in types work correctly!")
            print(f"   Classification: {result['classification']}")
            return True
        else:
            print("❌ Built-in types test failed")
            return False
            
    except Exception as e:
        print(f"❌ Built-in types test failed: {e}")
        return False


async def main():
    """Run all tests"""
    print("🚀 Testing Python execution engine typing fixes...")
    print("=" * 60)
    
    success = True
    
    # Test 1: Basic execution without typing
    test1_passed = await test_typing_fix()
    success = success and test1_passed
    
    # Test 2: Built-in types
    test2_passed = await test_no_typing_annotations()  
    success = success and test2_passed
    
    print("\n" + "=" * 60)
    if success:
        print("🎉 All tests passed! Typing fix is working correctly.")
        print("   ✅ No typing imports required")
        print("   ✅ Built-in types work properly")
        print("   ✅ Strategy execution is successful")
    else:
        print("❌ Some tests failed. Typing issues may still exist.")
    
    return success


if __name__ == "__main__":
    success = asyncio.run(main())
    sys.exit(0 if success else 1) 