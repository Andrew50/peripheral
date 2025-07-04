"""
Strategy Generator
Generates trading strategies from natural language using OpenAI o3 and validates them immediately.
"""

import os
import json
import logging
import asyncio
import traceback
import psycopg2
from psycopg2.extras import RealDictCursor
from datetime import datetime, time
from typing import Dict, Any, Optional, List
import re
import time
import threading
from contextlib import contextmanager

from openai import OpenAI
from validator import SecurityValidator, SecurityError, StrategyComplianceError
from accessor_strategy_engine import AccessorStrategyEngine

logger = logging.getLogger(__name__)

# Add rate limiting for database operations
class RateLimiter:
    def __init__(self, max_requests_per_minute=30):
        self.max_requests = max_requests_per_minute
        self.requests = []
        self.lock = threading.Lock()
    
    def can_proceed(self):
        with self.lock:
            now = time.time()
            # Remove requests older than 1 minute
            self.requests = [req_time for req_time in self.requests if now - req_time < 60]
            
            if len(self.requests) < self.max_requests:
                self.requests.append(now)
                return True
            return False
    
    def wait_if_needed(self):
        while not self.can_proceed():
            time.sleep(1)  # Wait 1 second before retrying

# Global rate limiter for database operations
db_rate_limiter = RateLimiter(max_requests_per_minute=20)

