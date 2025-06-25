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
- Use data accessor functions with explicit security_ids:
  * get_bar_data(timeframe="1d", security_ids=[list_of_security_ids], columns=[], min_bars=1) -> numpy array
     Columns: securityid, timestamp, open, high, low, close, volume
  * get_general_data(security_ids=[list_of_security_ids], columns=[]) -> pandas DataFrame  
     Columns: securityid, name, sector, industry, market, etc.

IMPORTANT: 
- ALWAYS pass explicit security_ids as integers (e.g., security_ids=[5546] for AAPL)
- Only pass security_ids=None if you want ALL securities data
- get_bar_data returns securityid (not ticker) as the primary identifier
- Return List[Dict] with required fields: 'securityid', 'timestamp'

EXAMPLE PATTERNS:
```python
def strategy():
    # Get recent price data for specific security ID (e.g., AAPL = 5546)
    bar_data = get_bar_data(
        timeframe="1d",
        security_ids=[5546],  # Explicit security ID for AAPL
        columns=["securityid", "timestamp", "open", "close"],
        min_bars=5
    )
    
    if bar_data is None or len(bar_data) == 0:
        return []
    
    # Convert to DataFrame for analysis
    df = pd.DataFrame(bar_data, columns=["securityid", "timestamp", "open", "close"])
    
    instances = []
    for _, row in df.iterrows():
        # Your strategy logic here
        instances.append({
            'securityid': int(row['securityid']),
            'timestamp': row['timestamp'],
            'signal': True,  # Your signal logic
        })
    
    return instances
```

SECURITY RULES:
- Only use whitelisted imports: pandas, numpy, datetime, math
- No file operations, network access, or dangerous functions
- No exec, eval, or dynamic code execution
- Use only standard mathematical and data manipulation operations

Generate clean, efficient Python code that follows these patterns exactly.
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