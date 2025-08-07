"""
Strategy Generator
Generates trading strategies from natural language using OpenAI o3 and validates them immediately.
"""

import json
import logging
import re
from datetime import datetime
from typing import Dict, Any, Optional

from google.genai import types
from validator import validate_strategy
from utils.context import Context
from utils.strategy_crud import fetch_strategy_code, save_strategy
from utils.error_utils import capture_exception
from utils.errors import ModelGenerationError
from utils.data_accessors import get_available_filter_values
from utils.ticker_extractor import extract_tickers

logger = logging.getLogger(__name__)

# Regex to find get_bar_data calls with timeframe parameter
_TIMEFRAME_RE = re.compile(
    r"get_bar_data\s*\([\s\S]*?timeframe\s*=\s*[\"']([^\"']+)[\"']",
    re.MULTILINE,
)

def _detect_min_timeframe(code: str) -> str:
    """Extract the minimum timeframe from strategy code by parsing get_bar_data calls"""
    matches = _TIMEFRAME_RE.findall(code)
    if not matches:
        return "1d"  # Default fallback
    
    def tf_as_minutes(tf: str) -> int:
        """Convert timeframe string to minutes for comparison"""
        # Parse using similar logic to data_accessors._parse_timeframe
        pattern = r'^(\d+)([mhdwqy]?)$'
        match = re.match(pattern, tf.lower())
        if not match:
            return 1440  # Default to 1 day in minutes
        
        value, unit = match.groups()
        value = int(value)
        
        # Convert to minutes
        if not unit or unit == 'm':  # minutes (no unit means minutes)
            return value
        if unit == 'h':  # hours
            return value * 60
        if unit == 'd':  # days
            return value * 1440
        if unit == 'w':  # weeks
            return value * 10080
        if unit == 'q':  # quarters (3 months = ~90 days)
            return value * 129600
        if unit == 'y':  # years (365 days)
            return value * 525600
        return 1440  # Default fallback
    
    # Return the timeframe with the smallest duration
    return min(matches, key=tf_as_minutes)


def _parse_filter_needs_response( response) -> Dict[str, bool]:
    """Parse the Gemini API response to determine which filters are needed"""
    try:
        response_text = response.text.strip()
        # Try to extract JSON if it's wrapped in code blocks
        if '```json' in response_text:
            json_match = re.search(r'```json\s*(\{.*?\})\s*```', response_text, re.DOTALL)
            if json_match:
                response_text = json_match.group(1)
        elif '```' in response_text:
            json_match = re.search(r'```\s*(\{.*?\})\s*```', response_text, re.DOTALL)
            if json_match:
                response_text = json_match.group(1)

        filter_needs = json.loads(response_text)
        return filter_needs
    except (json.JSONDecodeError, AttributeError) as e:
        logger.warning("Failed to parse filter needs JSON: %s, response: %s", e, response.text)
        # Default to needing all filters if parsing fails
        return {"sectors": True, "industries": True, "primary_exchanges": True}

