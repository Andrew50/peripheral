import logging
import traceback
import psycopg2
from typing import List, Dict, Tuple, Optional, Any
from .conn import Conn
from .context import Context
from psycopg2.extras import RealDictCursor

logger = logging.getLogger(__name__)


def fetch_strategy_code(ctx: Context, userId: int, strategyId: int, version: int = None) -> Tuple[str, int]:
    if not strategyId:
        raise ValueError("strategyId is required")

    with ctx.conn.transaction() as cursor:
        result = None


        if version is not None:
            cursor.execute(
                "SELECT pythonCode, version FROM strategies WHERE userId = %s AND strategyId = %s AND version = %s AND is_active = true",
                (userId, strategyId, version)
            )
            result = cursor.fetchone()

        # if version not given or that version not found
        if not result:
            cursor.execute(
                "SELECT pythonCode, version FROM strategies WHERE userId = %s AND strategyId = %s AND is_active = true ORDER BY version DESC LIMIT 1",
                (userId, strategyId)
            )
            result = cursor.fetchone()
            if version is not None and result:
                logger.warning(f"Requested version {version} not found for strategy_id {strategyId}, using latest version {result.get('version')}")

        if not result:
            raise ValueError(f"Strategy not found for strategyId: {strategyId}")

        pythoncode = result.get('pythoncode')
        version_fetched = result.get('version')
        if not pythoncode:
            raise ValueError(f"Strategy has no Python code for strategyId: {strategyId}")

        return pythoncode, version_fetched


#TODO: add version
def fetch_multiple_strategy_codes(ctx: Context, userId: int, strategyIds: List[int]) -> Dict[int, str]:
    if not strategyIds:
        raise ValueError("strategy_ids is required")

    if len(strategyIds) != len(set(strategyIds)):
        raise ValueError("strategy_ids must be unique")

    with ctx.conn.transaction() as cursor:
        cursor.execute(
            "SELECT strategyId, pythonCode FROM strategies WHERE userId = %s AND strategyId = ANY(%s) AND is_active = true",
            (userId, strategyIds)
        )
        rows = cursor.fetchall()
        strategy_codes = {
            row.get('strategyId'): row.get('pythonCode')
            for row in rows if row.get('pythonCode')
        }
        missing = set(strategyIds) - set(strategy_codes.keys())
        if missing:
            logger.warning(f"Strategies not found or missing Python code: {missing}")
            raise ValueError(f"Strategies not found or missing Python code: {missing}")
        return strategy_codes


'''async def _fetch_existing_strategy(ctx: Context, user_id: int, strategy_id: int) -> Optional[Dict[str, Any]]:
    """Fetch existing strategy for editing"""
    conn = None
    cursor = None
    try:
        #logger.info(f"📖 Fetching existing strategy (user_id: {user_id}, strategy_id: {strategy_id})")
        
        # Use connection manager if available, otherwise fallback to direct connection
        ctx.conn.ensure_db_connection()
        conn = ctx.conn.db_conn
        cursor = conn.cursor(cursor_factory=RealDictCursor)
                    
        cursor.execute("""
            SELECT strategyid, name, description, prompt, pythoncode
            FROM strategies 
            WHERE strategyid = %s AND userid = %s
        """, (strategy_id, user_id))
        
        result = cursor.fetchone()
        
        if result:
            #logger.info(f"✅ Found existing strategy: {result['name']}")
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
        # Ensure connections are properly cleaned up
        try:
            if cursor:
                cursor.close()
                logger.debug("🔌 Database cursor closed")
            # Only close connection if not using connection manager
            #if conn and not self.conn:
                #conn.close()
                #logger.debug("🔌 Database connection closed")
        except Exception as cleanup_error:
            logger.warning(f"⚠️ Error during database cleanup: {cleanup_error}")

'''
def save_strategy(ctx: Context, user_id: int, name: str, description: str, prompt: str, 
                        python_code: str, strategy_id: Optional[int] = None) -> Dict[str, Any]:
    """Save strategy to database with duplicate name handling"""
    conn = None
    cursor = None
    try:
        #logger.info(f"💾 Saving strategy to database (user_id: {user_id}, strategy_id: {strategy_id})")
        
        # Use connection manager if available, otherwise fallback to direct connection
        ctx.conn.ensure_db_connection()
        conn = ctx.conn.db_conn
        cursor = conn.cursor(cursor_factory=RealDictCursor)
        
        if strategy_id:
            # Create new version of existing strategy - preserve old version as separate row
            # First get the existing strategy data
            cursor.execute("""
                SELECT name, COALESCE(MAX(version), 0) + 1 as next_version
                FROM strategies 
                WHERE userid = %s AND name = (SELECT name FROM strategies WHERE strategyid = %s AND userid = %s)
                GROUP BY name
            """, (user_id, strategy_id, user_id))
            version_result = cursor.fetchone()
            
            if not version_result:
                raise Exception(f"Strategy {strategy_id} not found for user {user_id}")
            
            strategy_name = version_result['name']
            next_version = version_result['next_version']
            
            #logger.info(f"Creating new version {next_version} of strategy '{strategy_name}' for user {user_id}")
            
            # Insert new row with incremented version (preserves old version)
            cursor.execute("""
                INSERT INTO strategies (userid, name, description, prompt, pythoncode, 
                                        createdat, updated_at, alertactive, score, version)
                VALUES (%s, %s, %s, %s, %s, NOW(), NOW(), false, 0, %s)
                RETURNING strategyid, name, description, prompt, pythoncode, 
                            createdat, updated_at, alertactive, version
            """, (user_id, strategy_name, description, prompt, python_code, next_version))
        else:
            # Create new strategy - always start at version 1
            #logger.info(f"Creating new strategy '{name}' version 1 for user {user_id}")
            
            cursor.execute("""
                INSERT INTO strategies (userid, name, description, prompt, pythoncode, 
                                        createdat, updated_at, alertactive, score, version)
                VALUES (%s, %s, %s, %s, %s, NOW(), NOW(), false, 0, 1)
                RETURNING strategyid, name, description, prompt, pythoncode, 
                            createdat, updated_at, alertactive, version
            """, (user_id, name, description, prompt, python_code))
        
        result = cursor.fetchone()
        conn.commit()
        
        #logger.info(f"✅ Strategy saved successfully with ID: {result['strategyid'] if result else 'None'}")
        
        if result:
            return {
                'strategyId': result['strategyid'],
                'userId': user_id,
                'name': result['name'],
                'description': result['description'],
                'prompt': result['prompt'],
                'pythonCode': result['pythoncode'],
                'version': result['version'],
                'createdAt': result['createdat'].isoformat() if result['createdat'] else None,
                'updatedAt': result['updated_at'].isoformat() if result['updated_at'] else None,
                'isAlertActive': result['alertactive']
            }
        else:
            raise Exception("Failed to save strategy - no result returned")
            
    except Exception as e:
        logger.error("❌ Failed to save strategy: %s", e)
        logger.error("📄 Save strategy traceback: %s", traceback.format_exc())
        raise
    finally:
        # Ensure connections are properly cleaned up
        try:
            if cursor:
                cursor.close()
                logger.debug("🔌 Database cursor closed")
            # Only close connection if not using connection manager
            #if conn and not self.conn:
                #conn.close()
                #logger.debug("🔌 Database connection closed")
        except Exception as cleanup_error:
            logger.warning("⚠️ Error during database cleanup: %s", cleanup_error) 