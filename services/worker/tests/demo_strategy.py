#!/usr/bin/env python3
"""
Demo Strategy Test - A simple demonstration of strategy functionality
"""

def test_demo_strategy():
    """
    Simple demo strategy that demonstrates basic functionality
    """
    print("🎮 Running Demo Strategy Test...")
    print("  📋 Testing basic strategy functionality...")
    
    # Mock data for demonstration
    mock_data = {
        'symbols': ['AAPL', 'GOOGL', 'MSFT', 'TSLA'],
        'prices': [150.0, 2800.0, 420.0, 250.0],
        'volumes': [1000000, 500000, 750000, 2000000]
    }
    
    print("  📊 Mock market data loaded")
    print(f"     Symbols: {len(mock_data['symbols'])}")
    print(f"     Price range: ${min(mock_data['prices']):.2f} - ${max(mock_data['prices']):.2f}")
    
    # Simple strategy: find stocks above $200
    high_price_stocks = []
    for i, symbol in enumerate(mock_data['symbols']):
        price = mock_data['prices'][i]
        if price > 200:
            high_price_stocks.append({
                'symbol': symbol,
                'price': price
            })
    
    print(f"  🎯 Found {len(high_price_stocks)} stocks above $200:")
    for stock in high_price_stocks:
        print(f"     {stock['symbol']}: ${stock['price']:.2f}")
    
    # Simple volume filter
    high_volume_stocks = []
    for i, symbol in enumerate(mock_data['symbols']):
        volume = mock_data['volumes'][i]
        if volume > 800000:
            high_volume_stocks.append({
                'symbol': symbol,
                'volume': volume
            })
    
    print(f"  📈 Found {len(high_volume_stocks)} stocks with high volume:")
    for stock in high_volume_stocks:
        print(f"     {stock['symbol']}: {stock['volume']:,}")
    
    # Combined filter
    combined_results = []
    for stock in high_price_stocks:
        for vol_stock in high_volume_stocks:
            if stock['symbol'] == vol_stock['symbol']:
                combined_results.append({
                    'symbol': stock['symbol'],
                    'price': stock['price'],
                    'volume': vol_stock['volume']
                })
    
    print(f"  🚀 Combined filter results: {len(combined_results)} stocks")
    for stock in combined_results:
        print(f"     {stock['symbol']}: ${stock['price']:.2f}, Volume: {stock['volume']:,}")
    
    print("  ✅ Demo strategy completed successfully")
    return True


def test_strategy_patterns():
    """
    Test common strategy patterns
    """
    print("\n🧪 Testing Strategy Patterns...")
    
    # Test gap detection logic
    def detect_gap(prev_close, current_open, threshold=2.0):
        if prev_close <= 0:
            return False
        gap_percent = ((current_open - prev_close) / prev_close) * 100
        return gap_percent > threshold
    
    # Test cases
    test_cases = [
        (100.0, 103.0, 2.0, True),   # 3% gap up - should trigger
        (100.0, 101.0, 2.0, False), # 1% gap up - should not trigger
        (100.0, 95.0, 2.0, False),  # Gap down - should not trigger
        (0.0, 103.0, 2.0, False),   # Invalid prev_close - should not trigger
    ]
    
    print("  🔍 Testing gap detection logic...")
    for i, (prev_close, current_open, threshold, expected) in enumerate(test_cases, 1):
        result = detect_gap(prev_close, current_open, threshold)
        status = "✅" if result == expected else "❌"
        print(f"     Test {i}: {status} Gap {prev_close} -> {current_open} = {result}")
        if result != expected:
            return False
    
    print("  ✅ All gap detection tests passed")
    return True


def main():
    """Main demo function"""
    print("🎮 DEMO STRATEGY TEST")
    print("=" * 50)
    
    # Run tests
    tests = [
        test_demo_strategy,
        test_strategy_patterns
    ]
    
    passed = 0
    total = len(tests)
    
    for test in tests:
        try:
            if test():
                passed += 1
                print("✅ PASSED")
            else:
                print("❌ FAILED")
        except Exception as e:
            print(f"❌ ERROR: {e}")
    
    print("\n" + "=" * 50)
    print(f"📊 Demo Results: {passed}/{total} tests passed")
    
    if passed == total:
        print("🎉 All demo tests completed successfully!")
        return True
    else:
        print("⚠️ Some demo tests failed")
        return False


if __name__ == "__main__":
    success = main()
    exit(0 if success else 1) 