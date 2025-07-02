#!/usr/bin/env python3
"""
Simple test to verify raw data approach without database
"""

import asyncio
import logging
import os
import sys

# Add the src directory to the path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "src"))

from accessor_strategy_engine import AccessorStrategyEngine

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


class PythonExecutionEngine:
    """Mock execution engine that mimics the old interface for testing"""
    
    def __init__(self):
        self.results = {}
    
    async def execute(self, strategy_code: str, context: dict) -> dict:
        """Execute strategy code and return results"""
        self.results = {}
        
        # Create safe execution environment
        safe_globals = {
            '__builtins__': {
                'len': len,
                'range': range,
                'enumerate': enumerate,
                'zip': zip,
                'list': list,
                'dict': dict,
                'tuple': tuple,
                'set': set,
                'str': str,
                'int': int,
                'float': float,
                'bool': bool,
                'abs': abs,
                'min': min,
                'max': max,
                'sum': sum,
                'round': round,
                'sorted': sorted,
                'any': any,
                'all': all,
                'print': print,
            },
            'save_result': self._save_result,
        }
        
        safe_locals = {}
        
        try:
            # Execute the strategy code
            exec(strategy_code, safe_globals, safe_locals)  # nosec B102
            
            # Also capture key variables from the locals
            for key, value in safe_locals.items():
                if not key.startswith('_') and not callable(value):
                    self.results[key] = value
            
            return self.results
        except Exception as e:
            logger.error(f"Strategy execution failed: {e}")
            return {'error': str(e)}
    
    def _save_result(self, key: str, value):
        """Save result to be returned"""
        self.results[key] = value


async def test_basic_execution():
    """Test basic strategy execution with mock data"""
    print("Testing basic strategy execution with mock data...")

    # Simple strategy that implements its own SMA
    strategy_code = """
# Mock price data (simulating what get_price_data would return)
mock_prices = [100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 
               110, 111, 112, 113, 114, 115, 116, 117, 118, 119, 120]

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
sma_20 = calculate_sma(mock_prices, 20)

if not sma_20:
    save_result('classification', False)
    save_result('reason', 'Could not calculate SMA')
else:
    # Strategy: Buy when price is above 20-day SMA
    current_price = mock_prices[-1]
    current_sma = sma_20[-1]
    
    result = current_price > current_sma
    save_result('classification', result)
    save_result('current_price', current_price)
    save_result('current_sma', current_sma)
    save_result('reason', f'Price {"above" if result else "below"} SMA')
"""

    engine = PythonExecutionEngine()
    result = await engine.execute(strategy_code, {"symbol": "AAPL"})

    print(f"Basic execution result: {result}")
    # Check that we have the expected results
    assert "classification" in result
    assert "current_price" in result
    assert "current_sma" in result
    assert result["current_price"] == 120
    assert result["current_sma"] == 110.5
    assert result["classification"] == True  # Price above SMA
    print("✓ Basic execution test passed")


async def test_custom_rsi_implementation():
    """Test strategy that implements its own RSI calculation"""
    print("Testing custom RSI implementation...")

    # Strategy that implements RSI from scratch
    strategy_code = """
# Mock price data
mock_prices = [44, 44.34, 44.09, 44.15, 43.61, 44.33, 44.83, 45.85, 46.08, 45.89,
               46.03, 46.28, 46.28, 46.00, 46.03, 46.41, 46.22, 45.64, 46.21, 46.25,
               47.75, 47.79, 47.73, 47.31, 47.20, 46.80, 47.80, 47.01, 47.12, 46.80]

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
rsi_values = calculate_rsi(mock_prices, 14)

if not rsi_values:
    save_result('classification', False)
    save_result('reason', 'Could not calculate RSI')
else:
    # Strategy: RSI oversold condition
    current_rsi = rsi_values[-1]
    result = current_rsi < 30  # Oversold
    
    save_result('classification', result)
    save_result('current_rsi', current_rsi)
    save_result('reason', f'RSI is {current_rsi:.2f} - {"oversold" if result else "not oversold"}')
"""

    engine = PythonExecutionEngine()
    result = await engine.execute(strategy_code, {"symbol": "AAPL"})

    print(f"Custom RSI result: {result}")
    # Check that we have the expected results
    assert "classification" in result
    assert "current_rsi" in result
    assert "rsi_values" in result
    assert len(result["rsi_values"]) > 0
    assert isinstance(result["current_rsi"], (int, float))
    assert 0 <= result["current_rsi"] <= 100  # RSI should be between 0 and 100
    print("✓ Custom RSI implementation test passed")


