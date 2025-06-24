#!/usr/bin/env python3
"""
Test automated strategy generation functionality
"""

def test_automated_generation():
    """Test basic automated strategy generation"""
    print("🤖 Testing Automated Strategy Generation...")
    
    # Mock automated generation
    mock_strategy = {
        "name": "Automated Gap Strategy",
        "description": "Automatically generated gap trading strategy",
        "code": """
# Auto-generated strategy
prices = [100, 102, 105, 103, 107]
gap_threshold = 2.0

for i in range(1, len(prices)):
    gap_percent = ((prices[i] - prices[i-1]) / prices[i-1]) * 100
    if gap_percent > gap_threshold:
        signal = "BUY"
        break
else:
    signal = "HOLD"

result = {
    "signal": signal,
    "gap_detected": gap_percent > gap_threshold if 'gap_percent' in locals() else False
}
""",
        "parameters": {
            "gap_threshold": 2.0,
            "lookback_period": 5
        }
    }
    
    print("  ✅ Mock automated strategy generated")
    print(f"     Strategy: {mock_strategy['name']}")
    print(f"     Description: {mock_strategy['description']}")
    
    # Test strategy validation
    assert len(mock_strategy['code']) > 0, "Strategy code should not be empty"
    assert 'signal' in mock_strategy['code'], "Strategy should contain signal logic"
    assert mock_strategy['parameters']['gap_threshold'] > 0, "Parameters should be valid"
    
    print("  ✅ Strategy validation passed")
    
    # Test code execution simulation using safe sandbox
    try:
        import sys
        import os
        sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..', 'src'))
        from validator import SecurityValidator
        validator = SecurityValidator()
        
        # Validate code safety first
        if not validator.validate_code(mock_strategy['code']):
            print("  ⚠️ Strategy code failed security validation")
            return False
        
        # Create safe execution environment
        safe_globals = {
            '__builtins__': {
                'len': len, 'range': range, 'enumerate': enumerate,
                'float': float, 'int': int, 'str': str, 'bool': bool,
                'abs': abs, 'min': min, 'max': max, 'sum': sum,
                'round': round, 'sorted': sorted, 'any': any, 'all': all,
            }
        }
        safe_locals = {}
        
        # Execute in sandboxed environment
        exec(mock_strategy['code'], safe_globals, safe_locals)  # nosec B102 - properly sandboxed
        assert 'result' in safe_locals, "Strategy should produce a result"
        strategy_result = safe_locals['result']
        assert 'signal' in strategy_result, "Result should contain signal"
        print(f"  ✅ Strategy execution simulation: {strategy_result['signal']}")
    except Exception as e:
        print(f"  ⚠️ Strategy execution simulation failed: {e}")
        return False
    
    print("✅ Automated Generation Test PASSED")
    return True

def main():
    """Main test function"""
    try:
        success = test_automated_generation()
        if success:
            print("🎉 All automated generation tests passed!")
            return 0
        else:
            print("❌ Some tests failed")
            return 1
    except Exception as e:
        print(f"❌ Test suite failed with error: {e}")
        return 1

if __name__ == "__main__":
    exit(main()) 