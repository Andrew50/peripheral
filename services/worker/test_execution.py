#!/usr/bin/env python3
"""
Test script to verify Python execution functionality
"""

import asyncio
import logging
import sys
import os

# Add the src directory to the path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'src'))

from src.execution_engine import PythonExecutionEngine
from src.security_validator import SecurityValidator
from src.data_provider import DataProvider

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


async def test_basic_execution():
    """Test basic strategy execution with raw data only"""
    print("Testing basic strategy execution...")
    
    # Simple strategy that uses raw data and implements its own SMA
    strategy_code = """
# Get symbol from context
symbol = input_data.get('symbol', 'AAPL')

# Get raw price data
price_data = get_price_data(symbol, timeframe='1d', days=50)

if not price_data.get('close') or len(price_data['close']) < 20:
    save_result('classification', False)
    save_result('reason', 'Insufficient price data')
else:
    # Implement Simple Moving Average ourselves
    def calculate_sma(prices, period):
        if len(prices) < period:
            return []
        sma = []
        for i in range(period - 1, len(prices)):
            avg = sum(prices[i - period + 1:i + 1]) / period
            sma.append(avg)
        return sma
    
    # Calculate 20-day SMA
    closes = price_data['close']
    sma_20 = calculate_sma(closes, 20)
    
    if not sma_20:
        save_result('classification', False)
        save_result('reason', 'Could not calculate SMA')
    else:
        # Strategy: Buy when price is above 20-day SMA
        current_price = closes[-1]
        current_sma = sma_20[-1]
        
        result = current_price > current_sma
        save_result('classification', result)
        save_result('current_price', current_price)
        save_result('sma_20', current_sma)
        save_result('reason', f'Price {"above" if result else "below"} SMA')
"""
    
    engine = PythonExecutionEngine()
    result = await engine.execute(strategy_code, {'symbol': 'AAPL'})
    
    print(f"Basic execution result: {result}")
    assert result['success'] == True
    assert 'result' in result
    print("✓ Basic execution test passed")


async def test_data_functions():
    """Test that raw data accessor functions work"""
    print("Testing raw data accessor functions...")
    
    # Strategy that tests multiple raw data functions
    strategy_code = """
def classify_symbol(symbol):
    # Test raw price data
    price_data = get_price_data(symbol, timeframe='1d', days=30)
    if not price_data['close']:
        return False
    
    # Test security info
    info = get_security_info(symbol)
    if not info:
        return False
    
    # Test fundamental data
    fundamentals = get_fundamental_data(symbol, ['market_cap', 'eps'])
    
    # Test volume data
    volume_data = get_volume_data(symbol, days=30)
    
    # Test utility functions
    returns = calculate_returns(price_data['close'], periods=1)
    
    # Simple strategy: return True if we have data
    return len(price_data['close']) > 10 and len(returns) > 5
"""
    
    engine = PythonExecutionEngine()
    result = await engine.execute(strategy_code, {'symbol': 'AAPL'})
    
    print(f"Data functions result: {result}")
    assert result['success'] == True
    print("✓ Data functions test passed")


async def test_custom_rsi_implementation():
    """Test strategy that implements its own RSI calculation"""
    print("Testing custom RSI implementation...")
    
    # Strategy that implements RSI from scratch
    strategy_code = """
def classify_symbol(symbol):
    # Get raw price data
    price_data = get_price_data(symbol, timeframe='1d', days=50)
    
    if not price_data['close'] or len(price_data['close']) < 15:
        return False
    
    # Implement RSI calculation ourselves
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
    
    # Calculate RSI
    closes = price_data['close']
    rsi_values = calculate_rsi(closes, 14)
    
    if not rsi_values:
        return False
    
    # Strategy: RSI oversold condition
    current_rsi = rsi_values[-1]
    return current_rsi < 30  # Oversold
"""
    
    engine = PythonExecutionEngine()
    result = await engine.execute(strategy_code, {'symbol': 'AAPL'})
    
    print(f"Custom RSI result: {result}")
    assert result['success'] == True
    print("✓ Custom RSI implementation test passed")


