#!/usr/bin/env python3
"""
Test script for the unified strategy engine
Demonstrates all three execution modes: realtime, backtest, and screening
"""

import asyncio
import json
import logging
import sys
import os
from datetime import datetime, timedelta

# Add src to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "src"))

from src.unified_strategy_engine import (
    UnifiedStrategyEngine, 
    UnifiedStrategy, 
    ExecutionMode,
    UnifiedStrategyResult
)

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


async def test_realtime_momentum_strategy():
    """Test real-time momentum strategy execution"""
    print("\n🚨 Testing Real-time Momentum Strategy")
    print("=" * 50)
    
    # Real-time momentum strategy
    strategy_code = '''
def run_realtime_scan(symbols):
    """Real-time momentum scanning strategy"""
    alerts = []
    signals = {}
    
    log(f"Scanning {len(symbols)} symbols for momentum breakouts")
    
    for symbol in symbols[:5]:  # Limit for test
        try:
            # Get recent price data
            price_data = get_price_data(symbol, timeframe='1d', days=10)
            if not price_data.get('close') or len(price_data['close']) < 5:
                continue
            
            prices = price_data['close']
            volumes = price_data['volume']
            current_price = prices[-1]
            prev_price = prices[-2]
            
            # Calculate momentum
            daily_change = (current_price - prev_price) / prev_price
            five_day_change = (current_price - prices[-6]) / prices[-6] if len(prices) >= 6 else 0
            
            # Volume analysis
            avg_volume = sum(volumes[-5:]) / 5
            volume_ratio = volumes[-1] / avg_volume if avg_volume > 0 else 0
            
            # Signal criteria
            strong_move = abs(daily_change) > 0.02  # 2%+ move
            volume_spike = volume_ratio > 1.2  # 20%+ above average
            
            if strong_move and volume_spike:
                signal_data = {
                    'signal': True,
                    'symbol': symbol,
                    'daily_change': daily_change,
                    'volume_ratio': volume_ratio,
                    'current_price': current_price
                }
                
                signals[symbol] = signal_data
                
                alerts.append({
                    'symbol': symbol,
                    'type': 'momentum_breakout',
                    'message': f"{symbol} momentum: {daily_change:.2%}, vol: {volume_ratio:.1f}x",
                    'strength': abs(daily_change) + (volume_ratio * 0.1)
                })
                
                log(f"🚨 Alert: {symbol} momentum breakout")
        
        except Exception as e:
            log(f"Error processing {symbol}: {e}")
            continue
    
    return {'alerts': alerts, 'signals': signals}

def classify_symbol(symbol):
    """Fallback function"""
    try:
        price_data = get_price_data(symbol, timeframe='1d', days=5)
        if not price_data.get('close') or len(price_data['close']) < 2:
            return False
        
        current_price = price_data['close'][-1]
        prev_price = price_data['close'][-2]
        daily_change = (current_price - prev_price) / prev_price
        
        return abs(daily_change) > 0.02
        
    except Exception:
        return False
'''
    
    # Create strategy and engine
    engine = UnifiedStrategyEngine()
    strategy = UnifiedStrategy(strategy_code, strategy_id=1, name="Realtime Momentum")
    
    # Execute in real-time mode
    result = await engine.execute_strategy(
        strategy,
        ExecutionMode.REALTIME,
        symbols=["AAPL", "MSFT", "GOOGL", "AMZN", "TSLA", "NVDA", "META"]
    )
    
    # Display results
    print(f"✅ Execution successful: {result.success}")
    print(f"⏱️  Execution time: {result.execution_time_ms}ms")
    
    if result.success:
        print(f"🚨 Alerts generated: {len(result.alerts)}")
        print(f"📊 Signals found: {len(result.current_signals)}")
        
        for alert in result.alerts[:3]:  # Show first 3 alerts
            print(f"   - {alert['message']}")
        
        for symbol, signal in list(result.current_signals.items())[:3]:  # Show first 3 signals
            daily_change = signal.get('daily_change', 0)
            print(f"   - {symbol}: {daily_change:.2%} move")
    else:
        print(f"❌ Error: {result.error_message}")
    
    return result


