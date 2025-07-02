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

🔍 FUNCTION VALIDATION - ONLY THESE FUNCTIONS EXIST:
✅ get_bar_data(timeframe, columns, min_bars, filters) → numpy.ndarray
✅ get_general_data(columns, filters) → pandas.DataFrame

❌ DO NOT USE - THESE FUNCTIONS DO NOT EXIST:
get_security_details(), get_price_data(), get_fundamental_data(), get_multiple_symbols_data(), etc.

📋 NOTE: get_bar_data() and get_general_data() are automatically available in the execution environment.

CRITICAL REQUIREMENTS:
- Function named 'strategy()' with NO parameters
- Use data accessor functions with filters (NOT deprecated tickers parameter):
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
     
     IMPORTANT: min_bars cannot exceed 10,000 - use minimum needed:
       - 1 bar: Simple current patterns (volume spikes, price thresholds)
       - 2 bars: Patterns using shift() for previous values (gaps, daily changes)
       - 20+ bars: Technical indicators (moving averages, RSI)
  * get_bar_data(timeframe="1d", aggregate_mode=True, filters={{}}) 
     Use aggregate_mode=True ONLY when you need ALL market data together for calculations like market averages
  * get_general_data(columns=[], filters={{"tickers": ["AAPL", "MRNA"]}}) -> pandas DataFrame  
     Columns: ticker, name, sector, industry, market_cap, market, locale, primary_exchange, active, description, cik, total_shares

AVAILABLE FILTERS (use in filters parameter):
- sector: "{sectors_str}"
- industry: "{industries_str}"
- primary_exchange: "{exchanges_str}"
- locale: "{locales_str}" (us=United States, ca=Canada, mx=Mexico)
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

CRITICAL: X-MINUTE TIMEFRAME AND TIME ALIGNMENT:
- X-minute bars may not align exactly with specific times like 15:45, 15:55
- Use time ranges instead of exact matches: (time >= 15:45) & (time <= 15:50) for 15:45-15:50 period
- For Friday afternoon patterns, look for the closest X-minute bars to target times
- Example time range filtering:
  afternoon_bars = df[(df['datetime_et'].dt.time >= time(15, 40)) & (df['datetime_et'].dt.time <= time(16, 0))]

ERROR HANDLING NOTE:
- The strategy executor automatically wraps your strategy function in try-except blocks
- You do NOT need to include try-except in your strategy code
- Focus on the strategy logic - error handling is managed by the system
- If data is invalid, simply return an empty list: return []

