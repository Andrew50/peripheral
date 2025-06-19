"""
Unified Strategy Engine
Supports realtime alerts, backtests, and screening with a single strategy format
Optimized for high-performance execution across all modes
"""

import asyncio
import logging
import json
from datetime import datetime, timedelta
from typing import Any, Dict, List, Optional, Union, Tuple
from enum import Enum
import numpy as np
import pandas as pd

try:
    from .data_provider import DataProvider
    from .execution_engine import PythonExecutionEngine, CodeValidator, SecurityError
except ImportError:
    from data_provider import DataProvider
    from execution_engine import PythonExecutionEngine, CodeValidator, SecurityError

logger = logging.getLogger(__name__)


class ExecutionMode(Enum):
    """Strategy execution modes"""
    REALTIME = "realtime"
    BACKTEST = "backtest"  
    SCREENING = "screening"


class IterationStrategy(Enum):
    """Data iteration strategies for optimization"""
    TIMESTEP_FILTER = "timestep_filter"  # Apply strategy to each timestep
    BATCH_OPTIMIZE = "batch_optimize"    # Custom optimization in strategy
    HYBRID = "hybrid"                    # Combination approach


class UnifiedStrategyResult:
    """Result container for any strategy execution mode"""
    
    def __init__(self, mode: ExecutionMode):
        self.mode = mode
        self.execution_time_ms: int = 0
        self.timestamp = datetime.utcnow()
        
        # Common results
        self.success: bool = False
        self.error_message: Optional[str] = None
        self.strategy_results: Dict[str, Any] = {}
        
        # Mode-specific results
        if mode == ExecutionMode.REALTIME:
            self.alerts: List[Dict[str, Any]] = []
            self.current_signals: Dict[str, Any] = {}
            
        elif mode == ExecutionMode.BACKTEST:
            self.instances: List[Dict[str, Any]] = []
            self.summary: Dict[str, Any] = {}
            self.performance_metrics: Dict[str, Any] = {}
            
        elif mode == ExecutionMode.SCREENING:
            self.ranked_results: List[Dict[str, Any]] = []
            self.scores: Dict[str, float] = {}
            self.universe_size: int = 0


class UnifiedStrategy:
    """Base strategy class that works across all execution modes"""
    
    def __init__(self, strategy_code: str, strategy_id: int, name: str = ""):
        self.strategy_code = strategy_code
        self.strategy_id = strategy_id
        self.name = name
        self.iteration_strategy = IterationStrategy.BATCH_OPTIMIZE  # Default to most flexible
        
        # Strategy metadata extracted from code analysis
        self.required_data_sources: List[str] = []
        self.optimization_hints: Dict[str, Any] = {}
        self.universe_filters: Dict[str, Any] = {}
        
        self._analyze_strategy_code()
    
    def _analyze_strategy_code(self):
        """Analyze strategy code to extract optimization hints and requirements"""
        code_lower = self.strategy_code.lower()
        
        # Detect data sources used
        data_functions = [
            'get_price_data', 'get_historical_data', 'get_fundamental_data',
            'get_security_info', 'scan_universe', 'get_multiple_symbols_data'
        ]
        
        for func in data_functions:
            if func in code_lower:
                self.required_data_sources.append(func)
        
        # Detect iteration hints
        if 'scan_universe' in code_lower and 'for ' in code_lower:
            self.iteration_strategy = IterationStrategy.BATCH_OPTIMIZE
        elif 'def process_timestep' in code_lower:
            self.iteration_strategy = IterationStrategy.TIMESTEP_FILTER
        
        # Extract universe hints
        if 'sector' in code_lower:
            self.universe_filters['requires_sector'] = True
        if 'market_cap' in code_lower:
            self.universe_filters['requires_fundamentals'] = True