async def test_backtest_value_strategy():
    """Test backtest value strategy execution"""
    print("\n📈 Testing Backtest Value Strategy")
    print("=" * 50)
    
    # Value investing backtest strategy
    strategy_code = '''
def run_batch_backtest(start_date, end_date, symbols):
    """Value investing backtest strategy"""
    log(f"Running value backtest from {start_date} to {end_date} on {len(symbols)} symbols")
    
    instances = []
    performance_metrics = {}
    
    try:
        # Get universe with filters
        universe_data = scan_universe(
            filters={'min_market_cap': 1000000000},
            sort_by='market_cap',
            limit=min(50, len(symbols))  # Limit for test
        )
        
        valid_symbols = [s['ticker'] for s in universe_data.get('data', [])]
        log(f"Analyzing {len(valid_symbols)} symbols for value opportunities")
        
        for symbol in valid_symbols[:10]:  # Test with 10 symbols
            try:
                # Get fundamental data
                fundamentals = get_fundamental_data(symbol)
                if not fundamentals:
                    continue
                
                # Get price data
                price_data = get_price_data(symbol, timeframe='1d', days=30)
                if not price_data.get('close'):
                    continue
                
                # Extract metrics
                market_cap = fundamentals.get('market_cap', 0)
                eps = fundamentals.get('eps', 0)
                book_value = fundamentals.get('book_value', 0)
                current_price = price_data['close'][-1]
                
                # Value criteria
                if eps > 0 and market_cap > 1000000000:
                    pe_ratio = current_price / eps
                    
                    if pe_ratio < 20:  # Reasonable P/E
                        # Simulate return (in real backtest, this would be actual future performance)
                        expected_return = max(-0.1, min(0.3, (25 - pe_ratio) * 0.02))
                        
                        instance = {
                            'ticker': symbol,
                            'timestamp': int(datetime.utcnow().timestamp() * 1000),
                            'classification': True,
                            'pe_ratio': pe_ratio,
                            'market_cap': market_cap,
                            'current_price': current_price,
                            'expected_return': expected_return,
                            'strategy_results': {
                                'value_score': 25 - pe_ratio,
                                'fundamentals': fundamentals
                            }
                        }
                        
                        instances.append(instance)
                        log(f"Value pick: {symbol} (P/E: {pe_ratio:.1f})")
                
            except Exception as e:
                log(f"Error analyzing {symbol}: {e}")
                continue
        
        # Calculate metrics
        if instances:
            returns = [i['expected_return'] for i in instances]
            pe_ratios = [i['pe_ratio'] for i in instances]
            
            performance_metrics = {
                'total_picks': len(instances),
                'average_return': sum(returns) / len(returns),
                'average_pe': sum(pe_ratios) / len(pe_ratios),
                'positive_return_rate': len([r for r in returns if r > 0]) / len(returns)
            }
            
            log(f"✅ Found {len(instances)} value opportunities")
        
    except Exception as e:
        log(f"Backtest error: {e}")
    
    return {
        'instances': instances,
        'performance_metrics': performance_metrics
    }

def classify_symbol(symbol):
    """Simple value classification"""
    try:
        fundamentals = get_fundamental_data(symbol)
        if not fundamentals:
            return False
        
        eps = fundamentals.get('eps', 0)
        if eps <= 0:
            return False
        
        price_data = get_price_data(symbol, timeframe='1d', days=1)
        if not price_data.get('close'):
            return False
        
        pe_ratio = price_data['close'][-1] / eps
        return pe_ratio < 20
        
    except Exception:
        return False
'''
    
    # Create strategy and execute
    engine = UnifiedStrategyEngine()
    strategy = UnifiedStrategy(strategy_code, strategy_id=2, name="Value Backtest")
    
    # Execute backtest
    start_date = (datetime.now() - timedelta(days=365)).isoformat()
    end_date = datetime.now().isoformat()
    
    result = await engine.execute_strategy(
        strategy,
        ExecutionMode.BACKTEST,
        start_date=start_date,
        end_date=end_date,
        symbols=["AAPL", "MSFT", "GOOGL", "AMZN", "TSLA", "BRK.A", "JNJ", "V", "WMT", "PG"]
    )
    
    # Display results
    print(f"✅ Execution successful: {result.success}")
    print(f"⏱️  Execution time: {result.execution_time_ms}ms")
    
    if result.success:
        print(f"📊 Instances found: {len(result.instances)}")
        print(f"📈 Performance metrics: {len(result.performance_metrics)} metrics")
        
        if result.performance_metrics:
            metrics = result.performance_metrics
            print(f"   - Total picks: {metrics.get('total_picks', 0)}")
            print(f"   - Average return: {metrics.get('average_return', 0):.2%}")
            print(f"   - Average P/E: {metrics.get('average_pe', 0):.1f}")
        
        # Show sample instances
        for instance in result.instances[:3]:
            symbol = instance['ticker']
            pe = instance.get('pe_ratio', 0)
            ret = instance.get('expected_return', 0)
            print(f"   - {symbol}: P/E {pe:.1f}, Expected return {ret:.1%}")
    else:
        print(f"❌ Error: {result.error_message}")
    
    return result


