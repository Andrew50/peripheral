#!/usr/bin/env python3
"""
Test script to verify strategy duplicate name handling
"""

import os
import psycopg2
from psycopg2.extras import RealDictCursor
from datetime import datetime

def test_duplicate_name_handling():
    """Test our duplicate name handling logic"""
    
    # Database connection
    db_config = {
        'host': os.getenv('DB_HOST', 'db'),
        'port': os.getenv('DB_PORT', '5432'),
        'user': os.getenv('DB_USER', 'postgres'),
        'password': os.getenv('DB_PASSWORD', ''),
        'database': os.getenv('POSTGRES_DB', 'postgres')
    }
    
    try:
        conn = psycopg2.connect(**db_config)
        cursor = conn.cursor(cursor_factory=RealDictCursor)
        
        # Test parameters
        user_id = 1
        base_name = "Aapl Gaps Up Strategy"
        
        print(f"Testing duplicate name handling for: {base_name}")
        
        # Check if name already exists
        cursor.execute("""
            SELECT COUNT(*) as count FROM strategies 
            WHERE userid = %s AND name = %s
        """, (user_id, base_name))
        count_result = cursor.fetchone()
        
        print(f"Existing strategies with name '{base_name}': {count_result['count']}")
        
        if count_result and count_result['count'] > 0:
            # Name exists, generate new name with timestamp
            timestamp_suffix = datetime.now().strftime("%m%d_%H%M%S")
            new_name = f"{base_name} ({timestamp_suffix})"
            print(f"Conflict detected! Generated new name: {new_name}")
        else:
            new_name = base_name
            print(f"No conflict, using original name: {new_name}")
        
        # Test the insert (but rollback to not actually save)
        try:
            cursor.execute("""
                INSERT INTO strategies (userid, name, description, prompt, pythoncode, 
                                      createdat, updated_at, isalertactive, score, version)
                VALUES (%s, %s, %s, %s, %s, NOW(), NOW(), false, 0, '1.0')
                RETURNING strategyid, name
            """, (user_id, new_name, "Test strategy", "AAPL gaps up 2%", "def strategy(): return []"))
            
            result = cursor.fetchone()
            print(f"✅ Successfully would create strategy: ID {result['strategyid']}, Name: {result['name']}")
            
            # Rollback to not actually save the test strategy
            conn.rollback()
            print("🔄 Rolled back test insert")
            
        except Exception as e:
            print(f"❌ Insert failed: {e}")
            conn.rollback()
        
        cursor.close()
        conn.close()
        
    except Exception as e:
        print(f"❌ Test failed: {e}")

if __name__ == "__main__":
    test_duplicate_name_handling() 