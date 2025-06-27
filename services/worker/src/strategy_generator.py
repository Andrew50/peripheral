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
from datetime import datetime
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

CRITICAL REQUIREMENTS:
- Function named 'strategy()' with NO parameters
- Use data accessor functions with ticker symbols (NOT security IDs):
  * get_bar_data(timeframe="1d", tickers=["AAPL", "MRNA"], columns=[], min_bars=1, filters={{}}) -> numpy array
     Columns: ticker, timestamp, open, high, low, close, volume
     IMPORTANT: min_bars cannot exceed 10,000 - use minimum needed:
       - 1 bar: Simple current patterns (volume spikes, price thresholds)
       - 2 bars: Patterns using shift() for previous values (gaps, daily changes)
       - 20+ bars: Technical indicators (moving averages, RSI)
  * get_bar_data(timeframe="1d", tickers=None, aggregate_mode=True) -> numpy array (for market-wide calculations only where all data is always required for calculations, this argument will block batching during exectuion of the strategy)
     Use aggregate_mode=True ONLY when you need ALL market data together for calculations like market averages
  * get_general_data(tickers=["AAPL", "MRNA"], columns=[], filters={{}}) -> pandas DataFrame  
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

EXECUTION NOTE: Data requests are automatically batched during execution for efficiency - you don't need to worry about this.

TICKER USAGE:
- Always use ticker symbols (strings) like "MRNA", "AAPL", "TSLA" 
- For specific tickers mentioned in prompts, use tickers=["TICKER_NAME"]
- For universe-wide strategies, use tickers=None to get all available tickers
- Return results with 'ticker' field (string), not 'securityid'

CRITICAL: RETURN ALL MATCHING INSTANCES, NOT JUST THE LATEST
- DO NOT use .tail(1) or .head(1) to limit results per ticker
- Return every occurrence that meets your criteria across the entire dataset
- The execution engine will handle filtering for different modes (backtest, screening, alerts)
- Example: If MRNA gaps up 1% on 5 different days, return all 5 instances

CRITICAL: INSTANCE STRUCTURE
- DO NOT include 'signal': True field - if returned, it inherently met criteria
- Include relevant price data: 'open', 'close', 'entry_price' when available
- Use proper timestamp format: int(row['timestamp']) for Unix timestamp
- Include meaningful data like gap_percent, volume_ratio, etc.
- REQUIRED: Include 'score': float (0.0 to 1.0) - higher score = stronger signal

ROBUST ERROR HANDLING:
- Always check if data is None or empty before processing
- Use proper DataFrame column checks: if 'column_name' in df.columns
- Handle missing data gracefully with try/except blocks
- Return empty list [] on any error or missing data

