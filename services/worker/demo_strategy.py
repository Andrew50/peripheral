#!/usr/bin/env python3
"""
Demo: Creating and Testing Custom Python Trading Strategies

This demonstrates how to write Python strategies that implement their own
technical indicators using raw market data.
"""

import asyncio
import sys
import os

# Add the src directory to the path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'src'))

from src.execution_engine import PythonExecutionEngine


async def demo_mean_reversion_strategy():
    """Demo: Mean reversion strategy using custom RSI and Bollinger Bands"""
    print("📈 Demo: Mean Reversion Strategy")
    print("-" * 50)
    
    strategy_code = """
# Mean Reversion Strategy
# Buy when RSI < 30 AND price is below lower Bollinger Band
# This combines two oversold indicators for stronger signals

# Mock market data (normally would come from get_price_data())
mock_prices = [
    50.0, 49.5, 48.8, 48.2, 47.1, 46.5, 45.8, 45.2, 44.6, 44.0,  # Downtrend
    43.5, 43.8, 44.1, 44.5, 45.0, 45.5, 46.0, 46.8, 47.5, 48.2,  # Recovery
    49.0, 49.5, 50.1, 50.8, 51.2, 51.5, 51.8, 52.0, 52.2, 52.5   # Uptrend
]

def calculate_rsi(prices, period=14):
    \"\"\"Calculate Relative Strength Index\"\"\"
    if len(prices) < period + 1:
        return []
    
    deltas = [prices[i] - prices[i-1] for i in range(1, len(prices))]
    gains = [d if d > 0 else 0 for d in deltas]
    losses = [-d if d < 0 else 0 for d in deltas]
    
    avg_gain = sum(gains[:period]) / period
    avg_loss = sum(losses[:period]) / period
    
    rsi = []
    for i in range(period, len(gains)):
        if avg_loss == 0:
            rsi.append(100)
        else:
            rs = avg_gain / avg_loss
            rsi.append(100 - (100 / (1 + rs)))
        
        avg_gain = (avg_gain * (period - 1) + gains[i]) / period
        avg_loss = (avg_loss * (period - 1) + losses[i]) / period
    
    return rsi

def calculate_bollinger_bands(prices, period=20, std_dev=2.0):
    \"\"\"Calculate Bollinger Bands\"\"\"
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
        data_slice = prices[i:i + period]
        mean_val = sum(data_slice) / len(data_slice)
        variance = sum((x - mean_val) ** 2 for x in data_slice) / len(data_slice)
        std = variance ** 0.5
        
        upper.append(middle[i] + (std_dev * std))
        lower.append(middle[i] - (std_dev * std))
    
    return {'upper': upper, 'middle': middle, 'lower': lower}

# Calculate indicators
rsi_values = calculate_rsi(mock_prices, 14)
bb = calculate_bollinger_bands(mock_prices, 20, 2.0)

# Strategy logic
if rsi_values and bb['lower']:
    current_price = mock_prices[-1]
    current_rsi = rsi_values[-1]
    lower_band = bb['lower'][-1]
    middle_band = bb['middle'][-1]
    
    # Mean reversion signals
    rsi_oversold = current_rsi < 30
    price_below_bb = current_price < lower_band
    
    # Combined signal
    buy_signal = rsi_oversold and price_below_bb
    
    save_result('classification', buy_signal)
    save_result('current_price', current_price)
    save_result('current_rsi', current_rsi)
    save_result('lower_band', lower_band)
    save_result('middle_band', middle_band)
    save_result('rsi_oversold', rsi_oversold)
    save_result('price_below_bb', price_below_bb)
    save_result('signal_strength', 'STRONG' if buy_signal else 'WEAK')
    save_result('reason', f'RSI: {current_rsi:.1f}, Price vs BB: {current_price:.2f} vs {lower_band:.2f}')
else:
    save_result('classification', False)
    save_result('reason', 'Insufficient data for analysis')
"""
    
    engine = PythonExecutionEngine()
    result = await engine.execute(strategy_code, {'symbol': 'DEMO'})
    
    print("Strategy Results:")
    print(f"  📊 Classification: {result.get('classification')}")
    print(f"  💰 Current Price: ${result.get('current_price', 0):.2f}")
    print(f"  📈 RSI: {result.get('current_rsi', 0):.1f}")
    print(f"  📉 Lower BB: ${result.get('lower_band', 0):.2f}")
    print(f"  🎯 Signal Strength: {result.get('signal_strength', 'N/A')}")
    print(f"  💭 Reason: {result.get('reason', 'N/A')}")
    print()