async def test_screening_strategy():
    """Test screening strategy execution"""
    print("\n🔍 Testing Screening Strategy")
    print("=" * 50)
    
    # Momentum screening strategy
    strategy_code = '''
def run_screening(universe, limit):
    """Momentum screening strategy"""
    log(f"Screening {len(universe)} symbols for momentum")
    
    scored_symbols = []
    scores = {}
    
    try:
        for symbol in universe[:15]:  # Limit for test
            try:
                # Get price and volume data
                price_data = get_price_data(symbol, timeframe='1d', days=30)
                if not price_data.get('close') or len(price_data['close']) < 20:
                    continue
                
                prices = price_data['close']
                volumes = price_data['volume']
                
                # Calculate momentum metrics
                returns_5d = (prices[-1] / prices[-6]) - 1 if len(prices) >= 6 else 0
                returns_20d = (prices[-1] / prices[-21]) - 1 if len(prices) >= 21 else 0
                
                # Volume analysis
                avg_volume = sum(volumes[-10:]) / 10 if len(volumes) >= 10 else 0
                volume_ratio = volumes[-1] / avg_volume if avg_volume > 0 else 0
                
                # Combined momentum score
                momentum_score = (returns_5d * 2) + returns_20d + (volume_ratio * 0.1)
                
                if momentum_score > 0.05:  # 5% threshold
                    result_data = {
                        'symbol': symbol,
                        'score': momentum_score,
                        'returns_5d': returns_5d,
                        'returns_20d': returns_20d,
                        'volume_ratio': volume_ratio,
                        'current_price': prices[-1]
                    }
                    
                    scored_symbols.append(result_data)
                    scores[symbol] = momentum_score
                    
                    log(f"Momentum candidate: {symbol} (score: {momentum_score:.3f})")
                    
            except Exception as e:
                log(f"Error screening {symbol}: {e}")
                continue
        
        # Sort by score
        scored_symbols.sort(key=lambda x: x['score'], reverse=True)
        ranked_results = scored_symbols[:limit]
        
        log(f"✅ Found {len(ranked_results)} momentum candidates")
        
    except Exception as e:
        log(f"Screening error: {e}")
        ranked_results = []
        scores = {}
    
    return {
        'ranked_results': ranked_results,
        'scores': scores
    }

def score_symbol(symbol):
    """Score individual symbol"""
    try:
        price_data = get_price_data(symbol, timeframe='1d', days=20)
        if not price_data.get('close') or len(price_data['close']) < 10:
            return 0
        
        prices = price_data['close']
        returns_5d = (prices[-1] / prices[-6]) - 1 if len(prices) >= 6 else 0
        return returns_5d * 2
        
    except Exception:
        return 0
'''
    
    # Create strategy and execute
    engine = UnifiedStrategyEngine()
    strategy = UnifiedStrategy(strategy_code, strategy_id=3, name="Momentum Screening")
    
    result = await engine.execute_strategy(
        strategy,
        ExecutionMode.SCREENING,
        universe=["AAPL", "MSFT", "GOOGL", "AMZN", "TSLA", "NVDA", "META", "NFLX", "ADBE", "CRM"],
        limit=5
    )
    
    # Display results
    print(f"✅ Execution successful: {result.success}")
    print(f"⏱️  Execution time: {result.execution_time_ms}ms")
    
    if result.success:
        print(f"🔍 Ranked results: {len(result.ranked_results)}")
        print(f"📊 Scores calculated: {len(result.scores)}")
        print(f"🌐 Universe size: {result.universe_size}")
        
        # Show top ranked results
        for i, result_item in enumerate(result.ranked_results[:3]):
            symbol = result_item['symbol']
            score = result_item['score']
            returns_5d = result_item.get('returns_5d', 0)
            print(f"   {i+1}. {symbol}: Score {score:.3f}, 5d return {returns_5d:.2%}")
    else:
        print(f"❌ Error: {result.error_message}")
    
    return result


