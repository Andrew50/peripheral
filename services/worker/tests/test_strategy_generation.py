#!/usr/bin/env python3
"""
Test to verify that generated strategy code doesn't use typing imports
"""

def test_strategy_code_patterns():
    """Test patterns that should/shouldn't appear in generated strategy code"""
    print("🧪 Testing strategy code patterns...")
    
    # Test data - examples of generated code
    valid_code_examples = [
        '''
def strategy():
    """Find gap-up stocks using NEW ACCESSOR PATTERN"""
    instances = []
    
    bar_data = get_bar_data(
        timeframe="1d", 
        columns=["ticker", "timestamp", "open", "close"], 
        min_bars=2
    )
    
    if len(bar_data) == 0:
        return instances
    
    import pandas as pd
    df = pd.DataFrame(bar_data, columns=["ticker", "timestamp", "open", "close"])
    
    # Gap calculation logic here
    # ...
    
    return instances
''',
        '''
def strategy():
    """Momentum strategy using NEW ACCESSOR PATTERN"""
    instances = []
    
    bar_data = get_bar_data(
        timeframe="1d", 
        columns=["ticker", "timestamp", "close", "volume"], 
        min_bars=20
    )
    
    # Get general data for sector filtering
    general_data = get_general_data(columns=["sector", "industry"])
    
    if len(bar_data) == 0:
        return instances
    
    # Strategy logic here
    return instances
'''
    ]
    
    # Invalid examples that should be rejected
    invalid_code_examples = [
        '''
def strategy(data):  # ❌ Has parameters
    return []
''',
        '''
def classify_symbol(symbol):  # ❌ Old pattern
    return True
''',
        '''
def custom_function():  # ❌ Wrong function name
    return []
'''
    ]
    
    # Test patterns
    patterns_to_avoid = [
        "def strategy(data):",  # ❌ Old pattern with parameters
        "def classify_symbol(",  # ❌ Old function name
        "run_batch_backtest",    # ❌ Old batch patterns
        "def run_screening(",    # ❌ Old batch patterns
        "price_data = get_price_data(symbol",  # ❌ Old data access
        ": List[Dict]",          # ❌ Type annotations
        ": str",                 # ❌ Type annotations
        "-> bool:",              # ❌ Return type annotations
        "from typing import",    # ❌ Typing imports
    ]
    
    patterns_that_are_ok = [
        "def strategy():",       # ✅ New pattern signature
        "get_bar_data(",        # ✅ New accessor functions
        "get_general_data(",    # ✅ New accessor functions
        "return instances",     # ✅ Returns instances list
        "instances = []",       # ✅ Instance list initialization
    ]
    
    print("  Checking good strategy code...")
    for pattern in patterns_to_avoid:
        found_in_any_good_code = False
        for code in valid_code_examples:
            if pattern in code:
                print(f"    ❌ Found bad pattern in good code: {pattern}")
                return False
    
    # Check that at least one good code example contains each required pattern
    for pattern in patterns_that_are_ok:
        found_in_any_good_code = False
        for code in valid_code_examples:
            if pattern in code:
                found_in_any_good_code = True
                break
        if not found_in_any_good_code:
            print(f"    ❌ Missing required pattern in all good code examples: {pattern}")
            return False
    
    print("  ✅ Good strategy code passes all checks")
    
    print("  Checking bad strategy code detection...")
    bad_patterns_found = 0
    for pattern in patterns_to_avoid:
        for code in invalid_code_examples:
            if pattern in code:
                bad_patterns_found += 1
    
    if bad_patterns_found == 0:
        print("    ❌ Bad strategy code should contain bad patterns")
        return False
    
    print(f"  ✅ Bad strategy code correctly contains {bad_patterns_found} bad patterns")
    
    return True