async def demo_momentum_strategy():
    """Demo: Momentum strategy using custom moving averages"""
    print("🚀 Demo: Momentum Strategy")
    print("-" * 50)
    
    strategy_code = """
# Momentum Strategy
# Buy when short-term MA crosses above long-term MA (Golden Cross)

# Mock price data showing a golden cross pattern
mock_prices = [
    100, 101, 102, 103, 104, 105, 104, 103, 102, 101,  # Sideways
    100, 99, 98, 97, 96, 95, 96, 97, 98, 99,           # Dip and recovery  
    100, 101, 102, 103, 104, 105, 106, 107, 108, 109, # Strong uptrend
    110, 111, 112, 113, 114, 115, 116, 117, 118, 119  # Continued uptrend
]

def calculate_sma(prices, period):
    \"\"\"Calculate Simple Moving Average\"\"\"
    if len(prices) < period:
        return []
    return [sum(prices[i-period+1:i+1])/period for i in range(period-1, len(prices))]

def calculate_ema(prices, period):
    \"\"\"Calculate Exponential Moving Average\"\"\"
    if len(prices) < period:
        return []
    
    # Calculate smoothing factor
    alpha = 2 / (period + 1)
    ema = [sum(prices[:period]) / period]  # Start with SMA
    
    for i in range(period, len(prices)):
        ema.append(alpha * prices[i] + (1 - alpha) * ema[-1])
    
    return ema

# Calculate moving averages
sma_10 = calculate_sma(mock_prices, 10)
sma_20 = calculate_sma(mock_prices, 20)
ema_12 = calculate_ema(mock_prices, 12)
ema_26 = calculate_ema(mock_prices, 26)

# Strategy logic
if len(sma_10) >= 2 and len(sma_20) >= 2:
    # Current values
    current_price = mock_prices[-1]
    current_sma10 = sma_10[-1]
    current_sma20 = sma_20[-1]
    
    # Previous values
    prev_sma10 = sma_10[-2]
    prev_sma20 = sma_20[-2]
    
    # Golden Cross: 10-day MA crosses above 20-day MA
    golden_cross = (prev_sma10 <= prev_sma20) and (current_sma10 > current_sma20)
    
    # Additional momentum filters
    price_above_ma = current_price > current_sma10
    ma_slope_positive = current_sma10 > prev_sma10
    
    # Combined momentum signal
    buy_signal = golden_cross and price_above_ma and ma_slope_positive
    
    save_result('classification', buy_signal)
    save_result('current_price', current_price)
    save_result('sma_10', current_sma10)
    save_result('sma_20', current_sma20)
    save_result('golden_cross', golden_cross)
    save_result('price_above_ma', price_above_ma)
    save_result('ma_slope_positive', ma_slope_positive)
    save_result('signal_type', 'GOLDEN_CROSS' if golden_cross else 'NO_CROSS')
    save_result('reason', f'SMA10: {current_sma10:.2f}, SMA20: {current_sma20:.2f}')
else:
    save_result('classification', False)
    save_result('reason', 'Insufficient data for moving averages')
"""
    
    engine = PythonExecutionEngine()
    result = await engine.execute(strategy_code, {'symbol': 'DEMO'})
    
    print("Strategy Results:")
    print(f"  📊 Classification: {result.get('classification')}")
    print(f"  💰 Current Price: ${result.get('current_price', 0)}")
    print(f"  📈 SMA 10: ${result.get('sma_10', 0)}")
    print(f"  📉 SMA 20: ${result.get('sma_20', 0)}")
    print(f"  ✨ Golden Cross: {result.get('golden_cross', False)}")
    print(f"  🎯 Signal Type: {result.get('signal_type', 'N/A')}")
    print(f"  💭 Reason: {result.get('reason', 'N/A')}")
    print()


