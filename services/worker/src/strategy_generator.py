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
from src.validator import SecurityValidator, SecurityError, StrategyComplianceError

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
        """Get system instruction for strategy generation"""
        return """You are an expert Python developer and quantitative analyst specializing in trading strategy creation.

Your task is to generate Python trading strategy functions that take numpy arrays as input and return instances where patterns are found.

REQUIREMENTS:
1. Function must be named 'strategy(data)' where data is a numpy array
2. Function must return a list of dictionaries with 'ticker' and 'timestamp' fields  
3. Use only safe modules: numpy, pandas, math, datetime, statistics
4. No file I/O, network access, or dangerous operations
5. Data array contains: [ticker, date, open, high, low, close, volume, adj_close, fund_*]

EXAMPLE STRUCTURE:
```python
import numpy as np
from datetime import datetime

def strategy(data):
    instances = []
    
    # Your strategy logic here
    for i in range(len(data)):
        ticker = data[i, 0]  # ticker symbol
        date = data[i, 1]    # date string
        open_price = float(data[i, 2])
        high = float(data[i, 3])
        low = float(data[i, 4])
        close = float(data[i, 5])
        volume = float(data[i, 6])
        
        # Apply your pattern detection logic
        if your_condition_here:
            instances.append({
                'ticker': ticker,
                'timestamp': date,
                'signal': True,
                # Add any additional fields you want
            })
    
    return instances
```

Generate ONLY the Python function code. Do not include explanations or markdown formatting."""
    
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
            
            # Generate strategy code
            logger.info("Generating strategy code with OpenAI o3...")
            strategy_code = await self._generate_strategy_code(prompt, existing_strategy)
            
            if not strategy_code:
                return {
                    "success": False,
                    "error": "Failed to generate strategy code"
                }
            
            # Validate the generated code
            logger.info("Validating generated strategy code...")
            validation_result = await self._validate_strategy_code(strategy_code)
            
            if not validation_result["valid"]:
                # Try to regenerate with validation feedback
                logger.warning(f"Initial validation failed: {validation_result['error']}")
                retry_result = await self._retry_generation_with_feedback(prompt, validation_result["error"], existing_strategy)
                
                if not retry_result["success"]:
                    return retry_result
                
                strategy_code = retry_result["code"]
            
            # Extract description from the generated code or prompt
            description = self._extract_description(strategy_code, prompt)
            
            # Generate strategy name
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
                "validation_passed": True
            }
            
        except Exception as e:
            logger.error(f"Strategy creation failed: {e}")
            return {
                "success": False,
                "error": str(e)
            }
    
    async def _fetch_existing_strategy(self, user_id: int, strategy_id: int) -> Optional[Dict[str, Any]]:
        """Fetch existing strategy for editing"""
        try:
            db_config = {
                'host': os.getenv('DB_HOST', 'localhost'),
                'port': os.getenv('DB_PORT', '5432'),
                'user': os.getenv('DB_USER', 'postgres'),
                'password': os.getenv('DB_PASSWORD', ''),
                'database': 'atlantis'
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
    
    async def _generate_strategy_code(self, prompt: str, existing_strategy: Optional[Dict[str, Any]] = None) -> str:
        """Generate strategy code using OpenAI o3"""
        try:
            system_instruction = self._get_system_instruction()
            
            # Prepare the user prompt
            if existing_strategy:
                # Editing existing strategy
                user_prompt = f"""EDITING EXISTING STRATEGY:

Current Strategy Name: {existing_strategy.get('name', 'Unknown')}
Current Description: {existing_strategy.get('description', '')}
Original Prompt: {existing_strategy.get('prompt', '')}

Current Python Code:
```python
{existing_strategy.get('pythonCode', '')}
```

User's Edit Request: {prompt}

Please modify the existing strategy based on the user's edit request. You can:
1. Update the logic while keeping the same structure if the request is minor
2. Completely rewrite the strategy if the request requires major changes  
3. Add new functionality while preserving existing behavior where appropriate
4. Fix any bugs or improve performance if requested

Generate the updated Python strategy function named 'strategy(data)' that incorporates the requested changes."""
            else:
                # Creating new strategy
                user_prompt = f"""User Request: {prompt}

Please generate a Python strategy function that identifies the pattern the user is requesting. The function should be named 'strategy(data)' where data is a numpy array containing market data, and should return a list of instances where the pattern was found.

Focus on creating efficient, accurate pattern detection logic that matches the user's requirements."""
            
            # Call OpenAI o3
            response = self.openai_client.chat.completions.create(
                model="o3-mini",  # Using o3-mini as it's available and more cost-effective
                messages=[
                    {"role": "system", "content": system_instruction},
                    {"role": "user", "content": user_prompt}
                ],
                temperature=0.1,  # Low temperature for consistent code generation
                max_tokens=2000
            )
            
            strategy_code = response.choices[0].message.content.strip()
            
            # Extract Python code from response (remove any markdown formatting)
            strategy_code = self._extract_python_code(strategy_code)
            
            logger.info(f"Generated strategy code ({len(strategy_code)} characters)")
            return strategy_code
            
        except Exception as e:
            logger.error(f"OpenAI code generation failed: {e}")
            return ""
    
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
        """Validate strategy code using the security validator"""
        try:
            # Use the existing validator
            is_valid = self.validator.validate_code(strategy_code)
            
            return {
                "valid": is_valid,
                "error": None
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
    
    async def _retry_generation_with_feedback(self, original_prompt: str, validation_error: str, existing_strategy: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        """Retry strategy generation with validation feedback"""
        try:
            logger.info(f"Retrying generation with validation feedback: {validation_error}")
            
            # Enhanced prompt with validation feedback
            enhanced_prompt = f"""{original_prompt}

IMPORTANT: The previous attempt failed validation with this error:
{validation_error}

Please fix this issue and ensure the code follows all security and compliance requirements:
- Only use allowed modules (numpy, pandas, math, datetime, statistics)
- Function must be named 'strategy(data)'
- Must return List[Dict] with 'ticker' and 'timestamp' fields
- No file I/O, network access, or dangerous operations
- Proper error handling for edge cases"""
            
            # Generate new code with feedback
            strategy_code = await self._generate_strategy_code(enhanced_prompt, existing_strategy)
            
            if not strategy_code:
                return {
                    "success": False,
                    "error": "Failed to regenerate strategy code"
                }
            
            # Validate the retry
            validation_result = await self._validate_strategy_code(strategy_code)
            
            if validation_result["valid"]:
                return {
                    "success": True,
                    "code": strategy_code
                }
            else:
                return {
                    "success": False,
                    "error": f"Retry validation failed: {validation_result['error']}"
                }
                
        except Exception as e:
            logger.error(f"Retry generation failed: {e}")
            return {
                "success": False,
                "error": f"Retry failed: {str(e)}"
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
        prompt_words = prompt.split()[:10]  # First 10 words
        return f"Strategy based on: {' '.join(prompt_words)}{'...' if len(prompt.split()) > 10 else ''}"
    
    def _generate_strategy_name(self, prompt: str, is_edit: bool, existing_strategy: Optional[Dict[str, Any]] = None) -> str:
        """Generate a unique strategy name"""
        if is_edit and existing_strategy:
            # For edits, keep the original name with a version indicator
            original_name = existing_strategy.get('name', 'Strategy')
            timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
            return f"{original_name} (Updated {timestamp})"
        
        # For new strategies, generate from prompt
        words = prompt.split()[:4]  # First 4 words
        clean_words = []
        
        skip_words = {'create', 'a', 'an', 'the', 'strategy', 'for', 'when', 'find', 'identify'}
        
        for word in words:
            clean_word = re.sub(r'[^a-zA-Z0-9]', '', word)
            if clean_word.lower() not in skip_words and len(clean_word) > 1:
                clean_words.append(clean_word.title())
        
        if not clean_words:
            clean_words = ['Custom']
        
        # Add timestamp for uniqueness
        timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
        return f"{' '.join(clean_words)} Strategy {timestamp}"
    
    async def _save_strategy(self, user_id: int, name: str, description: str, prompt: str, 
                           python_code: str, strategy_id: Optional[int] = None) -> Dict[str, Any]:
        """Save strategy to database"""
        try:
            db_config = {
                'host': os.getenv('DB_HOST', 'localhost'),
                'port': os.getenv('DB_PORT', '5432'),
                'user': os.getenv('DB_USER', 'postgres'),
                'password': os.getenv('DB_PASSWORD', ''),
                'database': 'atlantis'
            }
            
            conn = psycopg2.connect(**db_config)
            cursor = conn.cursor(cursor_factory=RealDictCursor)
            
            if strategy_id:
                # Update existing strategy
                cursor.execute("""
                    UPDATE strategies 
                    SET name = %s, description = %s, prompt = %s, pythoncode = %s, 
                        updatedat = NOW()
                    WHERE strategyid = %s AND userid = %s
                    RETURNING strategyid, name, description, prompt, pythoncode, 
                             createdat, updatedat, isalertactive
                """, (name, description, prompt, python_code, strategy_id, user_id))
            else:
                # Create new strategy
                cursor.execute("""
                    INSERT INTO strategies (userid, name, description, prompt, pythoncode, 
                                          createdat, updatedat, isalertactive, score, version)
                    VALUES (%s, %s, %s, %s, %s, NOW(), NOW(), false, 0, '1.0')
                    RETURNING strategyid, name, description, prompt, pythoncode, 
                             createdat, updatedat, isalertactive
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
                    'updatedAt': result['updatedat'].isoformat() if result['updatedat'] else None,
                    'isAlertActive': result['isalertactive']
                }
            else:
                raise Exception("Failed to save strategy - no result returned")
                
        except Exception as e:
            logger.error(f"Failed to save strategy: {e}")
            raise 