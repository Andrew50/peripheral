"""
General Python Agent
"""

import logging
import re
import time
import asyncio
import json
from typing import Dict, Any, List, Tuple
from datetime import datetime
from google.genai import types
from validator import validate_code
from utils.context import Context
from utils.data_accessors import get_available_filter_values
from utils.error_utils import capture_exception
from sandbox import PythonSandbox, create_default_config
from generator import _parse_filter_needs_response


logger = logging.getLogger(__name__)

def _get_general_python_system_instruction(ctx: Context, prompt: str) -> str:
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
    return f"""You are a agent that generates Python code for financial data queries.

    Allowed imports:
    - pandas, numpy, datetime, math, plotly.
    - for datetime.datetime, ALWAYS do from datetime import datetime as dt

    FUNCTION VALIDATION - ONLY these custom functions exist, automatically available in the execution environment:
    - get_bar_data(timeframe, columns, min_bars, filters, extended_hours, start_date, end_date) → pandas.DataFrame
    - get_general_data(columns, filters) → pandas.DataFrame

    CRITICAL REQUIREMENTS:
    - code() function with no parameters
    - Use data accessor functions with filters:
    * get_bar_data(timeframe="1d", columns=[], min_bars=1, filters={{"tickers": ["AAPL", "MRNA"]}}) -> pandas DataFrame
        Columns: ticker, timestamp, open, high, low, close, volume
    * get_bar_data(timeframe="5m", filters={{"tickers": ["AAPL"]}}, start_date=datetime(2024,1,15), end_date=datetime(2024,1,15)+timedelta(days=1)) -> pandas DataFrame
        For precise date filtering - essential for multi-timeframe strategies and exact stop loss timing

        SUPPORTED TIMEFRAMES:
        • Direct table access: "1m", "1h", "1d", "1w" (fastest, use when available)
        • Custom aggregations: "5m", "10m", "15m", "30m"
                            "2h", "4h", "6h", "8h"
                            "2w", "3w"

        TIMEFRAME SELECTION GUIDE:
        - Scalping/Day Trading: Use "1m", "5m", "15m", "30m"
        - Swing Trading: Use "1h", "4h", "1d"
        - Position Trading: Use "1d", "1w"
        - Multi-timeframe: Combine different intervals for confirmation

        Min_bars: This is the minimum number of bars needed + 1 bar buffer
            - This cannot exceed 10,000. Use the minimum needed.
            - 1 bar: Simple current patterns (volume spikes, price thresholds)
            - 2 bars: Patterns using shift() for previous values (gaps, daily changes)
            - 20+ bars: Technical indicators (moving averages, RSI)
            - 10,000 bars: This is the maximum number of bars that can be used. If you need more than 10,000 bars, you should use the 1d timeframe.
            - min_bars enforces returning exactly that many bars if available, or all available bars if fewer exist.
            - If start_date and end_date are not provided, retrieves the most recent min_bars bars. For specific date ranges, always provide start_date and end_date.

    * get_bar_data(timeframe="1d", filters={{}})
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
    - NASDAQ biotech: filters={"industry": "Biotechnology", "primary_exchange": "NASDAQ"}''' if industries_str and exchanges_str else ""}{'''
    - Biotechnology stocks: filters={"industry": "Biotechnology"}''' if industries_str else ""}
    - Small cap stocks: filters={{"market_cap_max": 2000000000}}
    - Specific tickers: filters={{"tickers": ["AAPL", "MRNA", "TSLA"]}}

    TICKER USAGE:
    - Always use ticker symbols (strings) like "MRNA", "AAPL", "TSLA" in filters={{"tickers": ["SYMBOL"]}}
    - For specific tickers mentioned in prompts, use filters={{"tickers": ["TICKER_NAME"]}}
    - For universe-wide strategies, use filters={{}} or filters with sector/industry constraints
    - Return results with 'ticker' field (string), not 'securityid'
    - For Bitcoin exposure, use "IBIT" (iShares Bitcoin Trust ETF)
    - For Ethereum exposure, use "ETHE" (Grayscale Ethereum Trust)


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

    COMMON MISTAKES TO AVOID:
    - using TICKER-0 in instead of TICKER - ignore user input in this format and use actual ticker
    - Any value you attach to a dict, list, or Plotly trace must already be JSON-serialisable — so cast NumPy scalars to plain int/float/bool, turn any date-time object (np.datetime64, pd.Timestamp, datetime)
    into an ISO-8601 string (or Unix-seconds int), replace NaN/NA with None, and flatten arrays/Series to plain Python lists before you return or plot them.

    PATTERN RECOGNITION:
    - Gap patterns: Compare open vs previous close - return ALL gaps in timeframe
    min_bars=2 (need current + previous)
    - Volume patterns: Compare current vs historical average - return ALL volume spikes
    min_bars=1 for simple threshold, min_bars=20+ for rolling average
    - Price patterns: Use moving averages, RSI - return ALL qualifying instances
    min_bars=20+ for indicators
    - Breakout patterns: Identify price breakouts - return ALL breakouts
    min_bars=2+ for comparison
    - Fundamental patterns: Use market cap, sector data - return ALL qualifying companies
    min_bars=1 (current data only)

    SECURITY RULES:
    - Only use whitelisted imports
    - CRITICAL: DO NOT use math.fabs() - use the built-in abs() function instead.
    - No file operations, network access, or dangerous functions
    - No exec, eval, or dynamic code execution
    - Use only standard mathematical and data manipulation operations

    CODE OPTIMIZATIONS:
    - Minimize the number of shift() and index manipulation operations

    DATA VALIDATION:
    - Always check if data is None or empty before processing: if data is None or len(data) == 0: return []
    - Use proper DataFrame column checks when needed: if 'column_name' in df.columns
    - Handle missing data gracefully with pandas methods like dropna()
    - Handle edge cases like division by zero

    PRINTING DATA (REQUIRED):
    - Use print() to print useful data for the user
    - This should include things like but not limited to:number of instances, averages, medians, standard deviations, and other nuanced or unusual or interesting metrics.
    - This should SUPER comprehensive. The user will not have access to the data and information other than what is printed and the instance list.
    - Plan in advance the statistics you will print and determine the calculations/intermediary steps needed to get that data.
    - Do not print any time data in timestamp. Always print in the human readable format DD/MM/YYYY HH:MM:SS.

    PLOTLY PLOT GENERATION (REQUIRED):
    - Use plotly to generate plots of useful, value add visualizations of the data
    - Histograms of performance metrics, returns, etc
    - Always show the plot using .show()
    - ENSURE ALL (x,y,z) data is JSON serialisable. NEVER use pandas/numpy types (datetime64, int64, float64, timestamp) and np.ndarray, they cause JSON serialization errors
    - You should style the plot to be visually appealing and informative, specifically focusing on the colors of the layout based on the data. E.g. positive data should be green, negative data should be red, etc.
    - Ensure to name ALL traces in the plot, otherwise the trace will say 'trace 0'.
    - Even if the user does not ask for a plot, you should consider including a plot if it would be useful to the user. Good visualizaions make the USER very satisfied.
    - (Title Icons) For styling, include [TICKER] at the BEGINNING of the title to indicate the ticker icon should be displayed next to the title. ENSURE that this a singular stock ticker, like AAPL, not a spread or other complex instrument.
    - Titles should be concise and to the point and should not include unecessary text like date ranges, etc.
    - If the plot refers to several tickers, do not include a title icon.
    - When the dataset has fewer than five distinct points, avoid oversized bar/line charts. Instead, reason about and produce a visualization that scales gracefully with small‑N data.
    - Dates should always be in American format, and dates should ALWAYS be chronologically ordered, from oldest on the left to the most recent on the right, unless the user specifically asks otherwise.
    - Ensure that the data is properly ordered before plotting.


    **CRITICAL**:
    - NEVER MAKE UP DATA. If you do not have the data, do not include it. Fake data will make the user stop using the tool. The only data you have access to is the functions described above!!
    - If you do not have access to the data, include a print saying that you do not have access to whatever data the agent asked for and that it should websearch for the data.
    - REGARDLESS OF THE QUERY: You are a python agent. Your output should be python function/code in the format as described, with NO other text before, after, or in between the code.
    RETURN FORMAT:
    - Returns are optional
    - Information that would be useful to the user should be returned in the prints, persistent data can be returned.
    - CRITICAL JSON SAFETY: ALL values must be native Python types (int, float, str, bool)
    - REGARDLESS OF THE QUERY: NEVER return pandas/numpy types (datetime64, int64, float64) OR dataframes - they cause JSON serialization errors.
    - DO NOT RUN YOUR FUNCTION AT ALL. DO NOT USE if __name__ == "__main__". THIS WILL CAUSE AN ERROR.
    - NEVER RETURN large amounts of OHLCV data. This will make the user unhappy.
    Generate clean, robust Python code. DO NOT return any text, explanation, or other text following the code. The current date is {datetime.now().strftime('%Y-%m-%d')}. """