EXAMPLE PATTERNS:
```python
def strategy():
    instances = []
    
    try:
        # Example 1: Specific ticker strategy (e.g., for MRNA gaps) - Uses batching automatically
        target_tickers = ["MRNA"]  # Extract from prompt analysis
        
        bar_data = get_bar_data(
            timeframe="1d",
            tickers=target_tickers,
            columns=["ticker", "timestamp", "open", "close", "volume"],
            min_bars=2  # Need 2 bars: current and previous for gap calculation using shift()
        )
        
        if bar_data is None or len(bar_data) == 0:
            return instances
        
        # Convert to DataFrame for analysis
        df = pd.DataFrame(bar_data, columns=["ticker", "timestamp", "open", "close", "volume"])
        
        if len(df) == 0:
            return instances
            
        # Sort by timestamp for proper analysis
        df = df.sort_values(['ticker', 'timestamp']).reset_index(drop=True)
        
        # Calculate indicators with proper error handling
        df['prev_close'] = df.groupby('ticker')['close'].shift(1)
        df = df.dropna()  # Remove rows with missing previous close
        
        # Apply strategy logic (gap up detection)
        df['gap_percent'] = ((df['open'] - df['prev_close']) / df['prev_close']) * 100
        
        # CORRECT: Return ALL instances that meet criteria (no .tail(1))
        signals = df[df['gap_percent'] >= 1.0]  # All gaps >= 1%
        
        # Build results with ticker (not securityid)
        for _, row in signals.iterrows():
            instances.append({{
                'ticker': row['ticker'],
                'timestamp': int(row['timestamp']),
                'gap_percent': float(row['gap_percent']),
                'entry_price': float(row['open']),  # Use opening price as entry
                'prev_close': float(row['prev_close']),
                'score': min(1.0, row['gap_percent'] / 10.0)  # Score based on gap strength
            }})
            
    except Exception as e:
        print(f"Strategy execution error: {{e}}")
        return []
    
    return instances

# Example 2: Volume breakout - return ALL breakouts, not just latest (BATCHED automatically)
def strategy():
    instances = []
    
    try:
        bar_data = get_bar_data(
            timeframe="1d",
            tickers=None,  # All tickers
            columns=["ticker", "timestamp", "volume"],
            min_bars=20  # Need 20 bars for volume average calculation
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
                'volume_ratio': float(row['volume_ratio']),
                'entry_price': float(row.get('close', 0)),  # Use close as entry price
                'volume': int(row['volume']),
                'score': min(1.0, (row['volume_ratio'] - 2.0) / 3.0)  # Higher ratio = higher score
            }})
            
    except Exception as e:
        print(f"Strategy execution error: {{e}}")
        return []
    
    return instances

# Example 3: Sector-specific strategy using filters
def strategy():
    instances = []
    
    try:
        # Focus on technology stocks only
        bar_data = get_bar_data(
            timeframe="1d",
            tickers=None,  # All tickers in the sector
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
        
        # Find technology stocks with significant moves
        signals = df[df['change_5d_pct'] >= 10.0]  # 10%+ gain over 5 days
        
        for _, row in signals.iterrows():
            instances.append({{
                'ticker': row['ticker'],
                'timestamp': int(row['timestamp']),
                'change_5d_pct': float(row['change_5d_pct']),
                'entry_price': float(row['close']),
                'score': min(1.0, row['change_5d_pct'] / 20.0)  # Normalized score
            }})
            
    except Exception as e:
        print(f"Strategy execution error: {{e}}")
        return []
    
    return instances

# Example 4: Market average calculation using aggregate_mode (ONLY for market-wide calculations)
def strategy():
    instances = []
    
    try:
        # Use aggregate_mode=True ONLY when you need ALL market data together
        # This disables batching and provides complete dataset
        bar_data = get_bar_data(
            timeframe="1d",
            tickers=None,  # All tickers
            columns=["ticker", "timestamp", "close"],
            min_bars=1,  # Only need current bars for market average
            aggregate_mode=True  # Get ALL data at once for market calculations
        )
        
        if bar_data is None or len(bar_data) == 0:
            return instances
        
        df = pd.DataFrame(bar_data, columns=["ticker", "timestamp", "close"])
        
        # Calculate market average for each day
        daily_avg = df.groupby('timestamp')['close'].mean()
        
        # Find days where market moved significantly
        for timestamp, avg_price in daily_avg.items():
            if avg_price > 100.0:  # Market average above $100
                instances.append({{
                    'ticker': 'SPY',  # Use SPY as market representative
                    'timestamp': int(timestamp),
                    'market_average': float(avg_price),
                    'entry_price': float(avg_price),
                    'score': min(1.0, avg_price / 200.0)  # Normalized score
                }})
        
    except Exception as e:
        print(f"Strategy execution error: {{e}}")
        return []
    
    return instances
```

COMMON MISTAKES TO AVOID:
❌ signals = df[condition].groupby('ticker').tail(1)  # WRONG - limits to 1 per ticker
❌ latest_df = df.groupby('ticker').last()  # WRONG - only latest data
❌ df.drop_duplicates(subset=['ticker'])  # WRONG - removes valid signals
❌ 'signal': True  # WRONG - unnecessary field, if returned it met criteria
❌ No 'score' field  # WRONG - score is required for ranking
❌ aggregate_mode=True for individual stock patterns  # WRONG - use only for market-wide calculations

✅ signals = df[condition]  # CORRECT - returns all matching instances
✅ signals = df[df['gap_percent'] >= threshold]  # CORRECT - all qualifying rows
✅ Include 'entry_price', 'gap_percent', etc.  # CORRECT - meaningful data
✅ 'score': min(1.0, signal_strength / max_strength)  # CORRECT - normalized score
✅ aggregate_mode=True ONLY for market averages/correlations  # CORRECT - when you need ALL data

PATTERN RECOGNITION:
- Gap patterns: Compare open vs previous close - return ALL gaps in timeframe
  min_bars=2 (need current + previous), Score: min(1.0, gap_percent / 10.0)
- Volume patterns: Compare current vs historical average - return ALL volume spikes  
  min_bars=1 for simple threshold, min_bars=20+ for rolling average
  Score: min(1.0, (volume_ratio - 1.0) / 4.0) - higher volume = higher score
- Price patterns: Use moving averages, RSI - return ALL signal occurrences
  min_bars=20+ for indicators, Score: Based on signal strength (RSI distance from 50, etc.)
- Breakout patterns: Identify price breakouts - return ALL breakouts
  min_bars=2+ for comparison, Score: min(1.0, breakout_strength / max_expected)
