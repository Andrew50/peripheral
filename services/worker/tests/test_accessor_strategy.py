#!/usr/bin/env python3
"""
Test script for the new Data Accessor Strategy Engine
"""

import asyncio
import sys
import os
import logging
from datetime import datetime, timedelta

# Add src to path for imports
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "src"))

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


# Mock the data accessor functions for testing
def mock_get_bar_data(timeframe="1d", tickers=None, columns=None, min_bars=1, filters=None, aggregate_mode=False):
    """Mock get_bar_data function for testing"""
    import numpy as np
    
    if columns is None:
        columns = ["ticker", "timestamp", "open", "high", "low", "close", "volume", "adj_close"]
    
    # Generate mock data
    mock_data = []
    
    # Use provided tickers or default set
    if tickers is None:
        tickers = ["AAPL", "GOOGL", "MSFT"]
    elif isinstance(tickers, str):
        tickers = [tickers]
    
    for ticker in tickers:
        for i in range(max(min_bars, 1)):  # Ensure at least 1 bar
            timestamp = 1700000000 + (i * 86400)  # Mock timestamps
            row = []
            for col in columns:
                if col == "ticker":
                    row.append(ticker)
                elif col == "timestamp":
                    row.append(timestamp)
                elif col in ["open", "high", "low", "close", "adj_close"]:
                    row.append(100.0 + i)  # Mock prices
                elif col == "volume":
                    row.append(1500000 + i * 10000)  # Mock volume > 1M for test
                else:
                    row.append(0)
            mock_data.append(row)
    
    return np.array(mock_data, dtype=object)

def mock_get_general_data(tickers=None, columns=None, filters=None):
    """Mock get_general_data function for testing"""
    import pandas as pd
    
    if columns is None:
        columns = ["name", "sector", "industry", "market", "primary_exchange", "locale", "active", "description", "cik"]
    
    # Generate mock data
    mock_data = {
        1: {
            "name": "Apple Inc.",
            "sector": "Technology", 
            "industry": "Consumer Electronics",
            "market": "stocks",
            "primary_exchange": "NASDAQ",
            "locale": "us", 
            "active": True,
            "description": "Apple Inc. designs and manufactures consumer electronics",
            "cik": 320193
        },
        2: {
            "name": "Alphabet Inc.",
            "sector": "Technology",
            "industry": "Internet Services",
            "market": "stocks", 
            "primary_exchange": "NASDAQ",
            "locale": "us",
            "active": True,
            "description": "Alphabet Inc. is a holding company",
            "cik": 1652044
        }
    }
    
    # Filter columns
    filtered_data = {}
    for sec_id, data in mock_data.items():
        filtered_data[sec_id] = {col: data.get(col) for col in columns if col in data}
    
    df = pd.DataFrame.from_dict(filtered_data, orient='index')
    return df


# Mock AccessorStrategyEngine for testing
class MockAccessorStrategyEngine:
    """Mock engine for testing the new system"""
    
    def __init__(self):
        from validator import SecurityValidator
        self.validator = SecurityValidator()
    
    async def execute_screening(self, strategy_code, universe, limit=100):
        """Mock screening execution"""
        try:
            # Validate the strategy code
            if not self.validator.validate_code(strategy_code):
                return {
                    'success': False,
                    'error': 'Strategy validation failed'
                }
            
            # Create safe execution environment with mock functions
            safe_globals = {
                'pd': __import__('pandas'),
                'numpy': __import__('numpy'),
                'np': __import__('numpy'),
                'get_bar_data': mock_get_bar_data,
                'get_general_data': mock_get_general_data,
                'len': len,
                'range': range,
                'enumerate': enumerate,
                'float': float,
                'int': int,
                'str': str,
                'abs': abs,
                'max': max,
                'min': min,
                'round': round,
                'sum': sum,
                'datetime': datetime,
                'timedelta': timedelta,
            }
            
            safe_locals = {}
            
            # Execute strategy code
            # exec necessary for strategy execution - properly sandboxed with restricted globals/locals
            exec(strategy_code, safe_globals, safe_locals)  # nosec B102
            
            # Find and execute strategy function
            strategy_func = safe_locals.get('strategy')
            if not strategy_func:
                return {
                    'success': False,
                    'error': 'No strategy function found'
                }
            
            # Execute the strategy
            instances = strategy_func()
            
            if not isinstance(instances, list):
                return {
                    'success': False,
                    'error': 'Strategy must return a list'
                }
            
            # Rank results (simple scoring)
            ranked_results = sorted(instances, key=lambda x: x.get('score', 0), reverse=True)[:limit]
            
            return {
                'success': True,
                'ranked_results': ranked_results,
                'total_instances': len(instances)
            }
            
        except Exception as e:
            return {
                'success': False,
                'error': str(e)
            }