EXAMPLE PATTERNS:
```python
def strategy():
    instances = []
    
    # Example 1: Multi-timeframe momentum strategy using 5-minute and 4-hour data
    target_tickers = ["AAPL", "MSFT"]  # Extract from prompt analysis
    
    # Get 5-minute data for short-term momentum (aggregated from 1-minute)
    bars_5m = get_bar_data(
        timeframe="5m",  # Custom aggregation: 5-minute bars from 1-minute data
        columns=["ticker", "timestamp", "open", "high", "low", "close", "volume"],
        min_bars=5,  # Need 5 bars for short-term momentum calculation
        filters={{"tickers": target_tickers}}
    )
    
    # Get 4-hour data for trend confirmation (aggregated from 1-hour)  
    bars_4h = get_bar_data(
        timeframe="4h",  # Custom aggregation: 4-hour bars from 1-hour data
        columns=["ticker", "timestamp", "open", "high", "low", "close", "volume"],
        min_bars=3,  # Need 3 bars for trend analysis (current + 2 previous for moving average)
        filters={{"tickers": target_tickers}}
    )
    
    if bars_5m is None or len(bars_5m) == 0 or bars_4h is None or len(bars_4h) == 0:
        return instances
    
    # Convert to DataFrames for analysis
    df_5m = pd.DataFrame(bars_5m, columns=["ticker", "timestamp", "open", "high", "low", "close", "volume"])
    df_4h = pd.DataFrame(bars_4h, columns=["ticker", "timestamp", "open", "high", "low", "close", "volume"])
    
    if len(df_5m) == 0 or len(df_4h) == 0:
        return instances
    
    # Analyze each ticker separately
    for ticker in target_tickers:
        ticker_5m = df_5m[df_5m['ticker'] == ticker].sort_values('timestamp')
        ticker_4h = df_4h[df_4h['ticker'] == ticker].sort_values('timestamp')
        
        if len(ticker_5m) < 5 or len(ticker_4h) < 3:
            continue
        
        # Calculate 5-minute momentum (RSI-like indicator)
        ticker_5m['price_change'] = ticker_5m['close'].pct_change()
        recent_momentum = ticker_5m['price_change'].tail(5).mean()
        
        # Calculate 4-hour trend (simple moving average)
        ticker_4h['sma_3'] = ticker_4h['close'].rolling(3).mean()
        current_price = ticker_4h['close'].iloc[-1]
        current_sma = ticker_4h['sma_3'].iloc[-1]
        
        # Multi-timeframe strategy trigger: 5m momentum + 4h trend alignment
        if recent_momentum > 0.01 and current_price > current_sma:  # Bullish on both timeframes
            instances.append({{
                'ticker': ticker,
                'timestamp': int(ticker_5m['timestamp'].iloc[-1]),
                'entry_price': float(current_price),
                # CRITICAL: Include ALL calculated indicator values
                'momentum_5m': float(recent_momentum),
                'trend_4h': float(current_price / current_sma - 1),
                'sma_3_4h': float(current_sma),
                'price_change_5m': float(ticker_5m['price_change'].iloc[-1]),
                'score': min(1.0, recent_momentum * 10 + (current_price / current_sma - 1))
            }})
        
    return instances

# Example 2: Volume breakout - return ALL breakouts, not just latest (BATCHED automatically)
def strategy():
    instances = []
    
    bar_data = get_bar_data(
        timeframe="1d",
        columns=["ticker", "timestamp", "volume"],
        min_bars=20,  # Need 20 bars for volume average calculation
        filters={{}}  # All tickers
    )
    
    if bar_data is None or len(bar_data) == 0:
        return instances
    
    df = pd.DataFrame(bar_data, columns=["ticker", "timestamp", "volume"])
    df = df.sort_values(['ticker', 'timestamp']).reset_index(drop=True)
    
    # Calculate 20-day volume average
    df['volume_avg_20'] = df.groupby('ticker')['volume'].rolling(20).mean().reset_index(0, drop=True)
    df['volume_ratio'] = df['volume'] / df['volume_avg_20']
    
    # CORRECT: Find ALL volume breakouts (no .tail(1))
    breakouts = df[df['volume_ratio'] >= 2.0]  # Volume 2x average
    
    for _, row in breakouts.iterrows():
        instances.append({{
            'ticker': row['ticker'],
            'timestamp': int(row['timestamp']),
            'entry_price': float(row.get('close', 0)),  # Use close as entry price
            # CRITICAL: Include important indicator values that triggered the signal
            'volume_ratio': float(row['volume_ratio']),
            'volume': int(row['volume']),
            'volume_avg_20': int(row['volume_avg_20']),
            'volume_breakout_strength': float(row['volume_ratio'] - 2.0),  # How much above threshold
            'score': min(1.0, (row['volume_ratio'] - 2.0) / 3.0)  # Higher ratio = higher score
        }})
        
    return instances

# Example 3: Sector-specific strategy using filters
def strategy():
    instances = []
    
    # Focus on technology stocks only
    bar_data = get_bar_data(
        timeframe="1d",
        columns=["ticker", "timestamp", "open", "close", "volume"],
        min_bars=5,  # Need 5 bars to calculate 5-day price change
        filters={{"sector": "Technology"}}  # Filter to Technology sector only
    )
    
    if bar_data is None or len(bar_data) == 0:
        return instances
    
    df = pd.DataFrame(bar_data, columns=["ticker", "timestamp", "open", "close", "volume"])
    df = df.sort_values(['ticker', 'timestamp']).reset_index(drop=True)
    
    # Calculate 5-day price change for technology stocks
    df['prev_close_5d'] = df.groupby('ticker')['close'].shift(5)
    df = df.dropna()
    df['change_5d_pct'] = ((df['close'] - df['prev_close_5d']) / df['prev_close_5d']) * 100
    
    # CRITICAL: Ensure numeric dtype for statistical operations
    df['change_5d_pct'] = pd.to_numeric(df['change_5d_pct'], errors='coerce')
    df = df.dropna(subset=['change_5d_pct'])
    
    # Find technology stocks with significant moves
    qualifying_instances = df[df['change_5d_pct'] >= 10.0]  # 10%+ gain over 5 days
    
    for _, row in qualifying_instances.iterrows():
        instances.append({{
            'ticker': row['ticker'],
            'timestamp': int(row['timestamp']),
            'change_5d_pct': float(row['change_5d_pct']),
            'entry_price': float(row['close']),
            'score': min(1.0, row['change_5d_pct'] / 20.0)  # Normalized score
        }})
        
    return instances

# Example 4: RSI + MACD Strategy - DEMONSTRATES PROPER INDICATOR INCLUSION
def strategy():
    instances = []
    
    bar_data = get_bar_data(
        timeframe="1d",
        columns=["ticker", "timestamp", "close"],
        min_bars=26,  # Need exactly 26 bars for MACD calculation (slow EMA period)
        filters={{"market_cap_min": 1000000000}}  # Large cap only
    )
    
    if bar_data is None or len(bar_data) == 0:
        return instances
    
    df = pd.DataFrame(bar_data, columns=["ticker", "timestamp", "close"])
    df = df.sort_values(['ticker', 'timestamp']).reset_index(drop=True)
    
    # Calculate RSI (14-period)
    def calculate_rsi(prices, period=14):
        delta = prices.diff()
        gain = (delta.where(delta > 0, 0)).rolling(window=period).mean()
        loss = (-delta.where(delta < 0, 0)).rolling(window=period).mean()
        rs = gain / loss
        return 100 - (100 / (1 + rs))
    
    # Calculate MACD
    def calculate_macd(prices):
        ema_12 = prices.ewm(span=12).mean()
        ema_26 = prices.ewm(span=26).mean()
        macd_line = ema_12 - ema_26
        signal_line = macd_line.ewm(span=9).mean()
        histogram = macd_line - signal_line
        return macd_line, signal_line, histogram, ema_12, ema_26
        
        # Apply calculations per ticker
        for ticker in df['ticker'].unique():
            ticker_data = df[df['ticker'] == ticker].copy()
            if len(ticker_data) < 26:
                continue
                
            # Calculate indicators
            ticker_data['rsi'] = calculate_rsi(ticker_data['close'])
            macd, signal, histogram, ema_12, ema_26 = calculate_macd(ticker_data['close'])
            ticker_data['macd'] = macd
            ticker_data['macd_signal'] = signal
            ticker_data['macd_histogram'] = histogram
            ticker_data['ema_12'] = ema_12
            ticker_data['ema_26'] = ema_26
            
            # Strategy trigger: RSI oversold + MACD bullish crossover
            latest = ticker_data.iloc[-1]
            prev = ticker_data.iloc[-2]
            
            if (latest['rsi'] < 30 and  # RSI oversold
                latest['macd'] > latest['macd_signal'] and  # MACD above signal
                prev['macd'] <= prev['macd_signal']):  # Bullish crossover
                
                instances.append({{
                    'ticker': ticker,
                    'timestamp': int(latest['timestamp']),
                    'entry_price': float(latest['close']),
                    # CRITICAL: Include ALL calculated indicators - this is the key!
                    'rsi': round(float(latest['rsi']), 2),
                    'macd': round(float(latest['macd']), 4),
                    'macd_signal': round(float(latest['macd_signal']), 4),
                    'macd_histogram': round(float(latest['macd_histogram']), 4),
                    'ema_12': round(float(latest['ema_12']), 2),
                    'ema_26': round(float(latest['ema_26']), 2),
                    'macd_crossover_strength': float(latest['macd'] - latest['macd_signal']),
                    'score': min(1.0, (30 - latest['rsi']) / 20 + abs(latest['macd_histogram']) * 10)
                }})
    
    return instances

# Example 5: Scalping strategy using 1-minute and 5-minute timeframes
def strategy():
    instances = []
    
    # High-frequency scalping using minute-level data
    target_tickers = ["AAPL", "TSLA", "NVDA"]  # High-volume stocks for scalping
    
    # Get 1-minute data for entry detection
    bars_1m = get_bar_data(
        timeframe="1m",  # Direct 1-minute table access (fastest)
        columns=["ticker", "timestamp", "open", "high", "low", "close", "volume"],
        min_bars=5,  # Need 5 minutes for momentum calculation
        filters={{"tickers": target_tickers}}
    )
    
    # Get 5-minute data for trend confirmation  
    bars_5m = get_bar_data(
        timeframe="5m",  # Aggregated from 1-minute data
        columns=["ticker", "timestamp", "close"],
        min_bars=2,   # Need 2 periods for micro-trend (current vs previous)
        filters={{"tickers": target_tickers}}
    )
    
    if bars_1m is None or len(bars_1m) == 0 or bars_5m is None or len(bars_5m) == 0:
        return instances
    
    df_1m = pd.DataFrame(bars_1m, columns=["ticker", "timestamp", "open", "high", "low", "close", "volume"])
    df_5m = pd.DataFrame(bars_5m, columns=["ticker", "timestamp", "close"])
    
    for ticker in target_tickers:
        ticker_1m = df_1m[df_1m['ticker'] == ticker].sort_values('timestamp').tail(5)
        ticker_5m = df_5m[df_5m['ticker'] == ticker].sort_values('timestamp')
        
        if len(ticker_1m) < 3 or len(ticker_5m) < 2:
            continue
        
        # 1-minute momentum: recent price action
        recent_high = ticker_1m['high'].max()
        current_price = ticker_1m['close'].iloc[-1]
        momentum = (current_price / recent_high) - 1
        
        # 5-minute trend confirmation
        trend_up = ticker_5m['close'].iloc[-1] > ticker_5m['close'].iloc[-2]
        
        # Volume spike detection (1-minute)
        avg_volume = ticker_1m['volume'].mean()
        current_volume = ticker_1m['volume'].iloc[-1]
        volume_spike = current_volume > avg_volume * 1.5
        
        # Scalping strategy trigger: momentum + trend + volume
        if momentum > 0.005 and trend_up and volume_spike:  # 0.5%+ momentum with confirmations
            instances.append({{
                'ticker': ticker,
                'timestamp': int(ticker_1m['timestamp'].iloc[-1]),
                'entry_price': float(current_price),
                'momentum_1m': float(momentum),
                'volume_ratio': float(current_volume / avg_volume),
                'score': min(1.0, momentum * 100 + (current_volume / avg_volume - 1))
            }})
    
    return instances

# Example 5: Multi-timeframe swing trading (4-hour and daily)
def strategy():
    instances = []
    
    try:
        # Swing trading using 4-hour and daily timeframes
        bar_data_4h = get_bar_data(
            timeframe="4h",  # Custom aggregation from 1-hour data
            columns=["ticker", "timestamp", "open", "high", "low", "close", "volume"],
            min_bars=14,     # 14 periods for RSI calculation
            filters={{"sector": "Technology", "market_cap_min": 1000000000}}  # Large-cap tech
        )
        
        bar_data_1d = get_bar_data(
            timeframe="1d",  # Daily data for longer-term trend
            columns=["ticker", "timestamp", "close"],
            min_bars=20,     # 20 days for moving average
            filters={{"sector": "Technology", "market_cap_min": 1000000000}}
        )
        
        if bar_data_4h is None or len(bar_data_4h) == 0 or bar_data_1d is None or len(bar_data_1d) == 0:
            return instances
        
        df_4h = pd.DataFrame(bar_data_4h, columns=["ticker", "timestamp", "open", "high", "low", "close", "volume"])
        df_1d = pd.DataFrame(bar_data_1d, columns=["ticker", "timestamp", "close"])
        
        # Get unique tickers from both datasets
        common_tickers = set(df_4h['ticker']).intersection(set(df_1d['ticker']))
        
        for ticker in common_tickers:
            ticker_4h = df_4h[df_4h['ticker'] == ticker].sort_values('timestamp').tail(14)
            ticker_1d = df_1d[df_1d['ticker'] == ticker].sort_values('timestamp').tail(20)
            
            if len(ticker_4h) < 10 or len(ticker_1d) < 15:
                continue
            
            # Calculate 4-hour RSI (simplified)
            price_changes = ticker_4h['close'].pct_change().dropna()
            gains = price_changes.where(price_changes > 0, 0)
            losses = -price_changes.where(price_changes < 0, 0)
            avg_gain = gains.rolling(14).mean().iloc[-1]
            avg_loss = losses.rolling(14).mean().iloc[-1]
            rs = avg_gain / avg_loss if avg_loss > 0 else 100
            rsi_4h = 100 - (100 / (1 + rs))
            
            # Calculate daily trend (20-day SMA)
            sma_20 = ticker_1d['close'].rolling(20).mean().iloc[-1]
            current_price = ticker_1d['close'].iloc[-1]
            daily_trend = (current_price / sma_20) - 1
            
            # Swing trading strategy: oversold on 4h + uptrend on daily
            if rsi_4h < 35 and daily_trend > 0.05:  # RSI oversold + 5%+ above SMA
                instances.append({{
                    'ticker': ticker,
                    'timestamp': int(ticker_4h['timestamp'].iloc[-1]),
                    'entry_price': float(current_price),
                    'rsi_4h': float(rsi_4h),
                    'daily_trend': float(daily_trend),
                    'score': min(1.0, (35 - rsi_4h) / 35 + daily_trend)
                }})
        
    return instances

# Example 6: Weekly position trading with 2-week confirmation
def strategy():
    instances = []
    
        # Long-term position trading using weekly timeframes
        bars_1w = get_bar_data(
            timeframe="1w",  # Direct weekly table access
            columns=["ticker", "timestamp", "open", "high", "low", "close", "volume"],
            min_bars=20,     # Need 20 weeks for high calculation and volume average
            filters={{"market_cap_min": 10000000000}}  # Large-cap stocks only
        )
        
        bars_2w = get_bar_data(
            timeframe="2w",  # Custom aggregation from weekly data
            columns=["ticker", "timestamp", "close"],
            min_bars=3,      # Need 3 bi-weekly periods for trend calculation (current vs 2 periods ago)
            filters={{"market_cap_min": 10000000000}}
        )
        
        if bars_1w is None or len(bars_1w) == 0 or bars_2w is None or len(bars_2w) == 0:
            return instances
        
        df_1w = pd.DataFrame(bars_1w, columns=["ticker", "timestamp", "open", "high", "low", "close", "volume"])
        df_2w = pd.DataFrame(bars_2w, columns=["ticker", "timestamp", "close"])
        
        common_tickers = set(df_1w['ticker']).intersection(set(df_2w['ticker']))
        
        for ticker in common_tickers:
            ticker_1w = df_1w[df_1w['ticker'] == ticker].sort_values('timestamp').tail(20)
            ticker_2w = df_2w[df_2w['ticker'] == ticker].sort_values('timestamp').tail(3)
            
            if len(ticker_1w) < 15 or len(ticker_2w) < 3:
                continue
            
            # Weekly breakout detection
            recent_high_20w = ticker_1w['high'].max()
            current_price = ticker_1w['close'].iloc[-1]
            breakout_strength = (current_price / recent_high_20w) - 1
            
            # 2-week trend confirmation  
            trend_2w = ticker_2w['close'].iloc[-1] / ticker_2w['close'].iloc[-3] - 1
            
            # Volume confirmation
            avg_volume = ticker_1w['volume'].mean()
            recent_volume = ticker_1w['volume'].iloc[-1]
            volume_surge = recent_volume > avg_volume * 1.2
            
            # Position strategy: breakout + trend + volume
            if breakout_strength > 0.02 and trend_2w > 0.1 and volume_surge:
                instances.append({{
                    'ticker': ticker,
                    'timestamp': int(ticker_1w['timestamp'].iloc[-1]),
                    'entry_price': float(current_price),
                    'breakout_strength': float(breakout_strength),
                    'trend_2w': float(trend_2w),
                    'volume_ratio': float(recent_volume / avg_volume),
                    'score': min(1.0, breakout_strength * 10 + trend_2w + (recent_volume / avg_volume - 1))
                }})
        
    return instances
```

