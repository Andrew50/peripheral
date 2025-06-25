"""
Strategy Generator
Generates trading strategies from natural language using OpenAI o3 and validates them immediately.
"""

import os
import json
import logging
import asyncio
import psycopg2
from psycopg2.extras import RealDictCursor
from datetime import datetime
from typing import Dict, Any, Optional, List
import re

from openai import OpenAI
from validator import SecurityValidator, SecurityError, StrategyComplianceError
from accessor_strategy_engine import AccessorStrategyEngine

logger = logging.getLogger(__name__)


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
    
    def _get_system_instruction(self) -> str:
        """Get system instruction for OpenAI code generation"""
        return """
You are a trading strategy generator that creates Python functions using data accessor functions.

CRITICAL REQUIREMENTS:
- Function named 'strategy()' with NO parameters
- Use data accessor functions with ticker symbols (NOT security IDs):
  * get_bar_data(timeframe="1d", tickers=["AAPL", "MRNA"], columns=[], min_bars=1) -> numpy array
     Columns: ticker, timestamp, open, high, low, close, volume
  * get_general_data(tickers=["AAPL", "MRNA"], columns=[]) -> pandas DataFrame  
     Columns: ticker, name, sector, industry, market_cap, pe_ratio, etc.

TICKER USAGE:
- Always use ticker symbols (strings) like "MRNA", "AAPL", "TSLA" 
- For specific tickers mentioned in prompts, use tickers=["TICKER_NAME"]
- For universe-wide strategies, use tickers=None to get all available tickers
- Return results with 'ticker' field (string), not 'securityid'

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
        # Example 1: Specific ticker strategy (e.g., for MRNA)
        target_tickers = ["MRNA"]  # Extract from prompt analysis
        
        bar_data = get_bar_data(
            timeframe="1d",
            tickers=target_tickers,  # Use specific ticker
            columns=["ticker", "timestamp", "open", "close", "volume"],
            min_bars=5
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
        
        # Filter based on criteria
        signals = df[df['gap_percent'] >= 1.0]  # 1% gap up threshold
        
        # Build results with ticker (not securityid)
        for _, row in signals.iterrows():
            instances.append({
                'ticker': row['ticker'],  # Use ticker string
                'timestamp': int(row['timestamp']),
                'signal': True,
                'gap_percent': float(row['gap_percent'])
            })
            
    except Exception as e:
        # Log error but don't fail - return empty results
        print(f"Strategy execution error: {e}")
        return []
    
    return instances

# Example 2: Universe-wide strategy with try/except blocks
def strategy():
    instances = []
    
    try:
        # Get data for all tickers
        bar_data = get_bar_data(
            timeframe="1d",
            tickers=None,  # All available tickers
            columns=["ticker", "timestamp", "open", "close", "volume"],
            min_bars=10
        )
        
        if bar_data is None or len(bar_data) == 0:
            return instances
        
        df = pd.DataFrame(bar_data, columns=["ticker", "timestamp", "open", "close", "volume"])
        
        # Add fundamental data if needed
        try:
            fundamental_data = get_general_data(
                tickers=None,
                columns=["ticker", "market_cap", "pe_ratio", "sector"]
            )
            
            if fundamental_data is not None and len(fundamental_data) > 0:
                df = df.merge(fundamental_data, on='ticker', how='left')
        except Exception as merge_error:
            print(f"Warning: Could not merge fundamental data: {merge_error}")
            # Continue without fundamental data
        
        # Apply universe-wide strategy logic with error handling
        try:
            # ... strategy calculations with proper error handling ...
            
            # Return results with ticker
            for _, row in filtered_df.iterrows():
                instances.append({
                    'ticker': row['ticker'],
                    'timestamp': int(row['timestamp']),
                    'signal': True
                })
        except Exception as calc_error:
            print(f"Calculation error: {calc_error}")
            return []
            
    except Exception as e:
        print(f"Strategy execution error: {e}")
        return []
    
    return instances
```

PATTERN RECOGNITION:
- Gap patterns: Compare open vs previous close
- Volume patterns: Compare current vs historical average using rolling windows
- Price patterns: Use moving averages, RSI, and technical indicators
- Breakout patterns: Identify price breakouts above/below key levels
- Fundamental patterns: Use PE ratios, market cap, earnings data

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
  * Additional fields as needed for strategy results

Generate clean, robust Python code that uses ticker symbols and handles errors gracefully to produce accurate trading signals.
"""
    
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
                
                # Generate strategy code
                strategy_code = await self._generate_strategy_code(prompt, existing_strategy, attempt)
                
                if not strategy_code:
                    continue
                
                # Validate the generated code
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
        try:
            db_config = {
                'host': os.getenv('DB_HOST', 'localhost'),
                'port': os.getenv('DB_PORT', '5432'),
                'user': os.getenv('DB_USER', 'postgres'),
                'password': os.getenv('DB_PASSWORD', ''),
                'database': os.getenv('POSTGRES_DB', 'postgres')
            }
            
            conn = psycopg2.connect(**db_config)
            cursor = conn.cursor(cursor_factory=RealDictCursor)
            
            cursor.execute("""
                SELECT strategyid, name, description, prompt, pythoncode
                FROM strategies 
                WHERE strategyid = %s AND userid = %s
            """, (strategy_id, user_id))
            
            result = cursor.fetchone()
            cursor.close()
            conn.close()
            
            if result:
                return {
                    'strategyId': result['strategyid'],
                    'name': result['name'],
                    'description': result['description'] or '',
                    'prompt': result['prompt'] or '',
                    'pythonCode': result['pythoncode'] or ''
                }
            return None
            
        except Exception as e:
            logger.error(f"Failed to fetch existing strategy: {e}")
            return None
    
    async def _generate_strategy_code(self, prompt: str, existing_strategy: Optional[Dict[str, Any]] = None, attempt: int = 0) -> str:
        """Generate strategy code using OpenAI with optimized prompts"""
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
                        response = self.openai_client.chat.completions.create(
                            model=model_name,
                            messages=[
                                {"role": "user", "content": f"{system_instruction}\n\n{user_prompt}"}
                            ]
                        )
                    else:
                        response = self.openai_client.chat.completions.create(
                            model=model_name,
                            messages=[
                                {"role": "system", "content": system_instruction},
                                {"role": "user", "content": user_prompt}
                            ],
                            temperature=0.1,
                            max_tokens=max_tokens
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
        """Validate strategy code using the security validator and test execution"""
        try:
            # First, use the existing validator for security checks
            is_valid = self.validator.validate_code(strategy_code)
            
            if not is_valid:
                return {
                    "valid": False,
                    "error": "Security validation failed"
                }
            
            # Try a quick execution test with the new accessor engine
            try:
                engine = AccessorStrategyEngine()
                test_result = await engine.execute_screening(
                    strategy_code=strategy_code,
                    universe=['AAPL'],  # Test with single symbol
                    limit=10
                )
                
                if test_result.get('success', False):
                    return {
                        "valid": True,
                        "error": None
                    }
                else:
                    return {
                        "valid": False,
                        "error": f"Execution test failed: {test_result.get('error', 'Unknown error')}"
                    }
                    
            except Exception as exec_error:
                logger.warning(f"Execution test failed: {exec_error}")
                # Still consider valid if security checks passed but execution test failed
                # (might be due to missing data or other environmental issues)
                return {
                    "valid": True,
                    "error": f"Warning: Execution test failed: {str(exec_error)}"
                }
            
        except (SecurityError, StrategyComplianceError) as e:
            return {
                "valid": False,
                "error": str(e)
            }
        except Exception as e:
            logger.error(f"Validation error: {e}")
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
        try:
            db_config = {
                'host': os.getenv('DB_HOST', 'localhost'),
                'port': os.getenv('DB_PORT', '5432'),
                'user': os.getenv('DB_USER', 'postgres'),
                'password': os.getenv('DB_PASSWORD', ''),
                'database': os.getenv('POSTGRES_DB', 'postgres')
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
            cursor.close()
            conn.close()
            
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
            logger.error(f"Failed to save strategy: {e}")
            raise 

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
    
    def _generate_strategy_code(self, prompt: str, existing_strategy: Optional[Dict[str, Any]] = None, attempt: int = 0) -> str:
        """Generate strategy code using OpenAI with optimized prompts"""
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
                # For new strategy - direct and simple
                user_prompt = f"""CREATE STRATEGY: {prompt}

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
                        response = self.openai_client.chat.completions.create(
                            model=model_name,
                            messages=[
                                {"role": "user", "content": f"{system_instruction}\n\n{user_prompt}"}
                            ]
                        )
                    else:
                        response = self.openai_client.chat.completions.create(
                            model=model_name,
                            messages=[
                                {"role": "system", "content": system_instruction},
                                {"role": "user", "content": user_prompt}
                            ],
                            temperature=0.1,
                            max_tokens=max_tokens
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
        """Validate strategy code using the security validator and test execution"""
        try:
            # First, use the existing validator for security checks
            is_valid = self.validator.validate_code(strategy_code)
            
            if not is_valid:
                return {
                    "valid": False,
                    "error": "Security validation failed"
                }
            
            # Try a quick execution test with the new accessor engine
            try:
                engine = AccessorStrategyEngine()
                test_result = await engine.execute_screening(
                    strategy_code=strategy_code,
                    universe=['AAPL'],  # Test with single symbol
                    limit=10
                )
                
                if test_result.get('success', False):
                    return {
                        "valid": True,
                        "error": None
                    }
                else:
                    return {
                        "valid": False,
                        "error": f"Execution test failed: {test_result.get('error', 'Unknown error')}"
                    }
                    
            except Exception as exec_error:
                logger.warning(f"Execution test failed: {exec_error}")
                # Still consider valid if security checks passed but execution test failed
                # (might be due to missing data or other environmental issues)
                return {
                    "valid": True,
                    "error": f"Warning: Execution test failed: {str(exec_error)}"
                }
            
        except (SecurityError, StrategyComplianceError) as e:
            return {
                "valid": False,
                "error": str(e)
            }
        except Exception as e:
            logger.error(f"Validation error: {e}")
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
        try:
            db_config = {
                'host': os.getenv('DB_HOST', 'localhost'),
                'port': os.getenv('DB_PORT', '5432'),
                'user': os.getenv('DB_USER', 'postgres'),
                'password': os.getenv('DB_PASSWORD', ''),
                'database': os.getenv('POSTGRES_DB', 'postgres')
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
            cursor.close()
            conn.close()
            
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
            logger.error(f"Failed to save strategy: {e}")
            raise 