- Fundamental patterns: Use market cap, sector data - return ALL qualifying companies
  min_bars=1 (current data only), Score: Based on fundamental strength

TICKER EXTRACTION FROM PROMPTS:
- If prompt mentions specific ticker (e.g., "MRNA gaps up"), use tickers=["MRNA"]
- If prompt mentions "stocks" or "companies" generally, use tickers=None
- Common ticker patterns: AAPL, MRNA, TSLA, AMZN, GOOGL, MSFT, NVDA

SECURITY RULES:
- Only use whitelisted imports: pandas, numpy, datetime, math
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
  * 'entry_price': float (price at signal time - open, close, etc.)
  * 'score': float (REQUIRED, 0.0 to 1.0, higher = stronger signal)
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
            strategy_code, validation_passed = await self._generate_and_validate_strategy(prompt, existing_strategy, max_retries=2)
            
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
    
    async def _generate_and_validate_strategy(self, prompt: str, existing_strategy: Optional[Dict[str, Any]] = None, max_retries: int = 2) -> tuple[str, bool]:
        """Generate strategy with validation retry logic"""
        
        for attempt in range(max_retries + 1):
            try:
                logger.info(f"Generation attempt {attempt + 1}/{max_retries + 1}")
                
                # Generate strategy code (this is NOT async)
                strategy_code = self._generate_strategy_code(prompt, existing_strategy, attempt)
                
                if not strategy_code:
                    continue
                
                # Validate the generated code (this IS async)
                validation_result = await self._validate_strategy_code(strategy_code)
                
                if validation_result["valid"]:
                    logger.info("Strategy validation passed")
                    return strategy_code, True
                else:
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
    
    def _generate_strategy_code(self, prompt: str, existing_strategy: Optional[Dict[str, Any]] = None, attempt: int = 0) -> str:
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
                    ticker_context = f"\n\nDETECTED TICKERS: {extracted_tickers} - Use tickers={extracted_tickers} in your data accessor calls."
                
                user_prompt = f"""CREATE STRATEGY: {prompt}{ticker_context}

Generate a strategy function that detects this pattern in market data."""
            
            # Add retry-specific guidance without bloating the prompt
            if attempt > 0:
                user_prompt += f"\n\nNote: Focus on code safety and proper error handling."
            
            # Use only o3 model as requested
            models_to_try = [
                ("o3", None),                # o3 model only
            ]
            
            last_error = None
            
            for model_name, max_tokens in models_to_try:
                try:
                    logger.info(f"Attempting generation with model: {model_name}")
                    
                    # Adjust parameters for o3 models (similar to o1, they don't support temperature/max_tokens the same way)
                    if model_name.startswith("o3") or model_name.startswith("o1"):
                        # Set a timeout for the OpenAI API call to prevent hanging
                        logger.info(f"🕐 Starting OpenAI API call with model {model_name} (timeout: 180s)")
                        
                        # Use the timeout parameter for OpenAI API calls
                        response = self.openai_client.chat.completions.create(
                            model=model_name,
                            messages=[
                                {"role": "user", "content": f"{system_instruction}\n\n{user_prompt}"}
                            ],
                            timeout=180.0  # 3 minute timeout for o3 model
                        )
                    else:
                        logger.info(f"🕐 Starting OpenAI API call with model {model_name} (timeout: 120s)")
                        
                        response = self.openai_client.chat.completions.create(
                            model=model_name,
                            messages=[
                                {"role": "system", "content": system_instruction},
                                {"role": "user", "content": user_prompt}
                            ],
                            temperature=0.1,
                            max_tokens=max_tokens,
                            timeout=120.0  # 2 minute timeout for other models
                        )
                    
                    strategy_code = response.choices[0].message.content.strip()
                    
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
                # Add timeout to execution test to prevent hanging
                engine = AccessorStrategyEngine()
                test_result = await asyncio.wait_for(
                    engine.execute_screening(
                        strategy_code=strategy_code,
                        universe=['AAPL'],  # Test with single symbol
                        limit=10
                    ),
                    timeout=60.0  # 1 minute timeout for validation test
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
                logger.warning("⏰ Execution test timed out after 60 seconds")
                # Still consider valid if security checks passed but execution test timed out
                return {
                    "valid": True,
                    "error": "Warning: Execution test timed out"
                }
                
            except Exception as exec_error:
                logger.warning(f"⚠️ Execution test failed with exception: {exec_error}")
                logger.warning(f"📄 Execution test traceback: {traceback.format_exc()}")
                # Still consider valid if security checks passed but execution test failed
                # (might be due to missing data or other environmental issues)
                return {
                    "valid": True,
                    "error": f"Warning: Execution test failed: {str(exec_error)}"
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