class StrategyGenerator:
    """Generates and validates trading strategies using OpenAI o3"""
    
    def __init__(self):
        self.validator = SecurityValidator()
        self.openai_client = None
        self._init_openai_client()
        
    def _init_openai_client(self):
        """Initialize OpenAI client"""
        api_key = os.getenv('OPENAI_API_KEY')
        if not api_key:
            raise ValueError("OPENAI_API_KEY environment variable is required")
        
        self.openai_client = OpenAI(api_key=api_key)
        logger.info("OpenAI client initialized successfully")
    
    def _get_current_filter_values(self) -> Dict[str, List[str]]:
        """Get current available filter values from database - REQUIRED"""
        try:
            # Apply rate limiting to prevent connection storms
            db_rate_limiter.wait_if_needed()
            
            from data_accessors import DataAccessorProvider
            accessor = DataAccessorProvider()
            db_values = accessor.get_available_filter_values()
            
            # Validate that we got actual data
            required_keys = ['sectors', 'industries', 'primary_exchanges', 'locales']
            for key in required_keys:
                if key not in db_values or not db_values[key]:
                    raise ValueError(f"Database returned empty {key} list")
            
            logger.info(f"✅ Fetched current filter values: {len(db_values['sectors'])} sectors, {len(db_values['industries'])} industries")
            return db_values
            
        except Exception as e:
            logger.error(f"❌ CRITICAL: Could not fetch current filter values from database: {e}")
            raise RuntimeError(f"Strategy generation requires database connection to get current filter values: {e}") from e
    
    def _get_system_instruction(self) -> str:
        """Get system instruction for OpenAI code generation with current database filter values"""
        
        # Get current filter values from database
        filter_values = self._get_current_filter_values()
        
        # Format filter values for the prompt
        sectors_str = '", "'.join(filter_values['sectors'])
        industries_str = '", "'.join(filter_values['industries'])
        exchanges_str = '", "'.join(filter_values['primary_exchanges'])
        locales_str = '", "'.join(filter_values['locales'])
        
        return f"""You are a trading strategy generator that creates Python functions using data accessor functions.

ALLOWED IMPORTS: 
- pandas, numpy, datetime, math, plotly

FUNCTION VALIDATION - ONLY THESE FUNCTIONS EXIST:
- get_bar_data(timeframe, columns, min_bars, filters) → numpy.ndarray
- get_general_data(columns, filters) → pandas.DataFrame

These functions are automatically available in the execution environment.

❌ THESE FUNCTIONS DO NOT EXIST:
get_security_details(), get_price_data(), get_fundamental_data(), get_multiple_symbols_data(), etc.

CRITICAL REQUIREMENTS:
- Function named 'strategy()' with NO parameters
- Use data accessor functions with filters:
  * get_bar_data(timeframe="1d", columns=[], min_bars=1, filters={{"tickers": ["AAPL", "MRNA"]}}) -> numpy array
     Columns: ticker, timestamp, open, high, low, close, volume
     
     SUPPORTED TIMEFRAMES:
     • Direct table access: "1m", "1h", "1d", "1w" (fastest, use when available)
     • Custom aggregations: "5m", "10m", "15m", "30m" (from 1-minute data)
                           "2h", "4h", "6h", "8h", "12h" (from 1-hour data)  
                           "2w", "3w", "4w" (from 1-week data)
     
     TIMEFRAME SELECTION GUIDE:
     - Scalping/Day Trading: Use "1m", "5m", "15m", "30m"
     - Swing Trading: Use "1h", "4h", "1d" 
     - Position Trading: Use "1d", "1w", "2w"
     - Multi-timeframe: Combine different intervals for confirmation
     
     Min_bars: This is the minimum number of bars needed to determine whether an instance is valid. 
        - This cannot exceed 10,000. Use the minimum needed.
        - 1 bar: Simple current patterns (volume spikes, price thresholds)
        - 2 bars: Patterns using shift() for previous values (gaps, daily changes)
        - 20+ bars: Technical indicators (moving averages, RSI)
        - 10,000 bars: This is the maximum number of bars that can be used. If you need more than 10,000 bars, you should use the 1d timeframe.

  * get_bar_data(timeframe="1d", aggregate_mode=True, filters={{}}) 
     Use aggregate_mode=True ONLY when you need ALL market data together for calculations like market averages
  * get_general_data(columns=[], filters={{"tickers": ["AAPL", "MRNA"]}}) -> pandas DataFrame  
     Columns: ticker, name, sector, industry, market_cap, market, locale, primary_exchange, active, description, cik, total_shares

AVAILABLE FILTERS (use in filters parameter):
- sector: "{sectors_str}"
- industry: "{industries_str}"
- primary_exchange: "{exchanges_str}"
- market_cap_min: float (e.g., 1000000000 for $1B minimum)
- market_cap_max: float (e.g., 10000000000 for $10B maximum)

FILTER EXAMPLES:
- Technology stocks: filters={{"sector": "Technology"}}
- Large cap healthcare: filters={{"sector": "Healthcare", "market_cap_min": 10000000000}}
- NASDAQ biotech: filters={{"industry": "Biotechnology", "primary_exchange": "NASDAQ"}}
- Small cap stocks: filters={{"market_cap_max": 2000000000}}
- Specific tickers: filters={{"tickers": ["AAPL", "MRNA", "TSLA"]}}

EXECUTION NOTE: Data requests are automatically batched during execution for efficiency - you don't need to worry about this.

TICKER USAGE:
- Always use ticker symbols (strings) like "MRNA", "AAPL", "TSLA" in filters={{"tickers": ["SYMBOL"]}}
- For specific tickers mentioned in prompts, use filters={{"tickers": ["TICKER_NAME"]}}
- For universe-wide strategies, use filters={{}} or filters with sector/industry constraints
- Return results with 'ticker' field (string), not 'securityid'

CRITICAL: RETURN ALL MATCHING INSTANCES, NOT JUST THE LATEST
- DO NOT use .tail(1) or .head(1) to limit results per ticker
- Return every occurrence that meets your criteria across the entire dataset
- The execution engine will handle filtering for different modes (backtest, screening, alerts)
- Example: If MRNA gaps up 1% on 5 different days, return all 5 instances

CRITICAL: INSTANCE STRUCTURE
- DO NOT include 'signal': True field - if returned, it inherently met criteria
- Include relevant price data: 'open', 'close', 'entry_price' when available
                - Use proper timestamp format: int(row['timestamp']) for Unix timestamp (in seconds)
- REQUIRED: Include 'score': float (0.0 to 1.0) - higher score = stronger signal

CRITICAL: ALWAYS INCLUDE INDICATOR VALUES IN INSTANCES
- MUST include ALL calculated indicator values that triggered your strategy
- Examples: 'rsi': 75.2, 'macd': 0.45, 'volume_ratio': 2.3, 'gap_percent': 4.1
- Include intermediate calculations: 'sma_20': 150.5, 'ema_12': 148.2, 'bb_upper': 155.0
- Include percentage changes: 'change_1d_pct': 3.2, 'change_5d_pct': 8.7
- Include ratios and scores: 'momentum_score': 0.85, 'strength_ratio': 1.4
- DO NOT include static thresholds or constants (e.g., 'rsi_threshold': 30)
- This data is ESSENTIAL for backtesting, analysis, and understanding why signals triggered

CRITICAL: min_bars MUST BE ABSOLUTE MINIMUM - NO BUFFERS
- min_bars = EXACT number of bars required for calculation, NOT a suggestion
- Examples: RSI needs 14 bars → min_bars=14, MACD needs 26 bars → min_bars=26
- NEVER add buffer periods like "need 20 + 5 buffer = 25"
- If you need multiple indicators, use the MAXIMUM of their individual minimums
- Example: RSI(14) + SMA(50) strategy → min_bars=50 (not 64, not 55)

DATA VALIDATION:
- Always check if data is None or empty before processing: if data is None or len(data) == 0: return []
- Use proper DataFrame column checks when needed: if 'column_name' in df.columns
- Handle missing data gracefully with pandas methods like dropna()
- Return empty list [] when no valid data is available

CRITICAL: DATA TYPE SAFETY FOR QUANTILE/STATISTICAL OPERATIONS:
- Always convert calculated columns to numeric before groupby operations:
  df['calculated_column'] = pd.to_numeric(df['calculated_column'], errors='coerce')
- Remove NaN values before quantile operations:
  df = df.dropna(subset=['calculated_column'])
- For percentage calculations, ensure no division by zero:
  df = df[df['denominator'] != 0]
- Example safe quantile calculation:
  df['change_pct'] = pd.to_numeric(df['change_pct'], errors='coerce')
  df = df.dropna(subset=['change_pct'])
  quantile_val = df.groupby('timestamp')['change_pct'].quantile(0.9)

CRITICAL: TIMESTAMP FORMAT AND CONVERSION:
- Timestamps returned by get_bar_data() are Unix timestamps in SECONDS (not milliseconds)
- When converting to datetime, always use unit="s":
  df['dt'] = pd.to_datetime(df['timestamp'], unit="s")  # CORRECT
- NEVER use unit="ms" as this will cause incorrect datetime conversions
- For time-based filtering, convert to datetime first, then use .dt accessor for time operations
- For market hours (like Friday 3:45-3:55 PM), convert to Eastern Time:
  df['datetime_et'] = pd.to_datetime(df['timestamp'], unit='s').dt.tz_localize('UTC').dt.tz_convert('America/New_York')

X-MINUTE TIMEFRAME AND TIME ALIGNMENT:
- X-minute bars may not align exactly with specific times like 15:45, 15:55
- Use time ranges instead of exact matches: (time >= 15:45) & (time <= 15:50) for 15:45-15:50 period

ERROR HANDLING NOTE:
- The strategy executor automatically wraps your strategy function in try-except blocks
- You do NOT need to include try-except in your strategy code
- Focus on the strategy logic - error handling is managed by the system
- If data is invalid, simply return an empty list: return []

EXAMPLE PATTERNS:
```python
# Example 1: Volume Breakout - demonstrates returning ALL instances (critical concept)
def strategy():
    instances = []
    
    bar_data = get_bar_data(
        timeframe="1d",
        columns=["ticker", "timestamp", "close", "volume"],
        min_bars=20,  # Need 20 bars for volume average calculation
        filters={{"market_cap_min": 1000000000}}  # Large cap stocks
    )
    
    if bar_data is None or len(bar_data) == 0:
        return instances
    
    df = pd.DataFrame(bar_data, columns=["ticker", "timestamp", "close", "volume"])
    df = df.sort_values(['ticker', 'timestamp']).reset_index(drop=True)
    
    # Calculate 20-day volume average
    df['volume_avg_20'] = df.groupby('ticker')['volume'].rolling(20).mean().reset_index(0, drop=True)
    df = df.dropna()  # Remove rows with NaN values
    df['volume_ratio'] = df['volume'] / df['volume_avg_20']
    
    # CRITICAL: Find ALL volume breakouts (no .tail(1) or .groupby().last())
    breakouts = df[df['volume_ratio'] >= 2.0]  # Volume 2x average
    
    for _, row in breakouts.iterrows():
        instances.append({{
            'ticker': row['ticker'],
            'timestamp': int(row['timestamp']),
            'entry_price': float(row['close']),
            # CRITICAL: Include ALL calculated indicator values that triggered the signal
            'volume_ratio': round(float(row['volume_ratio']), 3),
            'volume': int(row['volume']),
            'volume_avg_20': int(row['volume_avg_20']),
            'volume_breakout_strength': round(float(row['volume_ratio'] - 2.0), 3),
            'score': round(min(1.0, (row['volume_ratio'] - 2.0) / 3.0), 3)
        }})
        
    return instances

# Example 2: Multi-timeframe RSI + Trend Strategy - demonstrates proper indicator inclusion
def strategy():
    instances = []
    
    # Get daily data for RSI calculation
    bar_data_1d = get_bar_data(
        timeframe="1d",
        columns=["ticker", "timestamp", "close"],
        min_bars=20,  # Need 20 bars for RSI calculation (14 + buffer)
        filters={{"sector": "Technology"}}  # Filter to technology sector
    )
    
    # Get hourly data for short-term trend
    bar_data_1h = get_bar_data(
        timeframe="1h",
        columns=["ticker", "timestamp", "close"],
        min_bars=5,   # Need 5 hours for short-term moving average
        filters={{"sector": "Technology"}}
    )
    
    if bar_data_1d is None or len(bar_data_1d) == 0 or bar_data_1h is None or len(bar_data_1h) == 0:
        return instances
    
    df_1d = pd.DataFrame(bar_data_1d, columns=["ticker", "timestamp", "close"])
    df_1h = pd.DataFrame(bar_data_1h, columns=["ticker", "timestamp", "close"])
    
    # Calculate RSI for each ticker
    def calculate_rsi(prices, period=14):
        delta = prices.diff()
        gain = (delta.where(delta > 0, 0)).rolling(window=period).mean()
        loss = (-delta.where(delta < 0, 0)).rolling(window=period).mean()
        rs = gain / loss
        return 100 - (100 / (1 + rs))
    
    common_tickers = set(df_1d['ticker']).intersection(set(df_1h['ticker']))
    
    for ticker in common_tickers:
        ticker_1d = df_1d[df_1d['ticker'] == ticker].sort_values('timestamp')
        ticker_1h = df_1h[df_1h['ticker'] == ticker].sort_values('timestamp')
        
        if len(ticker_1d) < 15 or len(ticker_1h) < 5:
            continue
        
        # Calculate daily RSI
        ticker_1d['rsi'] = calculate_rsi(ticker_1d['close'])
        latest_1d = ticker_1d.iloc[-1]
        
        # Calculate hourly trend (5-hour SMA)
        ticker_1h['sma_5'] = ticker_1h['close'].rolling(5).mean()
        latest_1h = ticker_1h.iloc[-1]
        trend_strength = (latest_1h['close'] / latest_1h['sma_5']) - 1
        
        # Strategy trigger: RSI oversold + positive hourly trend
        if latest_1d['rsi'] < 30 and trend_strength > 0.02:  # RSI < 30 + 2%+ above hourly SMA
            instances.append({{
                'ticker': ticker,
                'timestamp': int(latest_1d['timestamp']),
                'entry_price': float(latest_1d['close']),
                # CRITICAL: Include ALL calculated indicators - essential for analysis
                'rsi': round(float(latest_1d['rsi']), 2),
                'trend_strength_1h': round(float(trend_strength), 3),
                'sma_5_1h': round(float(latest_1h['sma_5']), 2),
                'rsi_oversold_depth': round(float(30 - latest_1d['rsi']), 2),
                'score': round(min(1.0, (30 - latest_1d['rsi']) / 20 + trend_strength), 3)
            }})
    
    return instances

# Example 3: Gap Strategy with Specific Tickers - demonstrates ticker filtering and gap calculation
def strategy():
    instances = []
    
    target_tickers = ["AAPL", "TSLA", "NVDA"]  # Specific tickers from prompt
    
    bar_data = get_bar_data(
        timeframe="1d",
        columns=["ticker", "timestamp", "open", "close"],
        min_bars=2,  # Need 2 bars: previous close + current open
        filters={{"tickers": target_tickers}}
    )
    
    if bar_data is None or len(bar_data) == 0:
        return instances
    
    df = pd.DataFrame(bar_data, columns=["ticker", "timestamp", "open", "close"])
    df = df.sort_values(['ticker', 'timestamp']).reset_index(drop=True)
    
    # Calculate gaps: compare current open vs previous close
    df['prev_close'] = df.groupby('ticker')['close'].shift(1)
    df = df.dropna()  # Remove rows without previous close
    df['gap_percent'] = ((df['open'] - df['prev_close']) / df['prev_close']) * 100
    
    # CRITICAL: Ensure numeric dtype for calculations
    df['gap_percent'] = pd.to_numeric(df['gap_percent'], errors='coerce')
    df = df.dropna(subset=['gap_percent'])
    
    # Find significant gaps (3%+ up or down)
    significant_gaps = df[abs(df['gap_percent']) >= 3.0]
    
    for _, row in significant_gaps.iterrows():
        instances.append({{
            'ticker': row['ticker'],
            'timestamp': int(row['timestamp']),
            'entry_price': float(row['open']),
            # CRITICAL: Include calculated values that define the signal
            'gap_percent': round(float(row['gap_percent']), 3),
            'prev_close': round(float(row['prev_close']), 2),
            'gap_direction': 'up' if row['gap_percent'] > 0 else 'down',
            'gap_magnitude': round(float(abs(row['gap_percent'])), 3),
            'score': round(min(1.0, abs(row['gap_percent']) / 10.0), 3)  # Normalize by 10%
        }})
    
    return instances
```

COMMON MISTAKES TO AVOID:
- qualifying_instances = df[condition].groupby('ticker').tail(1) - limits to 1 per ticker
- latest_df = df.groupby('ticker').last() - only latest data
- df.drop_duplicates(subset=['ticker']) - this removes valid instances
- 'signal': True - unnecessary field, if returned it inherently met criteria
- No 'score' field - score is required for ranking
- aggregate_mode=True for individual stock patterns - use only for market-wide calculations
- using TICKER-0 in instead of TICKER - ignore user input in this format and use actual ticker
- Any value you attach to a dict, list, or Plotly trace must already be JSON-serialisable — so cast NumPy scalars to plain int/float/bool, turn any date-time object (np.datetime64, pd.Timestamp, datetime)
into an ISO-8601 string (or Unix-seconds int), replace NaN/NA with None, and flatten arrays/Series to plain Python lists before you return or plot them.

✅ qualifying_instances = df[condition]  # CORRECT - returns all matching instances
✅ qualifying_instances = df[df['gap_percent'] >= threshold]  # CORRECT - all qualifying rows
✅ Include 'entry_price', 'gap_percent', etc.  # CORRECT - meaningful data
✅ 'score': min(1.0, instance_strength / max_strength)  # CORRECT - normalized score
✅ aggregate_mode=True ONLY for market averages/correlations  # CORRECT - when you need ALL data

PATTERN RECOGNITION:
- Gap patterns: Compare open vs previous close - return ALL gaps in timeframe
  min_bars=2 (need current + previous), Score: min(1.0, gap_percent / 10.0)
- Volume patterns: Compare current vs historical average - return ALL volume spikes  
  min_bars=1 for simple threshold, min_bars=20+ for rolling average
  Score: min(1.0, (volume_ratio - 1.0) / 4.0) - higher volume = higher score
- Price patterns: Use moving averages, RSI - return ALL qualifying instances
  min_bars=20+ for indicators, Score: Based on instance strength (RSI distance from 50, etc.)
- Breakout patterns: Identify price breakouts - return ALL breakouts
  min_bars=2+ for comparison, Score: min(1.0, breakout_strength / max_expected)
- Fundamental patterns: Use market cap, sector data - return ALL qualifying companies
  min_bars=1 (current data only), Score: Based on fundamental strength

TICKER EXTRACTION FROM PROMPTS:
- If prompt mentions specific ticker (e.g., "MRNA gaps up"), use filters={{"tickers": ["MRNA"]}}
- If prompt mentions "stocks" or "companies" generally, use filters={{}} or sector filters
- Common ticker patterns: AAPL, TSLA, AMZN, GOOGL, MSFT, NVDA

SECURITY RULES:
- Only use whitelisted imports
- CRITICAL: DO NOT use math.fabs() - use the built-in abs() function instead.
- No file operations, network access, or dangerous functions
- No exec, eval, or dynamic code execution
- Use only standard mathematical and data manipulation operations

DATA VALIDATION:
- Always validate DataFrame columns exist before using them
- Check for None/empty data at every step
- Use proper data type conversions (int, float, str)
- Handle edge cases like division by zero

**VERY IMPORTANT**: 
BE EXTREMELY CAREFUL WITH stop loss prices. The next day might gap above/below the stop loss price, which results in a loss greater than the stop. 
Ensure THIS IS PROPERLY ACCOUNTED FOR. You need to get the first price after the stop is triggered. IF THE STOP IS TRIGGERED, YOU SHOULD MAKE SURE 
THAT THE STOCK DID NOT GAP PAST THE STOP LOSS PRICE. IF IT DID, YOU SHOULD USE THE NEXT DAY'S OPEN PRICE AS THE STOP LOSS PRICE.


PRINTING DATA (REQUIRED): 
- Use print() to print useful data for the user
- This should include things like but not limited to:number of instances, averages, medians, standard deviations, and other nuanced or unusual or interesting metrics.

PLOTLY PLOT GENERATION:
- Use plotly to generate plots of useful visualizations of the data
- Histograms of performance metrics, returns, etc 
- Always show the plot using .show()
- Almost always include plots in the strategy to help the user understand the data
- ENSURE ALL (x,y,z) data is JSON serialisable. NEVER use pandas/numpy types (datetime64, int64, float64, timestamp), they cause JSON serialization errors

RETURN FORMAT:
- *ALWAYS* Return List[Dict] where each dict contains:
  * 'ticker': str (e.g., "MRNA", "AAPL")
  * 'timestamp': int (Unix timestamp)
  * 'entry_price': float (price at instance time - open, close, etc.)
  * 'score': float (REQUIRED, 0.0 to 1.0, higher = stronger instance. Rounded to 3 decimal places)
  * Additional fields as needed for strategy results (gap_percent, volume_ratio, etc. Rounded to 3 decimal places)
- CRITICAL JSON SAFETY: ALL values must be native Python types (int, float, str, bool)
- NEVER return pandas/numpy types (datetime64, int64, float64) - they cause JSON serialization errors
- DO NOT include 'signal': True - it's redundant
- ENSURE YOU RETURN THE TRADES/INSTANCES. Do not omit. 

Generate clean, robust Python code that returns ALL matching instances and lets the execution engine handle mode-specific filtering."""
    
    async def create_strategy_from_prompt(self, user_id: int, prompt: str, strategy_id: int = -1) -> Dict[str, Any]:
        """Create or edit a strategy from natural language prompt"""
        try:
            logger.info(f"Creating strategy for user {user_id}, prompt: {prompt[:100]}...")
            
            # Check if this is an edit operation
            is_edit = strategy_id != -1
            existing_strategy = None
            
            if is_edit:
                existing_strategy = await self._fetch_existing_strategy(user_id, strategy_id)
                if not existing_strategy:
                    return {
                        "success": False,
                        "error": f"Strategy {strategy_id} not found for user {user_id}"
                    }
            
            # Generate strategy code with retry logic
            logger.info("Generating strategy code with OpenAI o3...")
            strategy_code, validation_passed = await self._generate_and_validate_strategy(user_id, prompt, existing_strategy, max_retries=2)
            
            if not strategy_code:
                return {
                    "success": False,
                    "error": "Failed to generate valid strategy code after retries"
                }
            
            # Extract description and generate name
            description = self._extract_description(strategy_code, prompt)
            name = self._generate_strategy_name(prompt, is_edit, existing_strategy)
            
            # Save strategy to database
            logger.info("Saving strategy to database...")
            saved_strategy = await self._save_strategy(
                user_id=user_id,
                name=name,
                description=description,
                prompt=prompt,
                python_code=strategy_code,
                strategy_id=strategy_id if is_edit else None
            )
            
            logger.info(f"Strategy {'updated' if is_edit else 'created'} successfully: ID {saved_strategy['strategyId']}")
            
            return {
                "success": True,
                "strategy": saved_strategy,
                "validation_passed": validation_passed
            }
            
        except Exception as e:
            logger.error(f"Strategy creation failed: {e}")
            return {
                "success": False,
                "error": str(e)
            }
    
    async def _generate_and_validate_strategy(self, userID: int, prompt: str, existing_strategy: Optional[Dict[str, Any]] = None, max_retries: int = 2) -> tuple[str, bool]:
        """Generate strategy with validation retry logic"""
        
        last_validation_error = None
        
        for attempt in range(max_retries + 1):
            try:
                logger.info(f"Generation attempt {attempt + 1}/{max_retries + 1}")
                
                # Generate strategy code with error context for retries
                strategy_code = self._generate_strategy_code(userID, prompt, existing_strategy, attempt, last_validation_error)
                
                if not strategy_code:
                    continue
                
                # Validate the generated code (this IS async)
                validation_result = await self._validate_strategy_code(strategy_code)
                
                if validation_result["valid"]:
                    logger.info("Strategy validation passed")
                    return strategy_code, True
                else:
                    last_validation_error = validation_result['error']
                    logger.warning(f"Validation failed on attempt {attempt + 1}: {validation_result['error']}")
                    if attempt == max_retries:
                        # Return the last generated code even if validation failed
                        logger.warning("Max retries reached, returning last generated code")
                        return strategy_code, False
                    
            except Exception as e:
                logger.error(f"Generation attempt {attempt + 1} failed: {e}")
                if attempt == max_retries:
                    break
        
        return "", False
    
    async def _fetch_existing_strategy(self, user_id: int, strategy_id: int) -> Optional[Dict[str, Any]]:
        """Fetch existing strategy for editing"""
        conn = None
        cursor = None
        try:
            logger.info(f"📖 Fetching existing strategy (user_id: {user_id}, strategy_id: {strategy_id})")
            
            db_config = {
                'host': os.getenv('DB_HOST', 'localhost'),
                'port': os.getenv('DB_PORT', '5432'),
                'user': os.getenv('DB_USER', 'postgres'),
                'password': os.getenv('DB_PASSWORD', ''),
                'database': os.getenv('POSTGRES_DB', 'postgres'),
                'connect_timeout': 30
            }
            
            conn = psycopg2.connect(**db_config)
            cursor = conn.cursor(cursor_factory=RealDictCursor)
            
            cursor.execute("""
                SELECT strategyid, name, description, prompt, pythoncode
                FROM strategies 
                WHERE strategyid = %s AND userid = %s
            """, (strategy_id, user_id))
            
            result = cursor.fetchone()
            
            if result:
                logger.info(f"✅ Found existing strategy: {result['name']}")
                return {
                    'strategyId': result['strategyid'],
                    'name': result['name'],
                    'description': result['description'] or '',
                    'prompt': result['prompt'] or '',
                    'pythonCode': result['pythoncode'] or ''
                }
            else:
                logger.warning(f"⚠️ No strategy found for user_id {user_id}, strategy_id {strategy_id}")
                return None
            
        except Exception as e:
            logger.error(f"❌ Failed to fetch existing strategy: {e}")
            logger.error(f"📄 Fetch strategy traceback: {traceback.format_exc()}")
            return None
        finally:
            # Ensure connections are always closed
            try:
                if cursor:
                    cursor.close()
                    logger.debug("🔌 Database cursor closed")
                if conn:
                    conn.close()
                    logger.debug("🔌 Database connection closed")
            except Exception as cleanup_error:
                logger.warning(f"⚠️ Error during database cleanup: {cleanup_error}")
    
    def _generate_strategy_code(self, userID: int, prompt: str, existing_strategy: Optional[Dict[str, Any]] = None, attempt: int = 0, last_error: Optional[str] = None) -> str:
        """
        Generate strategy code using OpenAI with optimized prompts
        
        IMPORTANT: The system instruction emphasizes returning ALL matching instances,
        not just the latest per ticker. This prevents the .tail(1) bug that was
        limiting backtest results to only one instance per symbol.
        """
        try:
            system_instruction = self._get_system_instruction()
            
            # Create concise user prompt based on context
            if existing_strategy:
                # For editing - keep it minimal
                user_prompt = f"""EDIT REQUEST: {prompt}

                CURRENT STRATEGY: {existing_strategy.get('name', 'Strategy')}
                {self._get_code_summary(existing_strategy.get('pythonCode', ''))}

            Generate the updated strategy function."""
            else:

                user_prompt = f"""CREATE STRATEGY: {prompt}"""

            
            # Add retry-specific guidance with error context
            if attempt > 0:
                user_prompt += f"\n\nIMPORTANT - RETRY ATTEMPT {attempt + 1}:"
                user_prompt += f"\n- Previous attempt failed validation"
                if last_error:
                    user_prompt += f"\n- SPECIFIC ERROR: {last_error}"
                user_prompt += f"\n- Focus on data type safety for pandas operations"
                user_prompt += f"\n- Use pd.to_numeric() before .quantile() operations"
                user_prompt += f"\n- Handle NaN values with .dropna() before statistical operations"
                user_prompt += f"\n- Ensure proper error handling for edge cases"
            
            # Use only o3 model as requested
            models_to_try = [
                ("o4-mini", None),                # o3 model only
            ]
            
            last_error = None
            
            for model_name, max_tokens in models_to_try:
                try:
                    logger.info(f"Attempting generation with model: {model_name}")
                    
                    logger.info(f"🕐 Starting OpenAI API call with model {model_name} (timeout: 120s)")
                    
                    response = self.openai_client.responses.create(
                        model=model_name,
                        input=f"{user_prompt}",
                        instructions=f"{system_instruction}",
                        user=f"user:{userID}",
                        timeout=120.0  # 2 minute timeout for other models
                    )
                    
                    strategy_code = response.output_text
                    # Extract Python code from response
                    strategy_code = self._extract_python_code(strategy_code)
                    
                    logger.info(f"Generated strategy code with {model_name} ({len(strategy_code)} characters)")
                    return strategy_code
                    
                except Exception as e:
                    last_error = e
                    logger.warning(f"Model {model_name} failed: {e}")
                    continue
            
            # If all models failed, raise the last error
            raise last_error if last_error else Exception("All models failed")
            
        except Exception as e:
            logger.error(f"OpenAI code generation failed: {e}")
            return ""
    
    def _get_code_summary(self, code: str) -> str:
        """Get a brief summary of existing code to reduce token usage"""
        if not code:
            return "No existing code"
        
        # Extract just the function signature and first few lines
        lines = code.split('\n')
        summary_lines = []
        
        for line in lines[:15]:  # First 15 lines max
            if line.strip():
                summary_lines.append(line)
        
        if len(lines) > 15:
            summary_lines.append("... (code continues)")
        
        return '\n'.join(summary_lines)
    
    def _extract_python_code(self, response: str) -> str:
        """Extract Python code from response, removing markdown formatting"""
        # Remove markdown code blocks
        code_block_pattern = r'```(?:python)?\s*(.*?)\s*```'
        matches = re.findall(code_block_pattern, response, re.DOTALL)
        
        if matches:
            return matches[0].strip()
        
        # If no code blocks found, return the response as-is
        return response.strip()
    
    async def _validate_strategy_code(self, strategy_code: str) -> Dict[str, Any]:
        """Validate strategy code using the security validator and test execution with comprehensive error handling"""
        try:
            logger.info(f"🔍 Starting validation of strategy code ({len(strategy_code)} characters)")
            
            # Print the entire Python strategy returned by o3 before validation
            print("\n" + "="*80)
            print("📋 O3 RETURNED STRATEGY CODE (BEFORE VALIDATION)")
            print("="*80)
            print(strategy_code)
            print("="*80)
            print("🔍 NOW STARTING VALIDATION PROCESS...")
            print("="*80 + "\n")
            
            # First, use the existing validator for security checks
            logger.info("🛡️ Running security validation...")
            try:
                is_valid = self.validator.validate_code(strategy_code)
                logger.info(f"🛡️ Security validation result: {is_valid}")
            except Exception as security_error:
                logger.error(f"🚨 Security validation crashed: {security_error}")
                logger.error(f"📄 Security validation traceback: {traceback.format_exc()}")
                return {
                    "valid": False,
                    "error": f"Security validation crashed: {str(security_error)}"
                }
            
            if not is_valid:
                logger.warning("❌ Security validation failed")
                return {
                    "valid": False,
                    "error": "Security validation failed"
                }
            
            logger.info("✅ Security validation passed")
            
            # Try a quick execution test with the new accessor engine
            logger.info("🧪 Running execution test...")
            try:
                # Use fast validation mode with minimal data and short timeout
                engine = AccessorStrategyEngine()
                test_result = await asyncio.wait_for(
                    engine.execute_validation(
                        strategy_code=strategy_code
                    ),
                    timeout=15.0  # 15 second timeout for fast validation
                )
                
                logger.info(f"🧪 Execution test completed: success={test_result.get('success', False)}")
                
                if test_result.get('success', False):
                    logger.info("✅ Execution test passed")
                    return {
                        "valid": True,
                        "error": None
                    }
                else:
                    logger.warning(f"❌ Execution test failed: {test_result.get('error', 'Unknown error')}")
                    return {
                        "valid": False,
                        "error": f"Execution test failed: {test_result.get('error', 'Unknown error')}"
                    }
                    
            except asyncio.TimeoutError:
                logger.warning("⏰ Fast validation timed out after 15 seconds")
                # Timeout in validation mode suggests serious performance issues
                return {
                    "valid": False,
                    "error": "Validation timeout - strategy may have infinite loops or performance issues"
                }
                
            except Exception as exec_error:
                error_msg = str(exec_error)
                logger.warning(f"⚠️ Execution test failed with exception: {exec_error}")
                logger.warning(f"📄 Execution test traceback: {traceback.format_exc()}")
                
                # Classify error types - only allow data-related issues as warnings
                data_related_errors = [
                    "no data", "empty dataset", "missing data", "connection", 
                    "timeout", "network", "database", "redis"
                ]
                
                programming_errors = [
                    "quantile", "dtype", "syntax", "name", "attribute", 
                    "type", "index", "key", "value", "division by zero"
                ]
                
                error_lower = error_msg.lower()
                
                # If it's a clear programming error, mark as invalid for retry
                if any(prog_err in error_lower for prog_err in programming_errors):
                    logger.error(f"🚨 Programming error detected: {error_msg}")
                    return {
                        "valid": False,
                        "error": f"Programming error: {error_msg}"
                    }
                
                # Only allow data-related errors as warnings
                if any(data_err in error_lower for data_err in data_related_errors):
                    logger.info(f"💡 Data-related error (allowing as warning): {error_msg}")
                    return {
                        "valid": True,
                        "error": f"Warning: Data-related issue: {error_msg}"
                    }
                
                # Default: treat unknown errors as programming errors
                logger.error(f"🚨 Unknown error type, treating as programming error: {error_msg}")
                return {
                    "valid": False,
                    "error": f"Programming error: {error_msg}"
                }
            
        except (SecurityError, StrategyComplianceError) as e:
            logger.error(f"🚨 Strategy compliance error: {e}")
            return {
                "valid": False,
                "error": str(e)
            }
        except Exception as e:
            logger.error(f"💥 Unexpected validation error: {e}")
            logger.error(f"📄 Validation error traceback: {traceback.format_exc()}")
            return {
                "valid": False,
                "error": f"Validation failed: {str(e)}"
            }
    
    def _extract_description(self, strategy_code: str, prompt: str) -> str:
        """Extract or generate description from strategy code and prompt"""
        # Try to extract docstring from code
        docstring_pattern = r'"""(.*?)"""'
        matches = re.findall(docstring_pattern, strategy_code, re.DOTALL)
        
        if matches:
            description = matches[0].strip()
            # Clean up the description
            if len(description) > 200:
                description = description[:200] + "..."
            return description
        
        # Fall back to generating description from prompt
        prompt_words = prompt.split()[:15]  # Increased from 10 to 15 words
        return f"Strategy: {' '.join(prompt_words)}{'...' if len(prompt.split()) > 15 else ''}"
    
    def _generate_strategy_name(self, prompt: str, is_edit: bool, existing_strategy: Optional[Dict[str, Any]] = None) -> str:
        """Generate a strategy name"""
        if is_edit and existing_strategy:
            # For edits, keep the original name but add "Updated"
            original_name = existing_strategy.get('name', 'Strategy')
            if "(Updated" not in original_name:
                return f"{original_name} (Updated)"
            return original_name
        
        # For new strategies, generate from prompt
        words = prompt.split()[:5]  # Increased from 4 to 5 words
        clean_words = []
        
        skip_words = {'create', 'a', 'an', 'the', 'strategy', 'for', 'when', 'find', 'identify', 'that', 'this'}
        
        for word in words:
            clean_word = re.sub(r'[^a-zA-Z0-9]', '', word)
            if clean_word.lower() not in skip_words and len(clean_word) > 1:
                clean_words.append(clean_word.title())
        
        if not clean_words:
            clean_words = ['Custom']
        
        # Generate base name and add timestamp to ensure uniqueness
        base_name = f"{' '.join(clean_words)} Strategy"
        timestamp_suffix = datetime.now().strftime("%m%d%H%M")
        return f"{base_name} {timestamp_suffix}"
    
    async def _save_strategy(self, user_id: int, name: str, description: str, prompt: str, 
                           python_code: str, strategy_id: Optional[int] = None) -> Dict[str, Any]:
        """Save strategy to database with duplicate name handling"""
        conn = None
        cursor = None
        try:
            logger.info(f"💾 Saving strategy to database (user_id: {user_id}, strategy_id: {strategy_id})")
            
            db_config = {
                'host': os.getenv('DB_HOST', 'localhost'),
                'port': os.getenv('DB_PORT', '5432'),
                'user': os.getenv('DB_USER', 'postgres'),
                'password': os.getenv('DB_PASSWORD', ''),
                'database': os.getenv('POSTGRES_DB', 'postgres'),
                'connect_timeout': 30  # 30 second connection timeout
            }
            
            conn = psycopg2.connect(**db_config)
            cursor = conn.cursor(cursor_factory=RealDictCursor)
            
            if strategy_id:
                # Update existing strategy
                cursor.execute("""
                    UPDATE strategies 
                    SET name = %s, description = %s, prompt = %s, pythoncode = %s, 
                        updated_at = NOW()
                    WHERE strategyid = %s AND userid = %s
                    RETURNING strategyid, name, description, prompt, pythoncode, 
                             createdat, updated_at, isalertactive
                """, (name, description, prompt, python_code, strategy_id, user_id))
            else:
                # Create new strategy with duplicate name handling
                # First check if name already exists and modify if needed
                original_name = name
                cursor.execute("""
                    SELECT COUNT(*) as count FROM strategies 
                    WHERE userid = %s AND name = %s
                """, (user_id, name))
                count_result = cursor.fetchone()
                
                if count_result and count_result['count'] > 0:
                    # Name exists, add timestamp suffix
                    timestamp_suffix = datetime.now().strftime("%m%d_%H%M%S")
                    name = f"{original_name} ({timestamp_suffix})"
                    logger.info(f"Strategy name conflict detected, using: {name}")
                
                cursor.execute("""
                    INSERT INTO strategies (userid, name, description, prompt, pythoncode, 
                                          createdat, updated_at, isalertactive, score, version)
                    VALUES (%s, %s, %s, %s, %s, NOW(), NOW(), false, 0, '1.0')
                    RETURNING strategyid, name, description, prompt, pythoncode, 
                             createdat, updated_at, isalertactive
                """, (user_id, name, description, prompt, python_code))
            
            result = cursor.fetchone()
            conn.commit()
            
            logger.info(f"✅ Strategy saved successfully with ID: {result['strategyid'] if result else 'None'}")
            
            if result:
                return {
                    'strategyId': result['strategyid'],
                    'userId': user_id,
                    'name': result['name'],
                    'description': result['description'],
                    'prompt': result['prompt'],
                    'pythonCode': result['pythoncode'],
                    'createdAt': result['createdat'].isoformat() if result['createdat'] else None,
                    'updatedAt': result['updated_at'].isoformat() if result['updated_at'] else None,
                    'isAlertActive': result['isalertactive']
                }
            else:
                raise Exception("Failed to save strategy - no result returned")
                
        except Exception as e:
            logger.error(f"❌ Failed to save strategy: {e}")
            logger.error(f"📄 Save strategy traceback: {traceback.format_exc()}")
            raise
        finally:
            # Ensure connections are always closed
            try:
                if cursor:
                    cursor.close()
                    logger.debug("🔌 Database cursor closed")
                if conn:
                    conn.close()
                    logger.debug("🔌 Database connection closed")
            except Exception as cleanup_error:
                logger.warning(f"⚠️ Error during database cleanup: {cleanup_error}") 