async def start_general_python_agent(ctx: Context, user_id: int, prompt: str, data: str, conversation_id: str, message_id: str) -> Tuple[List[Dict], str, List[Dict], List[Dict], str, Exception]:
    # Generate unique execution_id for this run - accessible throughout method
    execution_serial = int(time.time())  # Seconds timestamp
    execution_id = f"{user_id}_{execution_serial}"


    system_instruction = _get_general_python_system_instruction(ctx, prompt)
    user_prompt = f"""{prompt}""" + f"\nData: {data}"

    last_error = None
    python_code = None

    # Retry loop for both validation and execution errors
    for attempt_count in range(3):
        logger.info("🔍 Attempt %s of 3", attempt_count + 1)
        if attempt_count > 0:
            user_prompt = str(prompt)  # Reset to original prompt
            user_prompt += f"\n\nIMPORTANT - RETRY ATTEMPT {attempt_count + 1}:"
            user_prompt += "\n- Previous attempt failed"
            if last_error:
                user_prompt += f"\n- SPECIFIC ERROR: {last_error}"
            user_prompt += "\n- Focus on data type safety for pandas operations"
            user_prompt += "\n- Use pd.to_numeric() before .quantile() operations"
            user_prompt += "\n- Handle NaN values with .dropna() before statistical operations"
            user_prompt += "\n- Ensure proper error handling for edge cases"
            user_prompt += "\n- Verify all imports are properly used"
            user_prompt += "\n- Make sure all variables are defined before use"


        try:
            # Generate code
            openai_response = ctx.conn.openai_client.responses.create(
                model="o4-mini",
                reasoning={"effort": "low"},
                input=user_prompt,
                instructions=system_instruction,
                user="user:0",
                metadata={"userID": str(user_id), "env": ctx.conn.environment, "convID": conversation_id, "msgID": message_id},
                timeout=120.0  # 2 minute timeout for other models
            )
            python_code = _extract_python_code(openai_response.output_text)

            # Validate code
            is_valid = validate_code(python_code, allow_none_return=True, allowed_entrypoints={"code"})
            if not is_valid:
                last_error = "Code failed security validation"
                continue

            # Execute code

            python_sandbox = PythonSandbox(create_default_config(), execution_id=execution_id)
            result = await python_sandbox.execute_code(ctx,python_code)

            # Check if execution was successful
            if not result.success:
                last_error = result.error
                capture_exception(logger, Exception(result.error))
                if result.error_details and result.error_details.get('full_traceback'):
                    logger.error("🔍 Sandbox traceback:\n%s", result.error_details['full_traceback'])

                # Add more specific error context if available
                if result.error_details:
                    error_context = []
                    if result.error_details.get('line_number'):
                        error_context.append(f"Line {result.error_details['line_number']}")
                    if result.error_details.get('error_type'):
                        error_context.append(f"Error type: {result.error_details['error_type']}")
                    if result.error_details.get('code_context'):
                        error_context.append(f"Code context:\n{result.error_details['code_context']}")

                    if error_context:
                        last_error = f"{result.error}\n\nDetailed context:\n" + "\n".join(error_context)

                continue

            # Success! Return results

            # Save successful execution to database in background (non-blocking)
            asyncio.create_task(_save_agent_python_code(
                ctx=ctx,
                user_id=user_id,
                prompt=prompt,
                python_code=python_code,
                execution_id=execution_id,
                result=result.result,
                prints=result.prints,
                plots=result.plots,
                response_images=result.response_images,
                error_message=None
            ))

            return result.result, result.prints, result.plots, result.response_images, execution_id, None

        except Exception as e:  # pylint: disable=broad-exception-caught
            capture_exception(logger, e)
            last_error = str(e)
            logger.error("🔍 Exception: %s", e)
            continue

    # If we get here, all attempts failed
    final_error = Exception(f"Failed to generate and execute valid Python code after 3 attempts. Last error: {last_error}")

    # Save failed execution to database with error info in background (non-blocking)
    asyncio.create_task(_save_agent_python_code(
        ctx=ctx,
        user_id=user_id,
        prompt=prompt,
        python_code=python_code if python_code else "",
        execution_id=execution_id,
        result=None,
        prints="",
        plots=[],
        response_images=[],
        error_message=str(final_error)
    ))

    return [], "", [], [], execution_id, final_error

        # Save failed execution to database with error info in background (non-blocking)
   