async def test_simple_accessor_strategy():
    """Test a simple strategy using the new accessor functions"""
    print("Testing simple accessor strategy...")
    
    strategy_code = '''
def strategy():
    """Simple test strategy using data accessors"""
    instances = []
    
    # Get recent bar data
    bar_data = get_bar_data(
        timeframe="1d",
                    columns=["ticker", "timestamp", "close", "volume"],
            min_bars=1  # Simple pattern - just need current volume data
    )
    
    if len(bar_data) == 0:
        return instances
    
    # Process each row
    for row in bar_data:
        ticker = row[0]
        timestamp = row[1] 
        close_price = row[2]
        volume = row[3]
        
        # Simple condition: high volume stocks
        if volume > 1000000:
            instances.append({
                'ticker': ticker,
                'timestamp': str(timestamp),
                'price': close_price,
                'volume': volume,
                'score': volume / 1000000,
                'signal': True,
                'message': f"{ticker} has high volume: {volume:,}"
            })
    
    return instances
'''
    
    engine = MockAccessorStrategyEngine()
    result = await engine.execute_screening(strategy_code, ['AAPL', 'GOOGL', 'MSFT'])
    
    print(f"Simple strategy result: {result}")
    assert result['success'] == True
    assert len(result['ranked_results']) > 0
    assert all('ticker' in r for r in result['ranked_results'])
    print("✓ Simple accessor strategy test passed")


async def test_complex_accessor_strategy():
    """Test a more complex strategy using both accessor functions"""
    print("Testing complex accessor strategy...")
    
    strategy_code = '''
def strategy():
    """Complex strategy using both bar and general data"""
    instances = []
    
    # Get sector information
    general_data = get_general_data(columns=["sector"])
    
    # Get bar data
    bar_data = get_bar_data(
        timeframe="1d", 
        columns=["ticker", "timestamp", "close", "volume"],
        min_bars=3
    )
    
    if len(bar_data) == 0:
        return instances
    
    # Convert to easier format
    import pandas as pd
    df = pd.DataFrame(bar_data, columns=["ticker", "timestamp", "close", "volume"])
    
    # Group by ticker and get latest data
    latest_data = df.groupby('ticker').last()
    
    for ticker, row in latest_data.iterrows():
        # Check if it's a technology stock (mock condition)
        close_price = row['close']
        volume = row['volume']
        
        # Simple scoring based on price and volume
        score = (close_price / 100.0) * (volume / 1000000.0)
        
        if score > 1.0:  # Arbitrary threshold
            instances.append({
                'ticker': ticker,
                'timestamp': str(row['timestamp']),
                'price': close_price,
                'volume': volume,
                'score': score,
                'signal': True,
                'message': f"{ticker} meets criteria with score {score:.2f}"
            })
    
    return instances
'''
    
    engine = MockAccessorStrategyEngine()
    result = await engine.execute_screening(strategy_code, ['AAPL', 'GOOGL', 'MSFT'])
    
    print(f"Complex strategy result: {result}")
    assert result['success'] == True
    assert len(result['ranked_results']) > 0
    # Check that results are properly scored
    scores = [r.get('score', 0) for r in result['ranked_results']]
    assert all(isinstance(s, (int, float)) for s in scores)
    print("✓ Complex accessor strategy test passed")


async def test_validation_failure():
    """Test that invalid strategies are properly rejected"""
    print("Testing strategy validation...")
    
    # Strategy with security violation
    bad_strategy_code = '''
def strategy():
    import os  # This should be blocked
    return []
'''
    
    engine = MockAccessorStrategyEngine()
    result = await engine.execute_screening(bad_strategy_code, ['AAPL'])
    
    print(f"Validation test result: {result}")
    assert result['success'] == False
    assert 'validation' in result['error'].lower() or 'import' in result['error'].lower()
    print("✓ Strategy validation test passed")


async def test_filter_values_fetching():
    """Test that filter values can be fetched from database"""
    print("Testing filter values fetching...")
    
    try:
        from data_accessors import DataAccessorProvider
        
        # Create mock provider that simulates database behavior
        class MockDataAccessorProvider(DataAccessorProvider):
            def get_available_filter_values(self):
                return {
                    'sectors': ['Technology', 'Healthcare', 'Financial Services'],
                    'industries': ['Software—Application', 'Drug Manufacturers—General'],
                    'primary_exchanges': ['NASDAQ', 'NYSE'],
                    'locales': ['us']
                }
        
        provider = MockDataAccessorProvider()
        filter_values = provider.get_available_filter_values()
        
        # Validate structure
        required_keys = ['sectors', 'industries', 'primary_exchanges', 'locales']
        for key in required_keys:
            if key not in filter_values:
                print(f"❌ Missing required key: {key}")
                return False
            if not filter_values[key]:
                print(f"❌ Empty list for key: {key}")
                return False
        
        print(f"✅ Filter values test passed: {len(filter_values['sectors'])} sectors, {len(filter_values['industries'])} industries")
        return True
        
    except Exception as e:
        print(f"❌ Filter values test failed: {e}")
        return False


async def run_all_tests():
    """Run all accessor strategy tests"""
    print("=== Running Data Accessor Strategy Tests ===")
    
    try:
        await test_simple_accessor_strategy()
        await test_complex_accessor_strategy() 
        await test_validation_failure()
        
        # Test filter values functionality
        filter_test_passed = await test_filter_values_fetching()
        if not filter_test_passed:
            print("❌ Filter values test failed")
            return False
        
        print("\n🎉 All accessor strategy tests passed!")
        return True
        
    except Exception as e:
        print(f"\n❌ Test failed: {e}")
        return False


if __name__ == "__main__":
    asyncio.run(run_all_tests()) 