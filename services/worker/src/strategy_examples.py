"""
Unified Strategy Examples
Demonstrates how to write strategies that work across realtime, backtest, and screening modes
"""

# Example 1: Real-time Momentum Strategy
REALTIME_MOMENTUM_STRATEGY = '''
def run_realtime_scan(symbols):
    """Real-time momentum scanning strategy - optimized for live trading alerts"""
    alerts = []
    signals = {}
    
    log(f"Scanning {len(symbols)} symbols for momentum breakouts")
    
    for symbol in symbols:
        try:
            # Get recent price data (last 10 days for context)
            price_data = get_price_data(symbol, timeframe='1d', days=10)
            if not price_data.get('close') or len(price_data['close']) < 5:
                continue
            
            # Get intraday data for more precise signals
            intraday_data = get_price_data(symbol, timeframe='1h', days=2)
            
            # Calculate momentum indicators
            prices = price_data['close']
            volumes = price_data['volume']
            current_price = prices[-1]
            prev_price = prices[-2]
            
            # Daily momentum
            daily_change = (current_price - prev_price) / prev_price
            
            # 5-day momentum
            five_day_change = (current_price - prices[-6]) / prices[-6] if len(prices) >= 6 else 0
            
            # Volume analysis
            avg_volume = sum(volumes[-5:]) / 5
            current_volume = volumes[-1]
            volume_ratio = current_volume / avg_volume if avg_volume > 0 else 0
            
            # Get sector info for context
            security_info = get_security_info(symbol)
            sector = security_info.get('sector', 'Unknown')
            
            # Signal criteria
            strong_daily_move = abs(daily_change) > 0.03  # 3%+
            volume_spike = volume_ratio > 1.5  # 50%+ above average
            upward_momentum = five_day_change > 0.02  # 2%+ over 5 days
            
            # Combined signal strength
            signal_strength = (abs(daily_change) * 2) + (five_day_change * 1) + (volume_ratio * 0.1)
            
            if strong_daily_move and volume_spike and signal_strength > 0.1:
                signal_data = {
                    'signal': True,
                    'timestamp': datetime.utcnow().isoformat(),
                    'symbol': symbol,
                    'signal_strength': round(signal_strength, 3),
                    'daily_change': round(daily_change, 4),
                    'five_day_change': round(five_day_change, 4),
                    'volume_ratio': round(volume_ratio, 2),
                    'current_price': current_price,
                    'sector': sector,
                    'direction': 'bullish' if daily_change > 0 else 'bearish'
                }
                
                signals[symbol] = signal_data
                
                # Create alert
                alert_message = f"{symbol} ({sector}) momentum breakout: {daily_change:.2%} daily, vol: {volume_ratio:.1f}x avg"
                alerts.append({
                    'symbol': symbol,
                    'type': 'momentum_breakout',
                    'message': alert_message,
                    'strength': signal_strength,
                    'priority': 'high' if signal_strength > 0.15 else 'medium',
                    'timestamp': datetime.utcnow().isoformat(),
                    'data': signal_data
                })
                
                log(f"🚨 Alert: {alert_message}")
        
        except Exception as e:
            log(f"Error processing {symbol}: {e}")
            continue
    
    log(f"Generated {len(alerts)} alerts from {len(symbols)} symbols")
    return {'alerts': alerts, 'signals': signals}

# Also provide fallback classify_symbol for compatibility
def classify_symbol(symbol):
    """Fallback function for single symbol classification"""
    try:
        price_data = get_price_data(symbol, timeframe='1d', days=5)
        if not price_data.get('close') or len(price_data['close']) < 2:
            return False
        
        current_price = price_data['close'][-1]
        prev_price = price_data['close'][-2]
        daily_change = (current_price - prev_price) / prev_price
        
        return abs(daily_change) > 0.03  # 3%+ move
        
    except Exception:
        return False
'''