COMMON MISTAKES TO AVOID:
❌ qualifying_instances = df[condition].groupby('ticker').tail(1)  # WRONG - limits to 1 per ticker
❌ latest_df = df.groupby('ticker').last()  # WRONG - only latest data
❌ df.drop_duplicates(subset=['ticker'])  # WRONG - removes valid instances
❌ 'signal': True  # WRONG - unnecessary field, if returned it inherently met criteria
❌ No 'score' field  # WRONG - score is required for ranking
❌ aggregate_mode=True for individual stock patterns  # WRONG - use only for market-wide calculations

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
- Common ticker patterns: AAPL, MRNA, TSLA, AMZN, GOOGL, MSFT, NVDA

SECURITY RULES:
- Only use whitelisted imports: pandas, numpy, datetime, math
- CRITICAL: DO NOT use math.fabs() - use the built-in abs() function instead.
- No file operations, network access, or dangerous functions
- No exec, eval, or dynamic code execution
- Use only standard mathematical and data manipulation operations

DATA VALIDATION:
- Always validate DataFrame columns exist before using them
- Check for None/empty data at every step
- Use proper data type conversions (int, float, str)
- Handle edge cases like division by zero

RETURN FORMAT:
- Return List[Dict] where each dict contains:
  * 'ticker': str (e.g., "MRNA", "AAPL")
  * 'timestamp': int (Unix timestamp)
  * 'entry_price': float (price at instance time - open, close, etc.)
  * 'score': float (REQUIRED, 0.0 to 1.0, higher = stronger instance)
  * Additional fields as needed for strategy results (gap_percent, volume_ratio, etc.)