async def test_bollinger_bands_implementation():
    """Test strategy that implements Bollinger Bands from scratch"""
    print("Testing custom Bollinger Bands implementation...")
    
    # Strategy that implements Bollinger Bands
    strategy_code = """
def classify_symbol(symbol):
    # Get raw price data
    price_data = get_price_data(symbol, timeframe='1d', days=50)
    
    if not price_data['close'] or len(price_data['close']) < 20:
        return False
    
    # Implement Bollinger Bands calculation
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
            # Get the data slice for standard deviation calculation
            data_slice = prices[i + period - len(middle):i + period]
            mean_val = sum(data_slice) / len(data_slice)
            variance = sum((x - mean_val) ** 2 for x in data_slice) / len(data_slice)
            std = variance ** 0.5
            
            upper.append(middle[i] + (std_dev * std))
            lower.append(middle[i] - (std_dev * std))
        
        return {'upper': upper, 'middle': middle, 'lower': lower}
    
    # Calculate Bollinger Bands
    closes = price_data['close']
    bb = calculate_bollinger_bands(closes, 20, 2.0)
    
    if not bb['lower']:
        return False
    
    # Strategy: Price near lower Bollinger Band (potential bounce)
    current_price = closes[-1]
    lower_band = bb['lower'][-1]
    
    return current_price <= lower_band * 1.02  # Within 2% of lower band
"""
    
    engine = PythonExecutionEngine()
    result = await engine.execute(strategy_code, {'symbol': 'AAPL'})
    
    print(f"Custom Bollinger Bands result: {result}")
    assert result['success'] == True
    print("✓ Custom Bollinger Bands implementation test passed")


async def test_security_validation():
    """Test security validation"""
    print("Testing security validation...")
    
    validator = SecurityValidator()
    
    # Test safe code
    safe_code = """
def classify_symbol(symbol):
    price_data = get_price_data(symbol)
    return len(price_data.get('close', [])) > 0
"""
    
    # Test unsafe code
    unsafe_code = """
import os
os.system('rm -rf /')
"""
    
    safe_result = validator.validate_code(safe_code)
    unsafe_result = validator.validate_code(unsafe_code)
    
    if safe_result and not unsafe_result:
        print("✅ Security validation test passed")
        return True
    else:
        print(f"❌ Security validation test failed: safe={safe_result}, unsafe={unsafe_result}")
        return False


async def test_data_provider():
    """Test data provider directly"""
    print("Testing data provider...")
    
    provider = DataProvider()
    
    try:
        # Test basic SQL execution
        result = await provider.execute_sql("SELECT 1 as test_column")
        if result and result.get('data'):
            print(f"✅ Data provider SQL test passed: {result}")
            return True
        else:
            print(f"❌ Data provider SQL test failed: {result}")
            return False
    except Exception as e:
        print(f"❌ Data provider test failed: {e}")
        return False


async def run_all_tests():
    """Run all tests"""
    print("🚀 Starting Python execution functionality tests...\n")
    
    tests = [
        ("Security Validation", test_security_validation),
        ("Data Provider", test_data_provider),
        ("Data Functions", test_data_functions),
        ("Basic Execution", test_basic_execution),
        ("Custom RSI Implementation", test_custom_rsi_implementation),
        ("Bollinger Bands Implementation", test_bollinger_bands_implementation),
    ]
    
    results = []
    for test_name, test_func in tests:
        print(f"Running {test_name} test...")
        try:
            result = await test_func()
            results.append((test_name, result))
        except Exception as e:
            print(f"❌ {test_name} test crashed: {e}")
            results.append((test_name, False))
        print()
    
    # Summary
    print("📊 Test Results Summary:")
    print("=" * 50)
    passed = 0
    total = len(results)
    
    for test_name, result in results:
        status = "✅ PASSED" if result else "❌ FAILED"
        print(f"{test_name:<25} {status}")
        if result:
            passed += 1
    
    print("=" * 50)
    print(f"Total: {passed}/{total} tests passed")
    
    if passed == total:
        print("🎉 All tests passed! The Python execution functionality is working correctly.")
        return True
    else:
        print("⚠️  Some tests failed. The functionality needs attention.")
        return False


if __name__ == "__main__":
    asyncio.run(run_all_tests()) 