async def demo_custom_indicator():
    """Demo: Custom technical indicator (Stochastic Oscillator)"""
    print("⚡ Demo: Custom Stochastic Oscillator")
    print("-" * 50)
    
    strategy_code = """
# Custom Stochastic Oscillator Strategy
# Buy when %K crosses above %D and both are below 20 (oversold)

# Mock OHLC data
mock_highs = [52, 53, 54, 55, 56, 55, 54, 53, 52, 51, 50, 49, 50, 51, 52, 53, 54, 55, 56, 57]
mock_lows = [48, 49, 50, 51, 52, 51, 50, 49, 48, 47, 46, 45, 46, 47, 48, 49, 50, 51, 52, 53]
mock_closes = [50, 51, 52, 53, 54, 53, 52, 51, 50, 49, 48, 47, 48, 49, 50, 51, 52, 53, 54, 55]

def calculate_stochastic(highs, lows, closes, k_period=14, d_period=3):
    \"\"\"Calculate Stochastic Oscillator %K and %D\"\"\"
    if len(highs) < k_period or len(lows) < k_period or len(closes) < k_period:
        return {'k': [], 'd': []}
    
    k_values = []
    
    # Calculate %K
    for i in range(k_period - 1, len(closes)):
        period_high = max(highs[i - k_period + 1:i + 1])
        period_low = min(lows[i - k_period + 1:i + 1])
        current_close = closes[i]
        
        if period_high == period_low:
            k_value = 50  # Avoid division by zero
        else:
            k_value = ((current_close - period_low) / (period_high - period_low)) * 100
        
        k_values.append(k_value)
    
    # Calculate %D (moving average of %K)
    d_values = []
    for i in range(d_period - 1, len(k_values)):
        d_value = sum(k_values[i - d_period + 1:i + 1]) / d_period
        d_values.append(d_value)
    
    return {'k': k_values, 'd': d_values}

# Calculate Stochastic
stoch = calculate_stochastic(mock_highs, mock_lows, mock_closes, 14, 3)

# Strategy logic
if len(stoch['k']) >= 2 and len(stoch['d']) >= 2:
    current_k = stoch['k'][-1]
    current_d = stoch['d'][-1]
    prev_k = stoch['k'][-2]
    prev_d = stoch['d'][-2]
    
    # Stochastic signals
    k_crosses_d = (prev_k <= prev_d) and (current_k > current_d)
    both_oversold = current_k < 20 and current_d < 20
    k_rising = current_k > prev_k
    
    # Combined signal
    buy_signal = k_crosses_d and both_oversold and k_rising
    
    save_result('classification', buy_signal)
    save_result('current_k', current_k)
    save_result('current_d', current_d)
    save_result('k_crosses_d', k_crosses_d)
    save_result('both_oversold', both_oversold)
    save_result('k_rising', k_rising)
    save_result('signal_strength', 'STRONG' if buy_signal else 'WEAK')
    save_result('reason', f'%K: {current_k:.1f}, %D: {current_d:.1f}')
else:
    save_result('classification', False)
    save_result('reason', 'Insufficient data for Stochastic calculation')
"""
    
    engine = PythonExecutionEngine()
    result = await engine.execute(strategy_code, {'symbol': 'DEMO'})
    
    print("Strategy Results:")
    print(f"  📊 Classification: {result.get('classification')}")
    print(f"  📈 %K: {result.get('current_k', 0)}")
    print(f"  📉 %D: {result.get('current_d', 0)}")
    print(f"  ✨ K Crosses D: {result.get('k_crosses_d', False)}")
    print(f"  🔄 Both Oversold: {result.get('both_oversold', False)}")
    print(f"  🎯 Signal Strength: {result.get('signal_strength', 'N/A')}")
    print(f"  💭 Reason: {result.get('reason', 'N/A')}")
    print()


async def main():
    """Run all strategy demos"""
    print("🐍 Python Strategy System Demo")
    print("=" * 60)
    print("This demonstrates how to create custom trading strategies")
    print("that implement their own technical indicators from scratch.")
    print("=" * 60)
    print()
    
    await demo_mean_reversion_strategy()
    await demo_momentum_strategy()
    await demo_custom_indicator()
    
    print("✅ Demo complete!")
    print()
    print("Key Takeaways:")
    print("• Strategies implement their own technical indicators")
    print("• Use save_result() to return classification and metrics")
    print("• Raw data approach promotes better understanding")
    print("• Custom indicators can be as sophisticated as needed")
    print("• No dependency on pre-built indicator libraries")


if __name__ == "__main__":
    asyncio.run(main()) 