class UnifiedStrategyEngine:
    """High-performance strategy engine supporting all execution modes"""
    
    def __init__(self):
        self.data_provider = DataProvider()
        self.execution_engine = PythonExecutionEngine()
        self.validator = CodeValidator()
        
        # Performance optimization caches
        self._universe_cache: Dict[str, List[str]] = {}
        self._data_cache: Dict[str, Dict] = {}
        self._cache_ttl = 300  # 5 minutes
        
    async def execute_strategy(
        self,
        strategy: UnifiedStrategy,
        mode: ExecutionMode,
        **kwargs
    ) -> UnifiedStrategyResult:
        """Execute strategy in specified mode with optimizations"""
        
        result = UnifiedStrategyResult(mode)
        start_time = datetime.utcnow()
        
        try:
            # Validate strategy code
            if not self.validator.validate(strategy.strategy_code):
                raise SecurityError("Strategy code validation failed")
            
            # Route to appropriate execution method
            if mode == ExecutionMode.REALTIME:
                await self._execute_realtime(strategy, result, **kwargs)
            elif mode == ExecutionMode.BACKTEST:
                await self._execute_backtest(strategy, result, **kwargs)
            elif mode == ExecutionMode.SCREENING:
                await self._execute_screening(strategy, result, **kwargs)
            
            result.success = True
            
        except Exception as e:
            logger.error(f"Strategy execution error: {e}")
            result.error_message = str(e)
            result.success = False
        
        # Calculate execution time
        end_time = datetime.utcnow()
        result.execution_time_ms = int((end_time - start_time).total_seconds() * 1000)
        
        return result
    
    async def _execute_realtime(
        self,
        strategy: UnifiedStrategy,
        result: UnifiedStrategyResult,
        symbols: Optional[List[str]] = None,
        **kwargs
    ):
        """Execute strategy for real-time alerts"""
        
        # Get current universe if not specified
        if not symbols:
            symbols = await self._get_optimized_universe(strategy, limit=100)
        
        # Prepare real-time execution context
        context = {
            'execution_mode': 'realtime',
            'current_timestamp': datetime.utcnow().isoformat(),
            'symbols': symbols,
            'alerts': [],
            'signals': {}
        }
        
        # Wrap strategy code for real-time execution
        wrapped_code = self._wrap_strategy_for_realtime(strategy.strategy_code)
        
        # Execute strategy
        execution_result = await self.execution_engine.execute(wrapped_code, context)
        
        # Extract real-time results
        result.strategy_results = execution_result
        result.alerts = execution_result.get('alerts', [])
        result.current_signals = execution_result.get('signals', {})
    
    async def _execute_backtest(
        self,
        strategy: UnifiedStrategy,
        result: UnifiedStrategyResult,
        start_date: str,
        end_date: str,
        symbols: Optional[List[str]] = None,
        **kwargs
    ):
        """Execute strategy for backtesting with optimized iteration"""
        
        # Get universe for backtesting
        if not symbols:
            symbols = await self._get_optimized_universe(strategy, limit=500)
        
        # Parse dates
        start_dt = datetime.fromisoformat(start_date.replace('Z', '+00:00'))
        end_dt = datetime.fromisoformat(end_date.replace('Z', '+00:00'))
        
        # Determine optimal iteration strategy
        if strategy.iteration_strategy == IterationStrategy.BATCH_OPTIMIZE:
            await self._execute_batch_backtest(strategy, result, start_dt, end_dt, symbols)
        else:
            await self._execute_timestep_backtest(strategy, result, start_dt, end_dt, symbols)
    
    async def _execute_batch_backtest(
        self,
        strategy: UnifiedStrategy,
        result: UnifiedStrategyResult,
        start_date: datetime,
        end_date: datetime,
        symbols: List[str]
    ):
        """Execute backtest with full data optimization in Python"""
        
        # Prepare comprehensive backtest context
        context = {
            'execution_mode': 'backtest',
            'start_date': start_date.isoformat(),
            'end_date': end_date.isoformat(),
            'symbols': symbols,
            'instances': [],
            'summary': {},
            'performance_metrics': {}
        }
        
        # Wrap strategy for batch backtest execution
        wrapped_code = self._wrap_strategy_for_backtest(strategy.strategy_code, batch_mode=True)
        
        # Execute strategy with full data access
        execution_result = await self.execution_engine.execute(wrapped_code, context)
        
        # Extract backtest results
        result.strategy_results = execution_result
        result.instances = execution_result.get('instances', [])
        result.summary = execution_result.get('summary', {})
        result.performance_metrics = execution_result.get('performance_metrics', {})
    
    async def _execute_timestep_backtest(
        self,
        strategy: UnifiedStrategy,
        result: UnifiedStrategyResult,
        start_date: datetime,
        end_date: datetime,
        symbols: List[str]
    ):
        """Execute backtest with timestep iteration (fallback method)"""
        
        # Generate date range
        current_date = start_date
        all_instances = []
        
        while current_date <= end_date:
            # Execute strategy for each date
            for symbol in symbols[:10]:  # Limit for performance
                context = {
                    'execution_mode': 'backtest_timestep',
                    'current_date': current_date.isoformat(),
                    'symbol': symbol,
                    'classification': False,
                    'strategy_data': {}
                }
                
                wrapped_code = self._wrap_strategy_for_timestep(strategy.strategy_code)
                execution_result = await self.execution_engine.execute(wrapped_code, context)
                
                if execution_result.get('classification', False):
                    instance = {
                        'ticker': symbol,
                        'timestamp': int(current_date.timestamp() * 1000),
                        'classification': True,
                        'strategy_results': execution_result
                    }
                    all_instances.append(instance)
            
            current_date += timedelta(days=1)
        
        result.instances = all_instances
        result.summary = {
            'total_instances': len(all_instances),
            'positive_signals': len(all_instances),
            'date_range': [start_date.isoformat(), end_date.isoformat()]
        }
    
    async def _execute_screening(
        self,
        strategy: UnifiedStrategy,
        result: UnifiedStrategyResult,
        universe: Optional[List[str]] = None,
        limit: int = 100,
        **kwargs
    ):
        """Execute strategy for screening with ranking"""
        
        # Get universe for screening
        if not universe:
            universe = await self._get_optimized_universe(strategy, limit=1000)
        
        # Prepare screening context
        context = {
            'execution_mode': 'screening',
            'universe': universe,
            'current_timestamp': datetime.utcnow().isoformat(),
            'ranked_results': [],
            'scores': {},
            'limit': limit
        }
        
        # Wrap strategy for screening execution
        wrapped_code = self._wrap_strategy_for_screening(strategy.strategy_code)
        
        # Execute strategy
        execution_result = await self.execution_engine.execute(wrapped_code, context)
        
        # Extract screening results
        result.strategy_results = execution_result
        result.ranked_results = execution_result.get('ranked_results', [])
        result.scores = execution_result.get('scores', {})
        result.universe_size = len(universe)
    
    async def _get_optimized_universe(
        self,
        strategy: UnifiedStrategy,
        limit: int = 100
    ) -> List[str]:
        """Get optimized universe based on strategy requirements"""
        
        # Check cache first
        cache_key = f"universe_{strategy.strategy_id}_{limit}"
        if cache_key in self._universe_cache:
            return self._universe_cache[cache_key]
        
        # Build filters based on strategy analysis
        filters = {}
        if strategy.universe_filters.get('requires_sector'):
            filters['sector'] = 'Technology'  # Default, strategy can override
        if strategy.universe_filters.get('requires_fundamentals'):
            filters['min_market_cap'] = 1000000000  # $1B+
        
        # Get universe from data provider
        universe_data = await self.data_provider.scan_universe(
            filters=filters,
            sort_by='market_cap',
            limit=limit
        )
        
        symbols = universe_data.get('symbols', [])
        
        # Cache result
        self._universe_cache[cache_key] = symbols
        
        return symbols
    
    def _wrap_strategy_for_realtime(self, strategy_code: str) -> str:
        """Wrap strategy code for real-time execution"""
        return f'''
# Real-time execution wrapper
import json
from datetime import datetime

# Original strategy code
{strategy_code}

# Real-time execution logic
def execute_realtime():
    symbols = input_data.get('symbols', [])
    alerts = []
    signals = {{}}
    
    try:
        # Strategy should implement real-time logic
        if 'run_realtime_scan' in globals():
            result = run_realtime_scan(symbols)
            if isinstance(result, dict):
                alerts = result.get('alerts', [])
                signals = result.get('signals', {{}})
        else:
            # Fallback: test strategy on current data
            for symbol in symbols[:10]:  # Limit for real-time performance
                try:
                    if 'classify_symbol' in globals():
                        classification = classify_symbol(symbol)
                        if classification:
                            signals[symbol] = {{
                                'signal': True,
                                'timestamp': datetime.utcnow().isoformat(),
                                'symbol': symbol
                            }}
                except Exception as e:
                    continue
        
        save_result('alerts', alerts)
        save_result('signals', signals)
        save_result('execution_mode', 'realtime')
        
    except Exception as e:
        save_result('error', str(e))
        save_result('alerts', [])
        save_result('signals', {{}})

# Execute real-time logic
execute_realtime()
'''
    
    def _wrap_strategy_for_backtest(self, strategy_code: str, batch_mode: bool = True) -> str:
        """Wrap strategy code for backtest execution"""
        if batch_mode:
            return f'''
# Batch backtest execution wrapper
import json
from datetime import datetime, timedelta

# Original strategy code
{strategy_code}

# Batch backtest execution logic
def execute_batch_backtest():
    start_date = input_data.get('start_date')
    end_date = input_data.get('end_date') 
    symbols = input_data.get('symbols', [])
    
    instances = []
    performance_metrics = {{}}
    
    try:
        # Strategy should implement batch backtest logic
        if 'run_batch_backtest' in globals():
            result = run_batch_backtest(start_date, end_date, symbols)
            if isinstance(result, dict):
                instances = result.get('instances', [])
                performance_metrics = result.get('performance_metrics', {{}})
        else:
            # Fallback: run strategy efficiently across time/symbols
            log(f"Running backtest from {{start_date}} to {{end_date}} on {{len(symbols)}} symbols")
            
            # Get bulk data for efficiency
            if 'backtest_bulk_analysis' in globals():
                result = backtest_bulk_analysis(start_date, end_date, symbols)
                instances = result.get('instances', [])
            else:
                # Simple fallback
                for symbol in symbols[:50]:  # Limit for performance
                    try:
                        if 'classify_symbol' in globals():
                            classification = classify_symbol(symbol)
                            if classification:
                                instances.append({{
                                    'ticker': symbol,
                                    'timestamp': int(datetime.utcnow().timestamp() * 1000),
                                    'classification': True,
                                    'strategy_results': {{'symbol': symbol}}
                                }})
                    except Exception as e:
                        continue
        
        summary = {{
            'total_instances': len(instances),
            'positive_signals': len([i for i in instances if i.get('classification', False)]),
            'date_range': [start_date, end_date],
            'symbols_processed': len(symbols)
        }}
        
        save_result('instances', instances)
        save_result('summary', summary)
        save_result('performance_metrics', performance_metrics)
        save_result('execution_mode', 'backtest')
        
    except Exception as e:
        save_result('error', str(e))
        save_result('instances', [])
        save_result('summary', {{}})

# Execute batch backtest
execute_batch_backtest()
'''
        else:
            return f'''
# Timestep backtest wrapper
{strategy_code}

# Execute for single symbol/date
symbol = input_data.get('symbol')
current_date = input_data.get('current_date')

try:
    if 'classify_symbol' in globals():
        result = classify_symbol(symbol)
        save_result('classification', result)
        save_result('symbol', symbol)
        save_result('date', current_date)
    else:
        save_result('classification', False)
except Exception as e:
    save_result('classification', False)
    save_result('error', str(e))
'''
    
    def _wrap_strategy_for_screening(self, strategy_code: str) -> str:
        """Wrap strategy code for screening execution"""
        return f'''
# Screening execution wrapper
import json
from datetime import datetime

# Original strategy code
{strategy_code}

# Screening execution logic
def execute_screening():
    universe = input_data.get('universe', [])
    limit = input_data.get('limit', 100)
    
    ranked_results = []
    scores = {{}}
    
    try:
        # Strategy should implement screening logic
        if 'run_screening' in globals():
            result = run_screening(universe, limit)
            if isinstance(result, dict):
                ranked_results = result.get('ranked_results', [])
                scores = result.get('scores', {{}})
        else:
            # Fallback: score each symbol in universe
            symbol_scores = []
            
            for symbol in universe:
                try:
                    if 'score_symbol' in globals():
                        score = score_symbol(symbol)
                        if isinstance(score, (int, float)) and score > 0:
                            symbol_scores.append({{'symbol': symbol, 'score': score}})
                            scores[symbol] = score
                    elif 'classify_symbol' in globals():
                        classification = classify_symbol(symbol)
                        if classification:
                            symbol_scores.append({{'symbol': symbol, 'score': 1.0}})
                            scores[symbol] = 1.0
                except Exception as e:
                    continue
            
            # Sort by score and limit results
            symbol_scores.sort(key=lambda x: x['score'], reverse=True)
            ranked_results = symbol_scores[:limit]
        
        save_result('ranked_results', ranked_results)
        save_result('scores', scores)
        save_result('universe_size', len(universe))
        save_result('execution_mode', 'screening')
        
    except Exception as e:
        save_result('error', str(e))
        save_result('ranked_results', [])
        save_result('scores', {{}})

# Execute screening
execute_screening()
'''

    async def create_realtime_alert_loop(
        self,
        strategy: UnifiedStrategy,
        symbols: List[str],
        interval_seconds: int = 60
    ):
        """Create a real-time alert monitoring loop"""
        
        logger.info(f"Starting real-time alert loop for strategy {strategy.strategy_id}")
        
        while True:
            try:
                result = await self.execute_strategy(
                    strategy,
                    ExecutionMode.REALTIME,
                    symbols=symbols
                )
                
                if result.success and result.alerts:
                    # Process alerts
                    for alert in result.alerts:
                        logger.info(f"Alert triggered: {alert}")
                        # Here you would send notifications, save to database, etc.
                
                await asyncio.sleep(interval_seconds)
                
            except Exception as e:
                logger.error(f"Error in real-time alert loop: {e}")
                await asyncio.sleep(interval_seconds)