# Example 2: Batch Backtest Value Strategy
BATCH_BACKTEST_VALUE_STRATEGY = '''
def run_batch_backtest(start_date, end_date, symbols):
    """Optimized batch value investing backtest"""
    log(f"Running value backtest from {start_date} to {end_date} on {len(symbols)} symbols")
    
    instances = []
    performance_metrics = {}
    
    try:
        # Step 1: Get universe with basic filters
        universe_data = scan_universe(
            filters={'min_market_cap': 1000000000},  # $1B+ companies
            sort_by='market_cap',
            limit=len(symbols)
        )
        
        valid_symbols = [s['ticker'] for s in universe_data.get('data', [])][:200]  # Limit for demo
        log(f"Filtered to {len(valid_symbols)} valid symbols")
        
        # Step 2: Bulk load all fundamental data
        fundamentals_batch = {}
        for symbol in valid_symbols:
            fundamentals_batch[symbol] = get_fundamental_data(symbol)
        
        log("Loaded fundamental data for all symbols")
        
        # Step 3: Apply value screening criteria
        value_candidates = []
        
        for symbol in valid_symbols:
            try:
                fundamentals = fundamentals_batch.get(symbol, {})
                if not fundamentals:
                    continue
                
                # Get price data for valuation
                price_data = get_price_data(symbol, timeframe='1d', days=252)  # 1 year
                if not price_data.get('close') or len(price_data['close']) < 50:
                    continue
                
                # Extract valuation metrics
                market_cap = fundamentals.get('market_cap', 0)
                book_value = fundamentals.get('book_value', 0)
                eps = fundamentals.get('eps', 0)
                revenue = fundamentals.get('revenue', 0)
                debt = fundamentals.get('debt', 0)
                cash = fundamentals.get('cash', 0)
                shares_outstanding = fundamentals.get('shares_outstanding', 1)
                
                current_price = price_data['close'][-1]
                
                # Calculate ratios
                if eps > 0:
                    pe_ratio = current_price / eps
                else:
                    continue  # Skip negative earnings
                
                if book_value > 0 and shares_outstanding > 0:
                    book_per_share = book_value / shares_outstanding
                    pb_ratio = current_price / book_per_share
                else:
                    pb_ratio = float('inf')
                
                # Net cash position
                net_cash = cash - debt
                
                # Value criteria (Benjamin Graham style)
                low_pe = pe_ratio < 15  # P/E under 15
                low_pb = pb_ratio < 1.5  # P/B under 1.5
                profitable = eps > 0
                adequate_size = market_cap > 1000000000  # $1B+
                strong_balance = net_cash > 0  # Net cash positive
                
                # Combined value score
                value_score = 0
                if low_pe: value_score += 2
                if low_pb: value_score += 2
                if profitable: value_score += 1
                if adequate_size: value_score += 1
                if strong_balance: value_score += 2
                
                # Require minimum score for inclusion
                if value_score >= 5:  # At least 5/8 criteria
                    
                    # Simulate future returns based on value metrics
                    # In reality, this would be calculated from actual historical performance
                    expected_return = min(0.20, (20 - pe_ratio) * 0.01 + (2 - pb_ratio) * 0.05)
                    expected_return = max(-0.10, expected_return)  # Cap downside at -10%
                    
                    # Add some randomness to simulate market variability
                    import time
                    random_factor = (hash(symbol + start_date) % 1000) / 5000  # -0.1 to +0.1
                    simulated_return = expected_return + random_factor
                    
                    instance = {
                        'ticker': symbol,
                        'timestamp': int(datetime.fromisoformat(start_date.replace('Z', '+00:00')).timestamp() * 1000),
                        'classification': True,
                        'entry_price': current_price,
                        'market_cap': market_cap,
                        'pe_ratio': round(pe_ratio, 2),
                        'pb_ratio': round(pb_ratio, 2),
                        'value_score': value_score,
                        'net_cash': net_cash,
                        'eps': eps,
                        'expected_return': round(expected_return, 3),
                        'simulated_return': round(simulated_return, 3),
                        'strategy_results': {
                            'value_criteria': {
                                'low_pe': low_pe,
                                'low_pb': low_pb,
                                'profitable': profitable,
                                'adequate_size': adequate_size,
                                'strong_balance': strong_balance
                            },
                            'fundamental_data': fundamentals
                        }
                    }
                    
                    instances.append(instance)
                    value_candidates.append((symbol, value_score, expected_return))
                    
            except Exception as e:
                log(f"Error processing {symbol}: {e}")
                continue
        
        # Step 4: Calculate performance metrics
        if instances:
            returns = [i['simulated_return'] for i in instances]
            pe_ratios = [i['pe_ratio'] for i in instances]
            pb_ratios = [i['pb_ratio'] for i in instances]
            
            performance_metrics = {
                'total_picks': len(instances),
                'average_return': round(sum(returns) / len(returns), 3),
                'median_return': round(sorted(returns)[len(returns)//2], 3),
                'best_return': round(max(returns), 3),
                'worst_return': round(min(returns), 3),
                'positive_return_rate': len([r for r in returns if r > 0]) / len(returns),
                'average_pe': round(sum(pe_ratios) / len(pe_ratios), 2),
                'average_pb': round(sum(pb_ratios) / len(pb_ratios), 2),
                'avg_value_score': round(sum(i['value_score'] for i in instances) / len(instances), 1),
                'strategy_type': 'value_investing',
                'universe_size': len(valid_symbols),
                'selection_rate': len(instances) / len(valid_symbols) if valid_symbols else 0
            }
            
            log(f"✅ Backtest complete: {len(instances)} value picks, avg return: {performance_metrics['average_return']:.1%}")
        else:
            performance_metrics = {'error': 'No valid value opportunities found'}
            
    except Exception as e:
        log(f"Backtest error: {e}")
        performance_metrics = {'error': str(e)}
    
    return {
        'instances': instances,
        'performance_metrics': performance_metrics
    }

# Fallback for simple classification
def classify_symbol(symbol):
    """Simple value classification for single symbol"""
    try:
        fundamentals = get_fundamental_data(symbol)
        if not fundamentals:
            return False
        
        eps = fundamentals.get('eps', 0)
        market_cap = fundamentals.get('market_cap', 0)
        
        if eps <= 0 or market_cap < 1000000000:  # Must be profitable and $1B+
            return False
        
        # Get current price
        price_data = get_price_data(symbol, timeframe='1d', days=1)
        if not price_data.get('close'):
            return False
        
        current_price = price_data['close'][-1]
        pe_ratio = current_price / eps
        
        return pe_ratio < 15  # Simple P/E criterion
        
    except Exception:
        return False
'''

