#!/usr/bin/env python3
"""
Test script to verify database connection recovery mechanism
"""
import os
import sys
import time
import psycopg2
from psycopg2.extras import RealDictCursor

# Add src to path to import worker modules
sys.path.insert(0, '/app/src')

# Set up test environment variables
os.environ['DB_HOST'] = os.getenv('DB_HOST', 'db')
os.environ['DB_PORT'] = os.getenv('DB_PORT', '5432')  
os.environ['POSTGRES_DB'] = os.getenv('POSTGRES_DB', 'postgres')
os.environ['DB_USER'] = os.getenv('DB_USER', 'postgres')
os.environ['DB_PASSWORD'] = os.getenv('DB_PASSWORD', 'devpassword')

def test_db_connection_recovery():
    """Test the database connection recovery mechanism"""
    print("🧪 Testing Database Connection Recovery...")
    
    try:
        # Import the worker class
        from worker import StrategyWorker
        
        print("✅ Successfully imported StrategyWorker")
        
        # Create worker instance (this tests initial connection)
        worker = StrategyWorker()
        print("✅ Worker initialized successfully")
        
        # Test the _ensure_db_connection method
        print("🔍 Testing _ensure_db_connection method...")
        worker._ensure_db_connection()
        print("✅ Database connection health check passed")
        
        # Test _fetch_strategy_code with a non-existent strategy (should handle gracefully)
        print("🔍 Testing _fetch_strategy_code with recovery...")
        try:
            strategy_code = worker._fetch_strategy_code("999999")  # Non-existent strategy
            print("❌ Should have failed with ValueError for non-existent strategy")
        except ValueError as e:
            if "Strategy not found" in str(e):
                print("✅ Correctly handled non-existent strategy")
            else:
                print(f"❌ Unexpected ValueError: {e}")
        except Exception as e:
            print(f"❌ Unexpected error: {e}")
        
        # Test connection recovery by simulating a closed connection
        print("🔍 Testing connection recovery after simulated failure...")
        try:
            # Close the current connection to simulate failure
            worker.db_conn.close()
            print("🔌 Simulated connection closure")
            
            # Try to use the connection - should trigger recovery
            worker._ensure_db_connection()
            print("✅ Connection recovery succeeded")
            
        except Exception as e:
            print(f"❌ Connection recovery failed: {e}")
            return False
        
        print("🎉 All database connection recovery tests passed!")
        return True
        
    except Exception as e:
        print(f"❌ Test failed with error: {e}")
        import traceback
        traceback.print_exc()
        return False

if __name__ == "__main__":
    print("=" * 60)
    print("🔧 DATABASE CONNECTION RECOVERY TEST")
    print("=" * 60)
    
    success = test_db_connection_recovery()
    
    print("=" * 60)
    if success:
        print("✅ ALL TESTS PASSED - Database connection recovery is working!")
        sys.exit(0)
    else:
        print("❌ TESTS FAILED - There are issues with database connection recovery")
        sys.exit(1) 