def _extract_python_code(response: str) -> str:
    """Extract Python code from response, removing markdown formatting"""
    # Remove markdown code blocks
    code_block_pattern = r'```(?:python)?\s*(.*?)\s*```'
    matches = re.findall(code_block_pattern, response, re.DOTALL)

    if matches:
        return matches[0].strip()

    # If no code blocks found, return the response as-is
    return response.strip()

async def _save_agent_python_code(ctx: Context, user_id: int, prompt: str, python_code: str,
                                execution_id: str, result: Any = None, prints: str = "",
                                plots: list = None, response_images: list = None,
                                error_message: str = None) -> bool:
    """Save Python agent execution to database"""
    conn = None
    cursor = None
    # Use connection manager if available, otherwise fallback to direct connection
    
    ctx.conn.ensure_db_connection()
    conn = ctx.conn.db_conn
    cursor = conn.cursor()

    # Convert complex objects to JSON strings for storage
    result_json = json.dumps(result) if result is not None else None
    plots_json = json.dumps(plots) if plots is not None else None
    response_images_json = json.dumps(response_images) if response_images is not None else None

    # Insert the execution record
    cursor.execute("""
        INSERT INTO python_agent_execs (
            userid, prompt, python_code, execution_id, result, prints,
            plots, response_images, error_message, created_at
        ) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, NOW())
    """, (
        user_id, prompt, python_code, execution_id, result_json,
        prints, plots_json, response_images_json, error_message
    ))

    conn.commit()
    if cursor:
        cursor.close()
    return True