- DO NOT include 'signal': True - it's redundant

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
                # For new strategy - extract tickers and enhance prompt
                extracted_tickers = self._extract_tickers_from_prompt(prompt)
                ticker_context = ""
                if extracted_tickers:
                    ticker_context = f"\n\nDETECTED TICKERS: {extracted_tickers} - Use filters={{'tickers': {extracted_tickers}}} in your data accessor calls."
                
                user_prompt = f"""CREATE STRATEGY: {prompt}{ticker_context}"""

            
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
                ("o3", None),                # o3 model only
            ]
            
            last_error = None
            
            for model_name, max_tokens in models_to_try:
                try:
                    logger.info(f"Attempting generation with model: {model_name}")
                    
                    # Adjust parameters for o3 models (similar to o1, they don't support temperature/max_tokens the same way)
                    if model_name.startswith("o3"):
                        # Set a timeout for the OpenAI API call to prevent hanging
                        logger.info(f"🕐 Starting OpenAI API call with model {model_name} (timeout: 180s)")
                        
                        # Use the timeout parameter for OpenAI API calls
                        response = self.openai_client.responses.create(
                            model=model_name,
                            input=f"{user_prompt}",
                            instructions=f"{system_instruction}",
                            user=f"user:{userID}",
                            timeout=180.0  # 3 minute timeout for o3 model
                        )
                    else:
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

    def _extract_tickers_from_prompt(self, prompt: str) -> List[str]:
        """Extract ticker symbols from user prompt"""
        import re
        
        # Common ticker patterns - look for 2-5 uppercase letters
        ticker_pattern = r'\b[A-Z]{2,5}\b'
        potential_tickers = re.findall(ticker_pattern, prompt.upper())
        
        # Known common tickers for validation
        known_tickers = {
            'AAPL', 'MSFT', 'GOOGL', 'GOOG', 'AMZN', 'TSLA', 'META', 'NVDA', 
            'MRNA', 'PFIZER', 'JNJ', 'V', 'JPM', 'UNH', 'HD', 'PG', 'MA', 'DIS',
            'ADBE', 'PYPL', 'INTC', 'CRM', 'VZ', 'KO', 'NKE', 'T', 'NFLX', 'ABT',
            'XOM', 'TMO', 'CVX', 'ACN', 'COST', 'AVGO', 'TXN', 'LLY', 'WMT', 'DHR',
            'NEE', 'BMY', 'QCOM', 'HON', 'UPS', 'LOW', 'AMD', 'ORCL', 'MDT', 'IBM',
            'LMT', 'AMT', 'SPGI', 'PFE', 'RTX', 'SBUX', 'CAT', 'DE', 'AXP', 'GS',
            'BLK', 'MMM', 'BKNG', 'ADP', 'TJX', 'CHTR', 'VRTX', 'ISRG', 'SYK', 'NOW'
        }
        
        # Filter to known tickers and remove common words that match pattern
        exclude_words = {'STOCK', 'STOCKS', 'TRADE', 'TRADES', 'PRICE', 'MARKET', 'DATA', 'TIME', 'WHEN', 'WHERE', 'WHAT', 'WITH'}
        
        valid_tickers = []
        for ticker in potential_tickers:
            if ticker in known_tickers and ticker not in exclude_words:
                valid_tickers.append(ticker)
        
        return list(set(valid_tickers))  # Remove duplicates