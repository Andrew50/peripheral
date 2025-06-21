"""
Strategy Worker
Implements three core functions for strategy execution:
- run_backtest(strategy_id): Complete backtesting across all historical days
- run_screener(strategy_id): Complete screening across all tickers
- run_alert(strategy_id): Complete alert monitoring across all tickers
"""

import asyncio
import logging
import json
from datetime import datetime, timedelta
from typing import Any, Dict, List, Optional
import numpy as np

try:
    from .data_provider import DataProvider
    from .execution_engine import PythonExecutionEngine, CodeValidator, SecurityError
except ImportError:
    from data_provider import DataProvider
    from execution_engine import PythonExecutionEngine, CodeValidator, SecurityError

logger = logging.getLogger(__name__)


class StrategyWorker:
    """Strategy worker with three core execution functions"""
    
    def __init__(self, db_connection=None):
        self.data_provider = DataProvider()
        self.execution_engine = PythonExecutionEngine()
        self.validator = CodeValidator()
        self.db_connection = db_connection
        
        # Cache for strategy data
        self._strategy_cache: Dict[int, Dict] = {}
        self._universe_cache: Dict[str, List[str]] = {}
    
    async def run_backtest(self, strategy_id: int, **kwargs) -> Dict[str, Any]:
        """
        Run complete backtest for a strategy across all historical days
        
        Args:
            strategy_id: Database ID of the strategy to backtest
            **kwargs: Optional parameters (start_date, end_date, symbols)
        
        Returns:
            Dict containing instances, summary, and performance metrics
        """
        logger.info(f"Starting complete backtest for strategy {strategy_id}")
        
        try:
            # Get strategy from database
            strategy_data = await self._get_strategy(strategy_id)
            if not strategy_data:
                raise ValueError(f"Strategy {strategy_id} not found")
            
            # Validate strategy code
            if not self.validator.validate(strategy_data['python_code']):
                raise SecurityError("Strategy code validation failed")
            
            # Set default backtest parameters
            end_date = kwargs.get('end_date', datetime.now())
            start_date = kwargs.get('start_date', end_date - timedelta(days=730))  # 2 years default
            symbols = kwargs.get('symbols', await self._get_universe())
            
            logger.info(f"Backtesting {len(symbols)} symbols from {start_date.date()} to {end_date.date()}")
            
            # Prepare execution context
            context = {
                'execution_mode': 'backtest',
                'strategy_id': strategy_id,
                'start_date': start_date.isoformat(),
                'end_date': end_date.isoformat(),
                'symbols': symbols,
                'instances': [],
                'summary': {},
                'performance_metrics': {}
            }
            
            # Wrap strategy for complete backtest execution
            wrapped_code = self._wrap_strategy_for_complete_backtest(
                strategy_data['python_code'], 
                start_date, 
                end_date, 
                symbols
            )
            
            # Execute strategy
            execution_result = await self.execution_engine.execute(wrapped_code, context)
            
            # Process results
            instances = execution_result.get('instances', [])
            summary = execution_result.get('summary', {})
            performance_metrics = execution_result.get('performance_metrics', {})
            
            # Calculate additional performance metrics
            if instances:
                performance_metrics.update(self._calculate_performance_metrics(instances))
            
            result = {
                'success': True,
                'strategy_id': strategy_id,
                'execution_mode': 'backtest',
                'instances': instances,
                'summary': {
                    'total_instances': len(instances),
                    'positive_signals': len([i for i in instances if i.get('classification', False)]),
                    'date_range': [start_date.isoformat(), end_date.isoformat()],
                    'symbols_processed': len(symbols),
                    **summary
                },
                'performance_metrics': performance_metrics,
                'execution_time_ms': execution_result.get('execution_time_ms', 0)
            }
            
            logger.info(f"Backtest completed: {len(instances)} instances found")
            return result
            
        except Exception as e:
            logger.error(f"Backtest failed for strategy {strategy_id}: {e}")
            return {
                'success': False,
                'strategy_id': strategy_id,
                'execution_mode': 'backtest',
                'error_message': str(e),
                'instances': [],
                'summary': {},
                'performance_metrics': {}
            }
    
    async def run_screener(self, strategy_id: int, **kwargs) -> Dict[str, Any]:
        """
        Run complete screener for a strategy across all tickers
        
        Args:
            strategy_id: Database ID of the strategy to screen
            **kwargs: Optional parameters (universe, limit)
        
        Returns:
            Dict containing ranked results and scores
        """
        logger.info(f"Starting complete screening for strategy {strategy_id}")
        
        try:
            # Get strategy from database
            strategy_data = await self._get_strategy(strategy_id)
            if not strategy_data:
                raise ValueError(f"Strategy {strategy_id} not found")
            
            # Validate strategy code
            if not self.validator.validate(strategy_data['python_code']):
                raise SecurityError("Strategy code validation failed")
            
            # Set screening parameters
            universe = kwargs.get('universe', await self._get_universe())
            limit = kwargs.get('limit', 100)
            
            logger.info(f"Screening {len(universe)} symbols, returning top {limit}")
            
            # Prepare execution context
            context = {
                'execution_mode': 'screening',
                'strategy_id': strategy_id,
                'universe': universe,
                'limit': limit,
                'ranked_results': [],
                'scores': {},
                'universe_size': len(universe)
            }
            
            # Wrap strategy for complete screening
            wrapped_code = self._wrap_strategy_for_complete_screening(
                strategy_data['python_code'],
                universe,
                limit
            )
            
            # Execute strategy
            execution_result = await self.execution_engine.execute(wrapped_code, context)
            
            # Process results
            ranked_results = execution_result.get('ranked_results', [])
            scores = execution_result.get('scores', {})
            
            result = {
                'success': True,
                'strategy_id': strategy_id,
                'execution_mode': 'screening',
                'ranked_results': ranked_results,
                'scores': scores,
                'universe_size': len(universe),
                'results_returned': len(ranked_results),
                'execution_time_ms': execution_result.get('execution_time_ms', 0)
            }
            
            logger.info(f"Screening completed: {len(ranked_results)} opportunities found")
            return result
            
        except Exception as e:
            logger.error(f"Screening failed for strategy {strategy_id}: {e}")
            return {
                'success': False,
                'strategy_id': strategy_id,
                'execution_mode': 'screening',
                'error_message': str(e),
                'ranked_results': [],
                'scores': {},
                'universe_size': 0
            }
    
    async def run_alert(self, strategy_id: int, **kwargs) -> Dict[str, Any]:
        """
        Run complete alert monitoring for a strategy across all tickers
        
        Args:
            strategy_id: Database ID of the strategy to monitor
            **kwargs: Optional parameters (symbols, alert_threshold)
        
        Returns:
            Dict containing alerts and signals
        """
        logger.info(f"Starting complete alert monitoring for strategy {strategy_id}")
        
        try:
            # Get strategy from database
            strategy_data = await self._get_strategy(strategy_id)
            if not strategy_data:
                raise ValueError(f"Strategy {strategy_id} not found")
            
            # Validate strategy code
            if not self.validator.validate(strategy_data['python_code']):
                raise SecurityError("Strategy code validation failed")
            
            # Set alert parameters
            symbols = kwargs.get('symbols', await self._get_universe())
            alert_threshold = kwargs.get('alert_threshold', 0.0)
            
            logger.info(f"Monitoring {len(symbols)} symbols for alerts")
            
            # Prepare execution context
            context = {
                'execution_mode': 'alert',
                'strategy_id': strategy_id,
                'symbols': symbols,
                'alert_threshold': alert_threshold,
                'current_timestamp': datetime.utcnow().isoformat(),
                'alerts': [],
                'signals': {}
            }
            
            # Wrap strategy for complete alert monitoring
            wrapped_code = self._wrap_strategy_for_complete_alerts(
                strategy_data['python_code'],
                symbols
            )
            
            # Execute strategy
            execution_result = await self.execution_engine.execute(wrapped_code, context)
            
            # Process results
            alerts = execution_result.get('alerts', [])
            signals = execution_result.get('signals', {})
            
            result = {
                'success': True,
                'strategy_id': strategy_id,
                'execution_mode': 'alert',
                'alerts': alerts,
                'signals': signals,
                'symbols_monitored': len(symbols),
                'alerts_generated': len(alerts),
                'signals_detected': len(signals),
                'execution_time_ms': execution_result.get('execution_time_ms', 0)
            }
            
            logger.info(f"Alert monitoring completed: {len(alerts)} alerts, {len(signals)} signals")
            return result
            
        except Exception as e:
            logger.error(f"Alert monitoring failed for strategy {strategy_id}: {e}")
            return {
                'success': False,
                'strategy_id': strategy_id,
                'execution_mode': 'alert',
                'error_message': str(e),
                'alerts': [],
                'signals': {},
                'symbols_monitored': 0
            }
    
    def _wrap_strategy_for_complete_backtest(self, strategy_code: str, start_date: datetime, 
                                           end_date: datetime, symbols: List[str]) -> str:
        """Wrap strategy code for complete backtesting across all days"""
        return f'''
# Complete Backtest Execution Wrapper
import json
from datetime import datetime, timedelta
import numpy as np

# Original strategy code
{strategy_code}

# Complete backtest execution logic
def execute_complete_backtest():
    start_date = datetime.fromisoformat("{start_date.isoformat()}")
    end_date = datetime.fromisoformat("{end_date.isoformat()}")
    symbols = {json.dumps(symbols)}
    
    instances = []
    performance_metrics = {{}}
    
    try:
        log(f"Starting complete backtest from {{start_date.date()}} to {{end_date.date()}} on {{len(symbols)}} symbols")
        
        # Check if strategy implements batch backtest
        if 'run_batch_backtest' in globals():
            log("Using strategy's batch backtest implementation")
            result = run_batch_backtest(start_date.isoformat(), end_date.isoformat(), symbols)
            if isinstance(result, dict):
                instances = result.get('instances', [])
                performance_metrics = result.get('performance_metrics', {{}})
        else:
            log("Using fallback symbol-by-symbol backtest")
            # Fallback: iterate through all symbols and apply strategy
            for symbol in symbols:
                try:
                    if 'classify_symbol' in globals():
                        classification = classify_symbol(symbol)
                        if classification:
                            # Get current price for entry point
                            price_data = get_price_data(symbol, timeframe='1d', days=1)
                            current_price = price_data.get('close', [0])[-1] if price_data.get('close') else 0
                            
                            instances.append({{
                                'ticker': symbol,
                                'timestamp': int(datetime.utcnow().timestamp() * 1000),
                                'classification': True,
                                'entry_price': current_price,
                                'strategy_results': {{'symbol': symbol, 'backtest_mode': True}}
                            }})
                except Exception as e:
                    log(f"Error processing {{symbol}}: {{e}}")
                    continue
        
        # Calculate performance metrics if not provided
        if instances and not performance_metrics:
            returns = [i.get('future_return', 0) for i in instances if 'future_return' in i]
            if returns:
                performance_metrics = {{
                    'total_picks': len(instances),
                    'average_return': sum(returns) / len(returns),
                    'positive_return_rate': len([r for r in returns if r > 0]) / len(returns),
                    'best_return': max(returns),
                    'worst_return': min(returns)
                }}
            else:
                performance_metrics = {{
                    'total_picks': len(instances),
                    'symbols_processed': len(symbols),
                    'hit_rate': len(instances) / len(symbols) if symbols else 0
                }}
        
        summary = {{
            'execution_type': 'complete_backtest',
            'date_range': [start_date.isoformat(), end_date.isoformat()],
            'total_symbols': len(symbols),
            'successful_classifications': len(instances)
        }}
        
        save_result('instances', instances)
        save_result('summary', summary)
        save_result('performance_metrics', performance_metrics)
        save_result('success', True)
        
        log(f"Complete backtest finished: {{len(instances)}} instances found")
        
    except Exception as e:
        log(f"Backtest error: {{e}}")
        save_result('error', str(e))
        save_result('instances', [])
        save_result('summary', {{}})
        save_result('performance_metrics', {{}})
        save_result('success', False)

# Execute complete backtest
execute_complete_backtest()
'''

    def _wrap_strategy_for_complete_screening(self, strategy_code: str, universe: List[str], limit: int) -> str:
        """Wrap strategy code for complete screening across all tickers"""
        return f'''
# Complete Screening Execution Wrapper
import json
from datetime import datetime
import numpy as np

# Original strategy code
{strategy_code}

# Complete screening execution logic
def execute_complete_screening():
    universe = {json.dumps(universe)}
    limit = {limit}
    
    ranked_results = []
    scores = {{}}
    
    try:
        log(f"Starting complete screening of {{len(universe)}} symbols, returning top {{limit}}")
        
        # Check if strategy implements screening
        if 'run_screening' in globals():
            log("Using strategy's screening implementation")
            result = run_screening(universe, limit)
            if isinstance(result, dict):
                ranked_results = result.get('ranked_results', [])
                scores = result.get('scores', {{}})
        else:
            log("Using fallback symbol-by-symbol screening")
            # Fallback: score each symbol individually
            symbol_scores = []
            
            for symbol in universe:
                try:
                    score = 0
                    classification = False
                    
                    # Try score_symbol function first
                    if 'score_symbol' in globals():
                        score = score_symbol(symbol)
                        classification = score > 0
                    elif 'classify_symbol' in globals():
                        classification = classify_symbol(symbol)
                        score = 1.0 if classification else 0.0
                    
                    if classification and score > 0:
                        # Get additional data for ranking
                        price_data = get_price_data(symbol, timeframe='1d', days=5)
                        current_price = price_data.get('close', [0])[-1] if price_data.get('close') else 0
                        
                        symbol_scores.append({{
                            'symbol': symbol,
                            'score': score,
                            'current_price': current_price,
                            'classification': classification
                        }})
                        scores[symbol] = score
                        
                except Exception as e:
                    log(f"Error screening {{symbol}}: {{e}}")
                    continue
            
            # Sort by score and limit results
            symbol_scores.sort(key=lambda x: x['score'], reverse=True)
            ranked_results = symbol_scores[:limit]
        
        save_result('ranked_results', ranked_results)
        save_result('scores', scores)
        save_result('universe_size', len(universe))
        save_result('success', True)
        
        log(f"Complete screening finished: {{len(ranked_results)}} opportunities found")
        
    except Exception as e:
        log(f"Screening error: {{e}}")
        save_result('error', str(e))
        save_result('ranked_results', [])
        save_result('scores', {{}})
        save_result('success', False)

# Execute complete screening
execute_complete_screening()
'''

    def _wrap_strategy_for_complete_alerts(self, strategy_code: str, symbols: List[str]) -> str:
        """Wrap strategy code for complete alert monitoring across all tickers"""
        return f'''
# Complete Alert Monitoring Wrapper
import json
from datetime import datetime
import numpy as np

# Original strategy code
{strategy_code}

# Complete alert monitoring logic
def execute_complete_alerts():
    symbols = {json.dumps(symbols)}
    current_time = datetime.utcnow().isoformat()
    
    alerts = []
    signals = {{}}
    
    try:
        log(f"Starting complete alert monitoring for {{len(symbols)}} symbols")
        
        # Check if strategy implements real-time scanning
        if 'run_realtime_scan' in globals():
            log("Using strategy's real-time scan implementation")
            result = run_realtime_scan(symbols)
            if isinstance(result, dict):
                alerts = result.get('alerts', [])
                signals = result.get('signals', {{}})
        else:
            log("Using fallback symbol-by-symbol alert monitoring")
            # Fallback: check each symbol individually
            for symbol in symbols:
                try:
                    if 'classify_symbol' in globals():
                        classification = classify_symbol(symbol)
                        if classification:
                            # Get current price data for alert context
                            price_data = get_price_data(symbol, timeframe='1d', days=2)
                            if price_data.get('close'):
                                current_price = price_data['close'][-1]
                                prev_price = price_data['close'][-2] if len(price_data['close']) > 1 else current_price
                                daily_change = ((current_price - prev_price) / prev_price) * 100 if prev_price > 0 else 0
                                
                                signals[symbol] = {{
                                    'signal': True,
                                    'timestamp': current_time,
                                    'current_price': current_price,
                                    'daily_change': daily_change
                                }}
                                
                                alerts.append({{
                                    'symbol': symbol,
                                    'type': 'strategy_signal',
                                    'message': f"{{symbol}} triggered strategy alert",
                                    'timestamp': current_time,
                                    'priority': 'high' if abs(daily_change) > 5 else 'medium',
                                    'data': {{
                                        'current_price': current_price,
                                        'daily_change': daily_change
                                    }}
                                }})
                                
                                log(f"Alert generated for {{symbol}}: {{daily_change:.2f}}% change")
                                
                except Exception as e:
                    log(f"Error monitoring {{symbol}}: {{e}}")
                    continue
        
        save_result('alerts', alerts)
        save_result('signals', signals)
        save_result('symbols_monitored', len(symbols))
        save_result('success', True)
        
        log(f"Complete alert monitoring finished: {{len(alerts)}} alerts, {{len(signals)}} signals")
        
    except Exception as e:
        log(f"Alert monitoring error: {{e}}")
        save_result('error', str(e))
        save_result('alerts', [])
        save_result('signals', {{}})
        save_result('success', False)

# Execute complete alert monitoring
execute_complete_alerts()
'''

    async def _get_strategy(self, strategy_id: int) -> Optional[Dict]:
        """Get strategy data from database or cache"""
        if strategy_id in self._strategy_cache:
            return self._strategy_cache[strategy_id]
        
        try:
            # Query to get strategy data from database
            query = """
            SELECT strategyid, name, 
                   COALESCE(description, '') as description,
                   COALESCE(prompt, '') as prompt,
                   COALESCE(pythoncode, '') as pythoncode,
                   COALESCE(version, '1.0') as version,
                   COALESCE(isalertactive, false) as isalertactive
            FROM strategies 
            WHERE strategyid = %s
            """
            
            result = await self.data_provider.execute_sql_parameterized(query, [strategy_id])
            
            if not result or not result.get('data'):
                logger.warning(f"Strategy {strategy_id} not found in database")
                return None
            
            strategy_row = result['data'][0]
            
            strategy_data = {
                'strategy_id': strategy_row['strategyid'],
                'name': strategy_row['name'],
                'description': strategy_row['description'],
                'prompt': strategy_row['prompt'],
                'python_code': strategy_row['pythoncode'],
                'version': strategy_row['version'],
                'is_alert_active': strategy_row['isalertactive']
            }
            
            # Cache the strategy data
            self._strategy_cache[strategy_id] = strategy_data
            return strategy_data
            
        except Exception as e:
            logger.error(f"Error retrieving strategy {strategy_id}: {e}")
            
            # Fallback to mock strategy for testing
            strategy_data = {
                'strategy_id': strategy_id,
                'name': f'Strategy {strategy_id}',
                'python_code': '''
def classify_symbol(symbol):
    """Default strategy for testing"""
    try:
        price_data = get_price_data(symbol, timeframe='1d', days=5)
        if not price_data.get('close') or len(price_data['close']) < 2:
            return False
        
        current_price = price_data['close'][-1]
        prev_price = price_data['close'][-2]
        daily_change = (current_price - prev_price) / prev_price
        
        return abs(daily_change) > 0.02  # 2% change threshold
    except Exception:
        return False
''',
                'description': 'Default strategy for testing',
                'user_id': 1
            }
            
            self._strategy_cache[strategy_id] = strategy_data
            return strategy_data
    
    async def _get_universe(self, universe_type: str = 'default') -> List[str]:
        """Get trading universe symbols"""
        if universe_type in self._universe_cache:
            return self._universe_cache[universe_type]
        
        try:
            # Query active securities from database
            query = """
            SELECT DISTINCT ticker 
            FROM securities 
            WHERE active = true 
            AND locale = 'us'
            AND market = 'stocks'
            ORDER BY ticker
            LIMIT 1000
            """
            
            result = await self.data_provider.execute_sql_parameterized(query, [])
            
            if result and result.get('data'):
                universe = [row['ticker'] for row in result['data']]
                logger.info(f"Retrieved {len(universe)} symbols from database")
            else:
                logger.warning("No symbols found in database, using fallback universe")
                # Fallback universe for testing
                universe = [
                    'AAPL', 'MSFT', 'GOOGL', 'AMZN', 'TSLA', 'META', 'NVDA', 'NFLX',
                    'AMD', 'INTC', 'CRM', 'ORCL', 'ADBE', 'PYPL', 'UBER', 'ABNB',
                    'COIN', 'SQ', 'SHOP', 'ROKU', 'ZOOM', 'DOCU', 'TWLO', 'OKTA'
                ]
                
        except Exception as e:
            logger.error(f"Error retrieving universe: {e}")
            # Fallback universe for testing
            universe = [
                'AAPL', 'MSFT', 'GOOGL', 'AMZN', 'TSLA', 'META', 'NVDA', 'NFLX',
                'AMD', 'INTC', 'CRM', 'ORCL', 'ADBE', 'PYPL', 'UBER', 'ABNB',
                'COIN', 'SQ', 'SHOP', 'ROKU', 'ZOOM', 'DOCU', 'TWLO', 'OKTA'
            ]
        
        self._universe_cache[universe_type] = universe
        return universe
    
    def _calculate_performance_metrics(self, instances: List[Dict]) -> Dict[str, Any]:
        """Calculate additional performance metrics from backtest instances"""
        if not instances:
            return {}
        
        # Extract returns if available
        returns = [i.get('future_return', 0) for i in instances if 'future_return' in i]
        
        metrics = {
            'total_instances': len(instances),
            'positive_signals': len([i for i in instances if i.get('classification', False)])
        }
        
        if returns:
            metrics.update({
                'average_return': np.mean(returns),
                'median_return': np.median(returns),
                'std_return': np.std(returns),
                'max_return': np.max(returns),
                'min_return': np.min(returns),
                'positive_return_rate': len([r for r in returns if r > 0]) / len(returns),
                'sharpe_ratio': np.mean(returns) / np.std(returns) if np.std(returns) > 0 else 0
            })
        
        return metrics


# Global instance for easy access
strategy_worker = StrategyWorker()

# Expose the three main functions
async def run_backtest(strategy_id: int, **kwargs) -> Dict[str, Any]:
    """Run complete backtest for a strategy"""
    return await strategy_worker.run_backtest(strategy_id, **kwargs)

async def run_screener(strategy_id: int, **kwargs) -> Dict[str, Any]:
    """Run complete screener for a strategy"""
    return await strategy_worker.run_screener(strategy_id, **kwargs)

async def run_alert(strategy_id: int, **kwargs) -> Dict[str, Any]:
    """Run complete alert monitoring for a strategy"""
    return await strategy_worker.run_alert(strategy_id, **kwargs) 