async def test_comprehensive_strategy():
    """Test a strategy that supports all three modes"""
    print("\n🎯 Testing Comprehensive Multi-Mode Strategy")
    print("=" * 50)
    
    # Strategy that works in all modes
    strategy_code = '''
# Multi-mode strategy that adapts based on execution context

def run_realtime_scan(symbols):
    """Real-time mode implementation"""
    alerts = []
    signals = {}
    
    for symbol in symbols[:3]:  # Limit for real-time
        if classify_symbol(symbol):
            signals[symbol] = {'signal': True, 'timestamp': datetime.utcnow().isoformat()}
            alerts.append({'symbol': symbol, 'type': 'signal', 'message': f'{symbol} triggered'})
    
    return {'alerts': alerts, 'signals': signals}

def run_batch_backtest(start_date, end_date, symbols):
    """Backtest mode implementation"""
    instances = []
    
    for symbol in symbols[:5]:  # Limit for test
        if classify_symbol(symbol):
            instances.append({
                'ticker': symbol,
                'timestamp': int(datetime.utcnow().timestamp() * 1000),
                'classification': True
            })
    
    return {'instances': instances, 'performance_metrics': {'total': len(instances)}}

def run_screening(universe, limit):
    """Screening mode implementation"""
    ranked_results = []
    scores = {}
    
    for symbol in universe:
        score = score_symbol(symbol)
        if score > 0:
            ranked_results.append({'symbol': symbol, 'score': score})
            scores[symbol] = score
    
    ranked_results.sort(key=lambda x: x['score'], reverse=True)
    return {'ranked_results': ranked_results[:limit], 'scores': scores}

def classify_symbol(symbol):
    """Core classification logic"""
    try:
        price_data = get_price_data(symbol, timeframe='1d', days=5)
        if not price_data.get('close') or len(price_data['close']) < 2:
            return False
        
        prices = price_data['close']
        daily_change = (prices[-1] / prices[-2]) - 1
        return abs(daily_change) > 0.01  # 1% threshold
        
    except Exception:
        return False

def score_symbol(symbol):
    """Scoring for screening"""
    try:
        price_data = get_price_data(symbol, timeframe='1d', days=10)
        if not price_data.get('close') or len(price_data['close']) < 5:
            return 0
        
        prices = price_data['close']
        returns = [(prices[i] / prices[i-1]) - 1 for i in range(1, len(prices))]
        return sum(returns) if returns else 0
        
    except Exception:
        return 0
'''
    
    engine = UnifiedStrategyEngine()
    strategy = UnifiedStrategy(strategy_code, strategy_id=4, name="Multi-Mode Strategy")
    
    # Test all three modes
    test_symbols = ["AAPL", "MSFT", "GOOGL", "AMZN", "TSLA"]
    
    print("Testing Realtime mode...")
    realtime_result = await engine.execute_strategy(
        strategy, ExecutionMode.REALTIME, symbols=test_symbols
    )
    print(f"   Realtime: {realtime_result.success}, {len(realtime_result.alerts)} alerts")
    
    print("Testing Backtest mode...")
    backtest_result = await engine.execute_strategy(
        strategy, ExecutionMode.BACKTEST,
        start_date=(datetime.now() - timedelta(days=30)).isoformat(),
        end_date=datetime.now().isoformat(),
        symbols=test_symbols
    )
    print(f"   Backtest: {backtest_result.success}, {len(backtest_result.instances)} instances")
    
    print("Testing Screening mode...")
    screening_result = await engine.execute_strategy(
        strategy, ExecutionMode.SCREENING, universe=test_symbols, limit=3
    )
    print(f"   Screening: {screening_result.success}, {len(screening_result.ranked_results)} results")
    
    return realtime_result, backtest_result, screening_result


async def main():
    """Run all tests"""
    print("🧪 Unified Strategy Engine Test Suite")
    print("=" * 60)
    
    try:
        # Test individual modes
        await test_realtime_momentum_strategy()
        await test_backtest_value_strategy()
        await test_screening_strategy()
        
        # Test comprehensive strategy
        await test_comprehensive_strategy()
        
        print("\n✅ All tests completed successfully!")
        
    except Exception as e:
        print(f"\n❌ Test failed: {e}")
        import traceback
        traceback.print_exc()


if __name__ == "__main__":
    asyncio.run(main()) 