# Example 3: Screening Strategy with Ranking
SCREENING_MOMENTUM_STRATEGY = '''
def run_screening(universe, limit):
    """Advanced momentum screening with multi-factor scoring"""
    log(f"Screening {len(universe)} symbols for momentum opportunities")
    
    scored_symbols = []
    scores = {}
    
    try:
        # Batch process symbols for efficiency
        symbol_data = {}
        
        # Load data for all symbols in batches
        batch_size = 20
        for i in range(0, len(universe), batch_size):
            batch_symbols = universe[i:i + batch_size]
            
            for symbol in batch_symbols:
                try:
                    # Get price data and fundamentals
                    price_data = get_price_data(symbol, timeframe='1d', days=60)
                    security_info = get_security_info(symbol)
                    
                    if price_data.get('close') and len(price_data['close']) >= 30:
                        symbol_data[symbol] = {
                            'price_data': price_data,
                            'security_info': security_info
                        }
                except Exception as e:
                    continue
        
        log(f"Loaded data for {len(symbol_data)} symbols")
        
        # Calculate momentum factors for each symbol
        for symbol, data in symbol_data.items():
            try:
                price_data = data['price_data']
                security_info = data['security_info']
                
                prices = price_data['close']
                volumes = price_data['volume']
                timestamps = price_data['timestamps']
                
                if len(prices) < 30:
                    continue
                
                current_price = prices[-1]
                sector = security_info.get('sector', 'Unknown')
                market_cap = security_info.get('market_cap', 0)
                
                # Momentum calculations
                returns_1d = (prices[-1] / prices[-2]) - 1 if len(prices) >= 2 else 0
                returns_5d = (prices[-1] / prices[-6]) - 1 if len(prices) >= 6 else 0
                returns_20d = (prices[-1] / prices[-21]) - 1 if len(prices) >= 21 else 0
                returns_60d = (prices[-1] / prices[-60]) - 1 if len(prices) >= 60 else 0
                
                # Volume analysis
                avg_volume_20d = sum(volumes[-20:]) / 20 if len(volumes) >= 20 else 0
                recent_volume = volumes[-1]
                volume_ratio = recent_volume / avg_volume_20d if avg_volume_20d > 0 else 0
                
                # Volatility (simplified)
                recent_returns = []
                for i in range(1, min(21, len(prices))):
                    daily_return = (prices[-i] / prices[-i-1]) - 1
                    recent_returns.append(daily_return)
                
                volatility = 0
                if recent_returns:
                    mean_return = sum(recent_returns) / len(recent_returns)
                    variance = sum((r - mean_return) ** 2 for r in recent_returns) / len(recent_returns)
                    volatility = variance ** 0.5
                
                # Technical indicators
                # Simple Moving Average (SMA) 20
                sma_20 = sum(prices[-20:]) / 20 if len(prices) >= 20 else current_price
                price_vs_sma = (current_price / sma_20) - 1
                
                # Price trend (linear regression slope approximation)
                if len(prices) >= 10:
                    recent_prices = prices[-10:]
                    x_vals = list(range(len(recent_prices)))
                    n = len(recent_prices)
                    
                    sum_x = sum(x_vals)
                    sum_y = sum(recent_prices)
                    sum_xy = sum(x * y for x, y in zip(x_vals, recent_prices))
                    sum_x2 = sum(x * x for x in x_vals)
                    
                    if n * sum_x2 - sum_x * sum_x != 0:
                        slope = (n * sum_xy - sum_x * sum_y) / (n * sum_x2 - sum_x * sum_x)
                        trend_strength = slope / current_price  # Normalize by price
                    else:
                        trend_strength = 0
                else:
                    trend_strength = 0
                
                # Multi-factor momentum score
                momentum_score = 0
                
                # Recent performance (40% weight)
                momentum_score += returns_1d * 10  # 1-day return
                momentum_score += returns_5d * 5   # 5-day return
                momentum_score += returns_20d * 2  # 20-day return
                
                # Volume factor (20% weight)
                volume_score = min(2.0, volume_ratio * 0.5)  # Cap at 2.0
                momentum_score += volume_score
                
                # Technical factors (30% weight)
                momentum_score += price_vs_sma * 5  # Price vs SMA
                momentum_score += trend_strength * 10  # Trend strength
                
                # Risk adjustment (10% weight)
                if volatility > 0:
                    risk_adjusted_return = returns_20d / volatility
                    momentum_score += min(1.0, risk_adjusted_return)
                
                # Quality filters
                min_market_cap = 500000000  # $500M minimum
                min_volume = 100000  # 100K average daily volume
                max_volatility = 0.10  # 10% max daily volatility
                
                quality_filter = (
                    market_cap >= min_market_cap and
                    avg_volume_20d >= min_volume and
                    volatility <= max_volatility
                )
                
                # Only include if passes quality filter and has positive momentum
                if quality_filter and momentum_score > 0.1:
                    result_data = {
                        'symbol': symbol,
                        'score': round(momentum_score, 4),
                        'current_price': current_price,
                        'sector': sector,
                        'market_cap': market_cap,
                        'returns_1d': round(returns_1d, 4),
                        'returns_5d': round(returns_5d, 4),
                        'returns_20d': round(returns_20d, 4),
                        'volume_ratio': round(volume_ratio, 2),
                        'volatility': round(volatility, 4),
                        'price_vs_sma': round(price_vs_sma, 4),
                        'trend_strength': round(trend_strength, 6),
                        'momentum_factors': {
                            'recent_performance': round(returns_1d * 10 + returns_5d * 5 + returns_20d * 2, 4),
                            'volume_factor': round(volume_score, 4),
                            'technical_score': round(price_vs_sma * 5 + trend_strength * 10, 4),
                            'risk_adjusted': round(risk_adjusted_return if volatility > 0 else 0, 4)
                        }
                    }
                    
                    scored_symbols.append(result_data)
                    scores[symbol] = momentum_score
                    
            except Exception as e:
                log(f"Error scoring {symbol}: {e}")
                continue
        
        # Sort by momentum score
        scored_symbols.sort(key=lambda x: x['score'], reverse=True)
        ranked_results = scored_symbols[:limit]
        
        log(f"✅ Screening complete: {len(ranked_results)} momentum candidates from {len(universe)} universe")
        
        # Log top picks
        if ranked_results:
            log("Top 3 momentum picks:")
            for i, result in enumerate(ranked_results[:3]):
                log(f"  {i+1}. {result['symbol']} ({result['sector']}) - Score: {result['score']:.3f}, 20d: {result['returns_20d']:.2%}")
        
    except Exception as e:
        log(f"Screening error: {e}")
        ranked_results = []
        scores = {}
    
    return {
        'ranked_results': ranked_results,
        'scores': scores
    }

# Also provide fallback functions
def score_symbol(symbol):
    """Score a single symbol for momentum"""
    try:
        price_data = get_price_data(symbol, timeframe='1d', days=20)
        if not price_data.get('close') or len(price_data['close']) < 10:
            return 0
        
        prices = price_data['close']
        returns_5d = (prices[-1] / prices[-6]) - 1 if len(prices) >= 6 else 0
        returns_20d = (prices[-1] / prices[-20]) - 1 if len(prices) >= 20 else 0
        
        return returns_5d * 2 + returns_20d
        
    except Exception:
        return 0

def classify_symbol(symbol):
    """Simple momentum classification"""
    return score_symbol(symbol) > 0.05  # 5% threshold
''' 