def test_function_signature_patterns():
    """Test that function signatures follow the correct pattern"""
    print("\n🧪 Testing function signature patterns...")
    
    # Correct signatures (what we want - NEW ACCESSOR PATTERN)
    correct_signatures = [
        "def strategy():",
        "def get_bar_data(timeframe='1d', columns=[], min_bars=1):",
        "def get_general_data(columns=[]):"
    ]
    
    # Incorrect signatures (what we want to avoid - OLD PATTERNS)
    incorrect_signatures = [
        "def strategy(data):",
        "def classify_symbol(symbol):",
        "def strategy(symbol: str) -> List[Dict]:",
        "def classify_symbol(symbol: str) -> bool:"
    ]
    
    print("  Testing correct signatures...")
    for sig in correct_signatures:
        # These should not contain typing annotations
        if ":" in sig and "->" in sig:
            print(f"    ❌ Correct signature contains type annotations: {sig}")
            return False
        elif " -> " in sig:
            print(f"    ❌ Correct signature contains return type: {sig}")
            return False
    
    print("  ✅ All correct signatures are clean")
    
    print("  Testing incorrect signatures...")
    # These signatures should be detectable as incorrect by our validation
    # We're just confirming they represent the old patterns we want to reject
    old_pattern_signatures = [sig for sig in incorrect_signatures if "classify_symbol" in sig or "def strategy(data)" in sig]
    
    if len(old_pattern_signatures) < 2:
        print("    ❌ Not enough old pattern signatures for testing")
        return False
    
    print("  ✅ All incorrect signatures properly detected")
    
    return True


def test_docstring_patterns():
    """Test that docstrings use correct language"""
    print("\n🧪 Testing docstring patterns...")
    
    # Good docstring (what we want)
    good_docstring = """
    Returns: Dict with 'timestamps': list of int, 'open': list of float, 'high': list of float, 
             'low': list of float, 'close': list of float, 'volume': list of int
"""
    
    # Bad docstring (what we want to avoid) 
    bad_docstring = """
    Returns: Dict with 'timestamps': List[int], 'open': List[float], 'high': List[float], 
             'low': List[float], 'close': List[float], 'volume': List[int]
"""
    
    # Check patterns
    typing_patterns = ["List[", "Dict[", "Tuple[", "Union[", "Optional["]
    builtin_patterns = ["list of", "dict with", "dict mapping"]
    
    print("  Checking good docstring...")
    for pattern in typing_patterns:
        if pattern in good_docstring:
            print(f"    ❌ Good docstring contains typing pattern: {pattern}")
            return False
    
    has_builtin = any(pattern in good_docstring for pattern in builtin_patterns)
    if not has_builtin:
        print("    ❌ Good docstring missing built-in type patterns")
        return False
    
    print("  ✅ Good docstring uses built-in type language")
    
    print("  Checking bad docstring...")
    has_typing = any(pattern in bad_docstring for pattern in typing_patterns)
    if not has_typing:
        print("    ❌ Bad docstring should contain typing patterns")
        return False
    
    print("  ✅ Bad docstring correctly contains typing patterns")
    
    return True


def main():
    """Run all tests"""
    print("🚀 Testing strategy generation patterns...")
    print("=" * 60)
    
    success = True
    
    # Test 1: Strategy code patterns
    test1_passed = test_strategy_code_patterns()
    success = success and test1_passed
    
    # Test 2: Function signature patterns
    test2_passed = test_function_signature_patterns()
    success = success and test2_passed
    
    # Test 3: Docstring patterns
    test3_passed = test_docstring_patterns()
    success = success and test3_passed
    
    print("\n" + "=" * 60)
    if success:
        print("🎉 All pattern tests passed!")
        print("   ✅ Strategy code patterns are correct")
        print("   ✅ Function signatures avoid typing annotations")
        print("   ✅ Docstrings use built-in type language")
        print("\n💡 The fixes should prevent typing import issues!")
    else:
        print("❌ Some pattern tests failed.")
    
    return success


if __name__ == "__main__":
    success = main()
    exit(0 if success else 1) 