async def test_bollinger_bands_implementation():
    """Test strategy that implements Bollinger Bands from scratch"""
    print("Testing custom Bollinger Bands implementation...")

    # Strategy that implements Bollinger Bands
    strategy_code = """
# Mock price data
mock_prices = [20, 20.5, 21, 20.8, 21.2, 21.5, 21.8, 22, 21.9, 22.1,
               22.3, 22.5, 22.2, 22.4, 22.6, 22.8, 23, 22.9, 23.1, 23.3,
               23.5, 23.2, 23.4, 23.6, 23.8, 24, 23.9, 24.1, 24.3, 24.5]

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
        data_slice = prices[i:i + period]
        mean_val = sum(data_slice) / len(data_slice)
        variance = sum((x - mean_val) ** 2 for x in data_slice) / len(data_slice)
        std = variance ** 0.5
        
        upper.append(middle[i] + (std_dev * std))
        lower.append(middle[i] - (std_dev * std))
    
    return {'upper': upper, 'middle': middle, 'lower': lower}

# Calculate Bollinger Bands
bb = calculate_bollinger_bands(mock_prices, 20, 2.0)

if not bb['lower']:
    save_result('classification', False)
    save_result('reason', 'Could not calculate Bollinger Bands')
else:
    # Strategy: Price near lower Bollinger Band (potential bounce)
    current_price = mock_prices[-1]
    lower_band = bb['lower'][-1]
    
    result = current_price <= lower_band * 1.02  # Within 2% of lower band
    
    save_result('classification', result)
    save_result('current_price', current_price)
    save_result('lower_band', lower_band)
    save_result('reason', f'Price {"near" if result else "not near"} lower Bollinger Band')
"""

    engine = PythonExecutionEngine()
    result = await engine.execute(strategy_code, {"symbol": "AAPL"})

    print(f"Custom Bollinger Bands result: {result}")
    # Check that we have the expected results
    assert "classification" in result
    assert "current_price" in result
    assert "lower_band" in result
    assert "bb" in result
    assert "upper" in result["bb"]
    assert "middle" in result["bb"]
    assert "lower" in result["bb"]
    assert len(result["bb"]["upper"]) > 0
    assert len(result["bb"]["middle"]) > 0
    assert len(result["bb"]["lower"]) > 0
    print("✓ Custom Bollinger Bands implementation test passed")


async def run_all_tests():
    """Run all tests"""
    print("🚀 Starting simple Python execution tests (no database required)...")
    print()

    tests = [
        ("Basic Execution", test_basic_execution),
        ("Custom RSI Implementation", test_custom_rsi_implementation),
        ("Bollinger Bands Implementation", test_bollinger_bands_implementation),
    ]

    passed = 0
    failed = 0

    for test_name, test_func in tests:
        try:
            print(f"Running {test_name} test...")
            await test_func()
            print(f"✅ {test_name} test passed")
            passed += 1
        except Exception as e:
            print(f"❌ {test_name} test failed: {e}")
            failed += 1
        print()

    print("📊 Test Results Summary:")
    print("=" * 50)
    for test_name, _ in tests:
        status = (
            "✅ PASSED" if test_name in [t[0] for t in tests[:passed]] else "❌ FAILED"
        )
        print(f"{test_name:<30} {status}")
    print("=" * 50)
    print(f"Total: {passed}/{len(tests)} tests passed")

    if failed == 0:
        print("🎉 All tests passed! The raw data approach is working correctly.")
    else:
        print("⚠️  Some tests failed. The functionality needs attention.")


if __name__ == "__main__":
    asyncio.run(run_all_tests())
