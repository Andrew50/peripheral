import logging
import traceback
import psycopg2
from typing import List, Dict, Tuple, Optional, Any
from .conn import Conn
from .context import Context
from psycopg2.extras import RealDictCursor

logger = logging.getLogger(__name__)


def fetch_strategy_code(ctx: Context, user_id: int, strategy_id: int, version: int = None) -> Tuple[str, int]:
    if not strategy_id:
        raise ValueError("strategy_id is required")

    with ctx.conn.transaction() as cursor:
        result = None


        if version is not None:
            cursor.execute(
                "SELECT pythonCode, version FROM strategies WHERE userId = %s AND strategyId = %s AND version = %s AND is_active = true",
                (user_id, strategy_id, version)
            )
            result = cursor.fetchone()

        # if version not given or that version not found
        if not result:
            cursor.execute(
                "SELECT pythonCode, version FROM strategies WHERE userId = %s AND strategyId = %s AND is_active = true ORDER BY version DESC LIMIT 1",
                (user_id, strategy_id)
            )
            result = cursor.fetchone()
            if version is not None and result:
                logger.warning(
                    "Requested version %s not found for strategy_id %s, using latest version %s",
                    version, strategy_id, result.get("version")
                )

        if not result:
            raise ValueError(f"Strategy not found for strategy_id: {strategy_id}")

        pythoncode = result.get('pythoncode')
        version_fetched = result.get('version')
        if not pythoncode:
            raise ValueError(f"Strategy has no Python code for strategy_id: {strategy_id}")

        return pythoncode, version_fetched


#TODO: add version
def fetch_multiple_strategy_codes(ctx: Context, user_id: int, strategy_ids: List[int]) -> Dict[int, str]:
    if not strategy_ids:
        raise ValueError("strategy_ids is required")

    if len(strategy_ids) != len(set(strategy_ids)):
        raise ValueError("strategy_ids must be unique")

    with ctx.conn.transaction() as cursor:
        cursor.execute(
            "SELECT strategyId, pythonCode FROM strategies WHERE userId = %s AND strategyId = ANY(%s) AND is_active = true",
            (user_id, strategy_ids)
        )
        rows = cursor.fetchall()
        strategy_codes = {
            row.get('strategyId'): row.get('pythonCode')
            for row in rows if row.get('pythonCode')
        }
        missing = set(strategy_ids) - set(strategy_codes.keys())
        if missing:
            logger.warning(
                "Strategies not found or missing Python code: %s",
                missing
            )
            raise ValueError(f"Strategies not found or missing Python code: {missing}")
        return strategy_codes


def save_strategy(ctx: Context, user_id: int, name: str, description: str, prompt: str, 
                        python_code: str, strategy_id: Optional[int] = None, min_timeframe: Optional[str] = None,
                        alert_universe_full: Optional[List[str]] = None) -> Dict[str, Any]:
    """Save strategy to database with duplicate name handling"""
    conn = None
    cursor = None
    try:
        
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
                raise ValueError(f"Strategy {strategy_id} not found for user {user_id}")
            
            strategy_name = version_result['name']
            next_version = version_result['next_version']
            
            
            # Insert new row with incremented version (preserves old version)
            cursor.execute("""
                INSERT INTO strategies (userid, name, description, prompt, pythoncode, 
                                        createdat, updated_at, alertactive, score, version, min_timeframe, alert_universe_full)
                VALUES (%s, %s, %s, %s, %s, NOW(), NOW(), false, 0, %s, %s, %s)
                RETURNING strategyid, name, description, prompt, pythoncode, 
                            createdat, updated_at, alertactive, version, min_timeframe, alert_universe_full
            """, (user_id, strategy_name, description, prompt, python_code, next_version, min_timeframe, alert_universe_full))
        else:
            # Create new strategy - always start at version 1
            
            cursor.execute("""
                INSERT INTO strategies (userid, name, description, prompt, pythoncode, 
                                        createdat, updated_at, alertactive, score, version, min_timeframe, alert_universe_full)
                VALUES (%s, %s, %s, %s, %s, NOW(), NOW(), false, 0, 1, %s, %s)
                RETURNING strategyid, name, description, prompt, pythoncode, 
                            createdat, updated_at, alertactive, version, min_timeframe, alert_universe_full
            """, (user_id, name, description, prompt, python_code, min_timeframe, alert_universe_full))
        
        result = cursor.fetchone()
        conn.commit()
        
        
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
                'isAlertActive': result['alertactive'],
                'minTimeframe': result['min_timeframe'],
                'alertUniverseFull': result['alert_universe_full']
            }
        raise ValueError("Failed to save strategy - no result returned")
            
    except Exception as e:
        logger.error("❌ Failed to save strategy: %s", e)
        logger.error("📄 Save strategy traceback: %s", traceback.format_exc())
        raise
    finally:
        # Ensure connections are properly cleaned up
        try:
            if cursor:
                cursor.close()
        except Exception as cleanup_error:
            logger.warning("⚠️ Error during database cleanup: %s", cleanup_error) 