def _get_system_instruction(ctx, prompt: str) -> str:
    """Get system instruction for OpenAI code generation with current database filter values"""
    contents = [
        types.Content(role="user", parts=[
            types.Part.from_text(text=prompt),
        ])
    ]
    generate_content_config = types.GenerateContentConfig(
        thinking_config = types.ThinkingConfig(
            thinking_budget=0
        ),
        system_instruction =[types.Part.from_text(text="""You are a lightweight classifier tasked to determine whether a the list of filter options is needed for a given strategy generation query. You will be given a strategy query and then
            you are to return a JSON struct of the following keys and false or true values of whether the filters values are needed.
            - sectors: A list of sector options like \"Energy\", \"Finance\", \"Health Care\"
            - industries: \"Life Insurance\", \"Major Banks\", \"Major Chemicals\"
            - primary_exchanges: NYSE, NASDAQ, ARCA
            ONLY include true if building a strategy around the prompt REQUIRES one of the filter options.""")],
    )
    response = ctx.conn.gemini_client.models.generate_content(
        model="gemini-2.5-flash-lite-preview-06-17",
        contents=contents,
        config=generate_content_config,
    )

    # Parse the JSON response to determine which filters are needed
    filter_needs = _parse_filter_needs_response(response)

    # Only get filter values from database if they're needed
    filter_values = {}
    if any(filter_needs.values()):
        db_filter_values = get_available_filter_values(ctx)

        # Only include the filter values that are marked as needed
        if filter_needs.get("sectors", False):
            filter_values['sectors'] = db_filter_values['sectors']
        if filter_needs.get("industries", False):
            filter_values['industries'] = db_filter_values['industries']
        if filter_needs.get("primary_exchanges", False):
            filter_values['primary_exchanges'] = db_filter_values['primary_exchanges']

    # Format filter values for the prompt (only include if they exist)
    sectors_str = '", "'.join(filter_values.get('sectors', [])) if filter_values.get('sectors') else ""
    industries_str = '", "'.join(filter_values.get('industries', [])) if filter_values.get('industries') else ""
    exchanges_str = '", "'.join(filter_values.get('primary_exchanges', [])) if filter_values.get('primary_exchanges') else ""

    return f"""You are a trading strategy generator that creates Python functions using data accessor functions.

        Allowed imports:
        - pandas, numpy, datetime, math, plotly.
        - for datetime.datetime, ALWAYS do from datetime import datetime as dt

        FUNCTION VALIDATION - ONLY these functions exist, automatically available in the execution environment:
        - get_bar_data(timeframe, columns, min_bars, filters, aggregate_mode, extended_hours, start_date, end_date) → pandas.DataFrame
        - get_general_data(columns, filters) → pandas.DataFrame
        - apply_drawdown_styling(fig) → returns styled fig
        - apply_equity_curve_styling(fig) → returns styled fig

        CRITICAL REQUIREMENTS:
        - Function must be named 'strategy' with no parameters
        - Use data accessor functions with filters (get_bar_data returns pandas.DataFrame directly):
        * get_bar_data(timeframe="1d", columns=[], min_bars=1, filters={{"tickers": ["AAPL", "MRNA"]}}) -> pandas.DataFrame
            Columns: ticker, timestamp, open, high, low, close, volume
        * get_bar_data(timeframe="5", filters={{"tickers": ["AAPL"]}}, start_date=datetime(2024,1,15), end_date=datetime(2024,1,15)+timedelta(days=1)) -> pandas.DataFrame
            For precise date filtering - essential for multi-timeframe strategies and exact stop loss timing

            SUPPORTED TIMEFRAMES:
            • Custom aggregations: Any integer prefix (1 to within reason) + time unit:
                - <none> (minute): "1", "5", "15", "30", "60", etc.
                - h (hourly): "1h", "2h", "4h", "8h", "12h", etc.
                - d (daily): "1d", "2d", "3d", "7d", etc.
                - w (weekly): "1w", "2w", "3w", "4w", etc.
                - m (monthly): "1m", "2m", "3m", "6m", etc.
                - q (quarterly): "1q", "2q", "3q", "4q", etc.
                - y (yearly): "1y", "2y", "3y", "5y", etc.

            TIMEFRAME SELECTION GUIDE:
            - Scalping/Day Trading: Use "1", "5", "15", "30"
            - Swing Trading: Use "1h", "4h", "1d"
            - Position Trading: Use "1d", "1w", "1m", "1q", "1y"
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
            Columns: ticker, name, sector, industry, market_cap, primary_exchange, active, total_shares

        AVAILABLE FILTERS (use in filters parameter):{f'''
        - sector: "{sectors_str}"''' if sectors_str else ""}{f'''
        - industry: "{industries_str}"''' if industries_str else ""}{f'''
        - primary_exchange: "{exchanges_str}"''' if exchanges_str else ""}
        - market_cap_min: float (e.g., 1000000000 for $1B minimum)
        - market_cap_max: float (e.g., 10000000000 for $10B maximum)

        FILTER EXAMPLES:{'''
        - Technology stocks: filters={"sector": "Technology"}''' if sectors_str else ""}{'''
        - Large cap healthcare: filters={"sector": "Healthcare", "market_cap_min": 10000000000}''' if sectors_str else ""}{'''
        - NASDAQ biotech: filters={"industry": "Biotechnology", "primary_exchange": "NASDAQ"}''' if industries_str and exchanges_str else '''
        - Biotechnology stocks: filters={"industry": "Biotechnology"}''' if industries_str else '''
        - NASDAQ stocks: filters={"primary_exchange": "NASDAQ"}''' if exchanges_str else ""}
        - Small cap stocks: filters={{"market_cap_max": 2000000000}}
        - Specific tickers: filters={{"tickers": ["AAPL", "MRNA", "TSLA"]}}

        TICKER USAGE:
        - Always use ticker symbols (strings) like "MRNA", "AAPL", "TSLA" in filters={{"tickers": ["SYMBOL"]}}
        - For specific tickers mentioned in prompts, use filters={{"tickers": ["TICKER_NAME"]}}
        - For universe-wide strategies, use filters={{}} or filters with sector/industry constraints
        - Return results with 'ticker' field (string), not 'securityid'
        - For Bitcoin exposure, use "IBIT" (iShares Bitcoin Trust ETF)
        - For Ethereum exposure, use "ETHE" (Grayscale Ethereum Trust)

        CRITICAL: RETURN ALL MATCHING INSTANCES, NOT JUST THE LATEST
        - DO NOT use .tail(1) or .head(1) to limit results per ticker
        - Return every occurrence that meets the criteria across the entire datasetç
        - Example: If MRNA gaps up 1% on 5 different days, return all 5 instances

        CRITICAL: INSTANCE STRUCTURE
        - Include relevant price data: 'open', 'close', 'entry_price' when available
                        - Use proper timestamp format: int(row['timestamp']) for Unix timestamp (in seconds)
        - REQUIRED: Include 'score': float (0.0 to 1.0) - higher score = stronger signal

        CRITICAL: ALWAYS INCLUDE INDICATOR VALUES IN INSTANCES
        - MUST include ALL calculated indicator values that triggered your strategy
        - Examples: 'volume_ratio': 2.3, 'gap_percent': 4.1
        - Include intermediate calculations: 'sma_20': 150.5, 'ema_12': 148.2, 'bb_upper': 155.0
        - Include percentage changes: 'change_1d_pct': 3.2, 'change_5d_pct': 8.7
        - Include ratios and scores: 'momentum_score': 0.85, 'strength_ratio': 1.4
        - DO NOT include static thresholds or constants (e.g., 'rsi_threshold': 30)
        - This data is ESSENTIAL for backtesting, analysis, and understanding why signals triggered

        CRITICAL: min_bars MUST BE ABSOLUTE MINIMUM + 1 BAR BUFFER (NO ADDITIONAL BUFFER)
        - min_bars = EXACT number of bars required for calculation, NOT a suggestion
        - Examples: RSI needs 14 bars → min_bars=15, MACD needs 26 bars → min_bars=27
        - If you need multiple indicators, use the MAXIMUM of their individual minimums
        - Example: RSI(14) + SMA(50) strategy → min_bars=51 (not 64, not 55)

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
        - If data is invalid, simply return an empty list: return []

        EXAMPLE PATTERNS:
        ```python
        # Example 1: Multi-timeframe RSI + Trend Strategy
        def strategy():
            instances = []

                        # Get daily data for RSI calculation
            df_1d = get_bar_data(
                timeframe="1d",
                columns=["ticker", "timestamp", "close"],
                min_bars=15,  # Need 15 bars for RSI calculation
                filters={{"sector": "Technology"}}  # Filter to technology sector
            )
            
            # Get hourly data for short-term trend
            df_1h = get_bar_data(
                timeframe="1h",
                columns=["ticker", "timestamp", "close"],
                min_bars=5,   # Need 5 hours for short-term moving average
                filters={{"sector": "Technology"}}
            )
            
            if df_1d is None or len(df_1d) == 0 or df_1h is None or len(df_1h) == 0:
                return instances

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

        # Example 2: Gap Strategy with Specific Tickers
        def strategy():
            instances = []

                        target_tickers = ["AAPL", "TSLA", "NVDA"]  # Specific tickers from prompt
            
            df = get_bar_data(
                timeframe="1d",
                columns=["ticker", "timestamp", "open", "close"],
                min_bars=2,  # Need 2 bars: previous close + current open
                filters={{"tickers": target_tickers}}
            )
            
            if df is None or len(df) == 0:
                return instances
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
                    'gap_percent': round(float(row['gap_percent']), 3),
                    'prev_close': round(float(row['prev_close']), 2),
                    'gap_magnitude': round(float(abs(row['gap_percent'])), 3),
                    'score': round(min(1.0, abs(row['gap_percent']) / 10.0), 3)  # Normalize by 10%
                }})

            return instances
        ```

        COMMON MISTAKES TO AVOID:
        - latest_df = df.groupby('ticker').last() - only latest data
        - df.drop_duplicates(subset=['ticker']) - this removes valid instances
        - No 'score' field - score is required for ranking
        - aggregate_mode=True for individual stock patterns - use only for market-wide calculations
        - using TICKER-0 in instead of TICKER - ignore user input in this format and use actual ticker
        - Converting get_bar_data result to DataFrame - it's already a pandas.DataFrame, use directly
        - Any value you attach to a dict, list, or Plotly trace must already be JSON-serialisable — so cast NumPy scalars to plain int/float/bool, turn any date-time object (np.datetime64, pd.Timestamp, datetime)
        into an ISO-8601 string (or Unix-seconds int), replace NaN/NA with None, and flatten arrays/Series to plain Python lists before you return or plot them.
        - BAD STOP LOSS: if low <= stop: exit_price = stop_price  # Ignores gaps!
        - NO DATE FILTERING: Using only daily data for precise stop timing
        - Appending instances only after exit is determined – ALWAYS record the entry as an instance, even when you can't yet determine an exit.

        ✅ qualifying_instances = df[condition]  # CORRECT - returns all matching instances
        ✅ qualifying_instances = df[df['gap_percent'] >= threshold]  # CORRECT - all qualifying rows
        ✅ df = get_bar_data(...)  # CORRECT - get_bar_data returns pandas.DataFrame directly
        ✅ Include 'entry_price', 'gap_percent', etc.  # CORRECT - meaningful data
        ✅ 'score': min(1.0, instance_strength / max_strength)  # CORRECT - normalized score
        ✅ aggregate_mode=True ONLY for market averages/correlations  # CORRECT - when you need ALL data
        ✅ GOOD STOP LOSS: if open <= stop: exit_price = open  # CORRECT - handles gaps
        ✅ GOOD STOP LOSS: Check gap first, then intraday  # CORRECT - proper order
        ✅ DATE FILTERING: get_bar_data(start_date=day_start, end_date=day_end)  # CORRECT - precise timing

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

        SECURITY RULES:
        - Only use whitelisted imports
        - CRITICAL: DO NOT use math.fabs() - use the built-in abs() function instead.
        - No file operations, network access, or dangerous functions
        - No exec, eval, or dynamic code execution
        - Use only standard mathematical and data manipulation operations

        CODE OPTIMIZATIONS:
        - Drop intermediate columns once they are no longer needed that are not output in the final instance list
        - Minimize the number of shift() and index manipulation operations

        DATA VALIDATION:
        - Check if DataFrame is None or empty before processing: if df is None or len(df) == 0: handle accordingly which may involve simply returning []
        - Use proper DataFrame column checks when needed: if 'column_name' in df.columns
        - Handle missing data gracefully with pandas methods like dropna()
        - Return empty list [] when no valid data is available
        - Handle edge cases like division by zero

        **CRITICAL STOP LOSS IMPLEMENTATION**:
        BE EXTREMELY CAREFUL WITH stop loss prices. Markets can gap through stops, resulting in much worse exits than expected.

        # Step 1: Check for overnight gaps FIRST
        if day_open <= stop_loss_price:
            exit_price = day_open  # ✅ Use actual gap-down price, NOT stop price
            exit_reason = 'gap_below_stop'
        # Step 2: Only then check intraday stop hits
        elif day_low <= stop_loss_price:
            exit_price = stop_loss_price  # ✅ Safe - no gap occurred
            exit_reason = 'intraday_stop_hit'

        ENHANCED PRECISION WITH DATE FILTERING:
        For exact stop timing, use multi-timeframe analysis:
        1. Daily data identifies potential stop days
        2. Use start_date/end_date to get intraday bars for specific days
        3. Walk through intraday bars to find exact stop hit timing
        4. Always check gap-down scenarios first before intraday analysis

        Example: get_bar_data(timeframe="5", filters={{"tickers": ["COIN"]}}, start_date=day_start, end_date=day_end)

        PRINTING DATA (REQUIRED):
        - Use print() to print useful data for the user
        - This should include things like but not limited to:number of instances, averages, medians, standard deviations, and other nuanced or unusual or interesting metrics.
        - This should SUPER comprehensive. The user will not have access to the data and information other than what is printed and the instance list.

        PLOTLY PLOT GENERATION (REQUIRED):
        - Use plotly to generate plots of useful visualizations of the data
        - Histograms of performance metrics, returns, etc
        - Always show the plot using .show()
        - Almost always include plots in the strategy to help the user understand the data
        - Ensure to name ALL traces in the plot, otherwise the trace will say 'trace 0'.
        - ENSURE ALL (x,y,z) data is JSON serialisable. NEVER use pandas/numpy types (datetime64, int64, float64, timestamp) and np.ndarray, they cause JSON serialization errors
        - Plot equity curve AND drawdown plot of the P/L and drawdown performance overtime on separate line plots. These should not be scatterplots.
        - For the drawdown plot, use apply_drawdown_styling(fig) to style the plot
        - For the equity curve plot, use apply_equity_curve_styling(fig) to style the plot
        - Do NOT use timestamp as x-axis values. Use dates instead.
        - (Title Icons) For styling, include [TICKER] at the VERY BEGINNING of the title to indicate the ticker who's company icon should be displayed next to the title.
        - Titles should be concise and to the point and should not include unecessary text like date ranges, etc.
        - ENSURE that this a singular stock ticker, like AAPL, not a spread or other complex instrument.
        - If the plot refers to several tickers, do not include this.
        - Dates should always be in American format.

        RETURN FORMAT:
        - *ALWAYS* Return List[Dict] where each dict contains:
        * 'ticker': str (required) (e.g., "MRNA", "AAPL")
        * 'timestamp': int (required) (Unix timestamp, NOT datetime or timestamptz)
        * 'entry_price': float (price at instance time - open, close, etc.)
        * 'score': float (REQUIRED, 0.0 to 1.0, higher = stronger instance. Rounded to 3 decimal places)
        * Additional fields as needed for strategy results (gap_percent, volume_ratio, etc. Rounded to 3 decimal places)
        * Order these fields logically that would make it best for the reader to understand the table of instances.
        - CRITICAL JSON SAFETY: ALL values must be native Python types (int, float, str, bool)
        - NEVER return pandas/numpy types (datetime64, int64, float64) - they cause JSON serialization errors
        - ENSURE YOU RETURN THE TRADES/INSTANCES. Do not omit.
        - ALL trades should be shown. The instance list should still consider new trades even if there are open trades.
        - Instance should STILL be added even if exits have not occured or there is not enough data yet to calculate an exit. Both closed and open trades should be returned.

        Generate clean, robust Python code."""