# Example unified strategy patterns
EXAMPLE_UNIFIED_STRATEGIES = {
    'realtime_momentum': '''
def run_realtime_scan(symbols):
    """Real-time momentum scanning strategy"""
    alerts = []
    signals = {}
    
    for symbol in symbols:
        try:
            # Get recent price data
            price_data = get_price_data(symbol, timeframe='1d', days=5)
            if not price_data.get('close') or len(price_data['close']) < 5:
                continue
            
            # Calculate momentum
            prices = price_data['close']
            current_price = prices[-1]
            prev_price = prices[-2]
            five_day_avg = sum(prices[-5:]) / 5
            
            # Momentum signal
            daily_change = (current_price - prev_price) / prev_price
            vs_five_day = (current_price - five_day_avg) / five_day_avg
            
            if daily_change > 0.03 and vs_five_day > 0.05:  # 3% daily, 5% vs 5-day avg
                signals[symbol] = {
                    'signal': True,
                    'strength': daily_change + vs_five_day,
                    'daily_change': daily_change,
                    'vs_five_day': vs_five_day,
                    'current_price': current_price
                }
                
                alerts.append({
                    'symbol': symbol,
                    'type': 'momentum_breakout',
                    'message': f"{symbol} momentum breakout: {daily_change:.2%} daily, {vs_five_day:.2%} vs 5-day avg",
                    'strength': daily_change + vs_five_day
                })
        except Exception as e:
            continue
    
    return {'alerts': alerts, 'signals': signals}
''',

    'batch_backtest_value': '''
def run_batch_backtest(start_date, end_date, symbols):
    """Batch value investing backtest with optimized data access"""
    instances = []
    performance_metrics = {}
    
    log(f"Running value backtest on {len(symbols)} symbols")
    
    # Bulk data loading for efficiency
    universe_data = scan_universe(filters={'min_market_cap': 1000000000}, limit=len(symbols))
    valid_symbols = [s['ticker'] for s in universe_data.get('data', [])]
    
    for symbol in valid_symbols[:100]:  # Limit for demo
        try:
            # Get fundamental data
            fundamentals = get_fundamental_data(symbol)
            if not fundamentals:
                continue
            
            # Get price data
            price_data = get_price_data(symbol, timeframe='1d', days=252)  # 1 year
            if not price_data.get('close') or len(price_data['close']) < 50:
                continue
            
            # Value analysis
            market_cap = fundamentals.get('market_cap', 0)
            book_value = fundamentals.get('book_value', 0)
            eps = fundamentals.get('eps', 0)
            current_price = price_data['close'][-1]
            
            if eps > 0 and book_value > 0:
                pe_ratio = current_price / eps
                pb_ratio = current_price / (book_value / fundamentals.get('shares_outstanding', 1))
                
                # Value criteria
                if pe_ratio < 15 and pb_ratio < 1.5 and market_cap > 1000000000:
                    # Calculate future returns (simplified)
                    entry_price = current_price
                    future_return = 0.15  # Simulated 15% return for value picks
                    
                    instances.append({
                        'ticker': symbol,
                        'timestamp': int(datetime.utcnow().timestamp() * 1000),
                        'classification': True,
                        'entry_price': entry_price,
                        'pe_ratio': pe_ratio,
                        'pb_ratio': pb_ratio,
                        'market_cap': market_cap,
                        'future_return': future_return,
                        'strategy_results': {
                            'value_score': (20 - pe_ratio) + (2 - pb_ratio),
                            'fundamentals': fundamentals
                        }
                    })
        except Exception as e:
            continue
    
    # Calculate performance metrics
    if instances:
        avg_return = sum(i.get('future_return', 0) for i in instances) / len(instances)
        performance_metrics = {
            'total_picks': len(instances),
            'average_return': avg_return,
            'avg_pe': sum(i.get('pe_ratio', 0) for i in instances) / len(instances),
            'avg_pb': sum(i.get('pb_ratio', 0) for i in instances) / len(instances)
        }
    
    return {'instances': instances, 'performance_metrics': performance_metrics}
''',

    'screening_momentum': '''
def run_screening(universe, limit):
    """Momentum screening with custom scoring"""
    scored_symbols = []
    scores = {}
    
    log(f"Screening {len(universe)} symbols for momentum")
    
    for symbol in universe:
        try:
            # Get price and volume data
            price_data = get_price_data(symbol, timeframe='1d', days=30)
            if not price_data.get('close') or len(price_data['close']) < 20:
                continue
            
            prices = price_data['close']
            volumes = price_data['volume']
            current_price = prices[-1]
            
            # Calculate momentum factors
            returns_5d = (prices[-1] / prices[-6]) - 1 if len(prices) >= 6 else 0
            returns_20d = (prices[-1] / prices[-21]) - 1 if len(prices) >= 21 else 0
            
            # Volume analysis
            avg_volume_20d = sum(volumes[-20:]) / 20 if len(volumes) >= 20 else 0
            recent_volume = volumes[-1]
            volume_ratio = recent_volume / avg_volume_20d if avg_volume_20d > 0 else 0
            
            # Combined momentum score
            momentum_score = (returns_5d * 2) + returns_20d + (volume_ratio * 0.1)
            
            if momentum_score > 0.1:  # 10% threshold
                scored_symbols.append({
                    'symbol': symbol,
                    'score': momentum_score,
                    'returns_5d': returns_5d,
                    'returns_20d': returns_20d,
                    'volume_ratio': volume_ratio,
                    'current_price': current_price
                })
                scores[symbol] = momentum_score
                
        except Exception as e:
            continue
    
    # Sort by score and return top results
    scored_symbols.sort(key=lambda x: x['score'], reverse=True)
    ranked_results = scored_symbols[:limit]
    
    return {'ranked_results': ranked_results, 'scores': scores}
'''
} 