def python_agent(ctx: Context, user_id: int = None,
                                    prompt: str = None, data: str = None, conversation_id: str = None, message_id: str = None) -> Dict[str, Any]:
# Initialize defaults to avoid scope issues
    result, prints, plots, response_images = [], "", [], []
    execution_id = None  # Initialize to avoid UnboundLocalError

    #try:
    # Validate input parameters
    if user_id is None:
        raise ValueError("user_id is required for general Python agent")
    if not prompt or not prompt.strip():
        raise ValueError("prompt is required for general Python agent")

    # Publish progress update
    #if task_id:
        #ctx.publish_progress(task_id, "initializing", "Starting general Python agent execution...")

    # Execute with timeout
    result, prints, plots, response_images, execution_id, error = asyncio.run(
        start_general_python_agent(
            ctx=ctx,
            user_id=user_id,
            prompt=prompt,
            data=data,
            conversation_id=conversation_id,
            message_id=message_id
        )
    )

    # Check if there was an error
    if error:
        #logger.error("❌ General Python agent execution FAILED for task %s: %s", task_id, error)
        return {
            "success": False,
            "error": str(error),
            "result": result,
            "prints": prints,
            "plots": plots,
            "responseImages": response_images,
            "executionID": execution_id,
        }


    return {
        "success": True,
        "result": result,
        "prints": prints,
        "plots": plots,
        "responseImages": response_images,
        "executionID": execution_id,
    }

    #except Exception as e:
        #capture_exception(logger, e)

        #return {
            #"success": False,
            #"error": str(e),
            #"result": result,
            #"prints": prints,
            #"plots": plots,
            #"responseImages": response_images,
            #"executionID": execution_id,
        #}