def create_strategy(ctx: Context, user_id: int, prompt: str, strategy_id: int = -1, conversation_id: str = None, message_id: str = None) -> Dict[str, Any]:


    # Check if this is an edit operation
    is_edit = strategy_id != -1
    existing_strategy = None

    if is_edit:
        result = fetch_strategy_code(ctx, user_id, strategy_id)
        if not result:
            return {
                "success": False,
                "error": f"Strategy {strategy_id} not found for user {user_id}"
            }
        # Unpack fetched strategy code into dict for consistent access
        existing_strategy = {"pythonCode": result[0], "version": result[1]}

    strategy_code = _generate_and_validate_strategy(ctx, user_id, prompt, existing_strategy, conversation_id, message_id, max_retries=2)

    if not strategy_code:
        return {
            "success": False,
            "error": "Failed to generate valid strategy code after retries"
        }

    # Detect minimum timeframe from the generated strategy code
    min_timeframe = _detect_min_timeframe(strategy_code)
    logger.info("Detected minimum timeframe for strategy: %s", min_timeframe)

    # Extract ticker universe from the generated strategy code
    ticker_extraction = extract_tickers(strategy_code)
    alert_universe_full = ticker_extraction["all_tickers"]
    is_global_strategy = ticker_extraction["has_global"]
    
    logger.info("Extracted ticker universe: %s (global: %s)", alert_universe_full, is_global_strategy)

    description = _extract_description(strategy_code, prompt)
    if is_edit:
        name = existing_strategy.get('name', 'Strategy')
    else:
        name = _generate_strategy_name(prompt)

    saved_strategy = save_strategy(
        ctx,
        user_id=user_id,
        name=name,
        description=description,
        prompt=prompt,
        python_code=strategy_code,
        strategy_id=strategy_id if is_edit else None,
        min_timeframe=min_timeframe,
        alert_universe_full=alert_universe_full if not is_global_strategy else None
    )

    return {
        "success": True,
        "strategy": saved_strategy,
        "validation_passed": True  # If we got here, validation passed
    }



