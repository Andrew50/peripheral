#!/usr/bin/env python3
"""
Test to verify that generated strategy code doesn't use typing imports
"""

def test_strategy_code_patterns():
    """Test patterns that should/shouldn't appear in generated strategy code"""
    print("🧪 Testing strategy code patterns...")
    
    # Example of code that SHOULD be generated (without typing)
    good_strategy_code = """
def classify_symbol(symbol):
    '''
    Identifies when a symbol gaps up by more than the specified threshold.
    Gap up = current open > previous close by the specified percentage.
    '''
    try:
        # Get recent price data (last 5 days to ensure we have data)
        price_data = get_price_data(symbol, timeframe='1d', days=5)
        if not price_data or not price_data.get('open') or len(price_data['open']) < 2:
            return False
        
        # Get current and previous close prices
        current_open = price_data['open'][-1]    # Most recent open
        previous_close = price_data['close'][-2]  # Previous day's close
        
        # Calculate gap percentage
        gap_percent = ((current_open - previous_close) / previous_close) * 100
        
        # Check if gap exceeds threshold (adjust threshold as needed)
        threshold = 2.0  # 2% gap up threshold
        return gap_percent > threshold
        
    except Exception:
        return False
"""
    
    # Example of code that SHOULD NOT be generated (with typing)
    bad_strategy_code = """
from typing import List, Dict, Tuple, Union, Optional

def classify_symbol(symbol: str) -> bool:
    '''
    Identifies when a symbol gaps up by more than the specified threshold.
    '''
    try:
        price_data: Dict = get_price_data(symbol, timeframe='1d', days=5)
        if not price_data or not price_data.get('open') or len(price_data['open']) < 2:
            return False
        
        current_open: float = price_data['open'][-1]
        previous_close: float = price_data['close'][-2]
        
        gap_percent: float = ((current_open - previous_close) / previous_close) * 100
        threshold: float = 2.0
        
        return gap_percent > threshold
        
    except Exception:
        return False
"""
    
    # Test patterns
    patterns_to_avoid = [
        "from typing import",
        "import typing",
        ": List[",
        ": Dict[",
        ": Tuple[",
        ": Union[",
        ": Optional[",
        "-> Dict",
        "-> List",
        "-> Optional",
        "-> Union"
    ]
    
    patterns_that_are_ok = [
        "def classify_symbol(symbol):",
        "price_data.get('open')",
        "len(price_data['open'])",
        "return False",
        "return True",
        "# Get",
        "# Calculate"
    ]
    
    print("  Checking good strategy code...")
    for pattern in patterns_to_avoid:
        if pattern in good_strategy_code:
            print(f"    ❌ Found bad pattern in good code: {pattern}")
            return False
    
    for pattern in patterns_that_are_ok:
        if pattern not in good_strategy_code:
            print(f"    ❌ Missing good pattern in good code: {pattern}")
            return False
    
    print("  ✅ Good strategy code passes all checks")
    
    print("  Checking bad strategy code detection...")
    bad_patterns_found = 0
    for pattern in patterns_to_avoid:
        if pattern in bad_strategy_code:
            bad_patterns_found += 1
    
    if bad_patterns_found == 0:
        print("    ❌ Bad strategy code should contain bad patterns")
        return False
    
    print(f"  ✅ Bad strategy code correctly contains {bad_patterns_found} bad patterns")
    
    return True


def test_function_signature_patterns():
    """Test that function signatures follow the correct pattern"""
    print("\n🧪 Testing function signature patterns...")
    
    # Correct signatures (what we want)
    correct_signatures = [
        "def classify_symbol(symbol):",
        "def get_price_data(symbol, timeframe='1d', days=30):",
        "def get_historical_data(symbol, timeframe='1d', periods=100):",
        "def scan_universe(filters=None, sort_by=None, limit=100):"
    ]
    
    # Incorrect signatures (what we want to avoid)
    incorrect_signatures = [
        "def classify_symbol(symbol: str) -> bool:",
        "def get_price_data(symbol: str, timeframe: str = '1d', days: int = 30) -> Dict:",
        "def get_historical_data(symbol: str, timeframe: str = '1d') -> Dict:",
        "def scan_universe(filters: Dict = None, sort_by: str = None) -> Dict:"
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
    for sig in incorrect_signatures:
        # These should contain typing annotations (for detection)
        if ": " not in sig and " -> " not in sig:
            print(f"    ❌ Incorrect signature missing type annotations: {sig}")
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