def _generate_and_validate_strategy(ctx: Context, user_id: int, prompt: str, existing_strategy: Optional[Dict[str, Any]] = None, conversation_id: str = None, message_id: str = None, max_retries: int = 2) -> str:
    """Generate strategy with validation retry logic"""

    last_validation_error = None

    for attempt in range(max_retries + 1):
        try:
            strategy_code = _generate_strategy_code(ctx, user_id, prompt, existing_strategy, last_validation_error, conversation_id, message_id)
            if strategy_code:
                valid, validation_error = validate_strategy(ctx, strategy_code)
                if valid:
                    return strategy_code
                # Store the detailed validation error for the next retry
                last_validation_error = validation_error
                logger.warning("Validation failed on attempt %s: %s...", attempt + 1, validation_error[:200])
        except ModelGenerationError as e:
            capture_exception(logger, e)
            last_validation_error = str(e)
      
        if attempt == max_retries:
            break

    return ""


def _generate_strategy_code(ctx, user_id: int, prompt: str, existing_strategy: Optional[Dict[str, Any]] = None,  last_error: Optional[str] = None, conversation_id: str = None, message_id: str = None) -> str:
    """
    Generate strategy code using OpenAI with optimized prompts
    """
    try:
        system_instruction = _get_system_instruction(ctx, prompt)

        # Create concise user prompt based on context
        if existing_strategy:
            # For editing - keep it minimal
            user_prompt = f"""
            CURRENT STRATEGY: {existing_strategy.get('name', 'Strategy')} \n
            CURRENT STRATEGY CODE: {existing_strategy.get('pythonCode', '')} \n
            EDIT REQUEST: {prompt} \n
            Generate the updated strategy function."""
        else:

            user_prompt = f"""CREATE STRATEGY: {prompt}"""


        # Add retry-specific guidance with error context
        if last_error:
            user_prompt += "\n\nIMPORTANT - RETRY ATTEMPT:"
            user_prompt += "\n- Previous attempt failed validation"
            if last_error:
                # Truncate error message if too long to avoid overwhelming the model
                err_msg = (last_error[:1500] + "...") if len(last_error) > 1500 else last_error
                user_prompt += f"\n- SPECIFIC ERROR:\n{err_msg}"
            user_prompt += "\n- Focus on data type safety for pandas operations"
            user_prompt += "\n- Use pd.to_numeric() before .quantile() operations"
            user_prompt += "\n- Handle NaN values with .dropna() before statistical operations"
            user_prompt += "\n- Ensure proper error handling for edge cases"

        model_name = "gpt-5-mini"
        last_error = None



        response = ctx.conn.openai_client.responses.create(
            model=model_name,
            reasoning={"effort": "low"},
            input=user_prompt,
            instructions=system_instruction,
            user="user:0",
            metadata={"userID": str(user_id), "env": ctx.conn.environment, "convID": conversation_id, "msgID": message_id},
            timeout=150.0  # 150 second timeout for other models
        )

        strategy_code = response.output_text
        strategy_code = _extract_python_code(strategy_code)

        return strategy_code

    except Exception as e:
        # Capture and log any unexpected exception with full traceback
        capture_exception(logger, e)
        raise ModelGenerationError(f"OpenAI API call failed: {str(e)}") from e




def _extract_python_code(response: str) -> str:
    """Extract Python code from response, removing markdown formatting"""
    # Remove markdown code blocks
    code_block_pattern = r'```(?:python)?\s*(.*?)\s*```'
    matches = re.findall(code_block_pattern, response, re.DOTALL)

    if matches:
        return matches[0].strip()

    # If no code blocks found, return the response as-is
    return response.strip()

def _extract_description(strategy_code: str, prompt: str) -> str:
    """Extract or generate description from strategy code and prompt"""
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

def _generate_strategy_name(prompt: str) -> str:
    """Generate a strategy name"""

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
