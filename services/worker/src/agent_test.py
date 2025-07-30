#!/usr/bin/env python3
"""
Basic Agent Test Script
Tests the validation and execution of Python code using actual Conn and Context objects
"""

import logging
import time
import asyncio
from datetime import datetime
from datetime import timedelta

# Import the actual classes
from utils.conn import Conn
from utils.context import Context
from validator import validate_code
from sandbox import PythonSandbox, create_default_config

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# The Python code to test
TEST_CODE = '''from datetime import datetime as dt, timedelta
import pandas as pd
import plotly.graph_objects as go

def code():
    # Fetch daily bar data for all tickers
    target_date = dt(2023,7,15).date()  # 2023-07-15
    start_dt = dt(2023,7,15)
    end_dt = start_dt + timedelta(days=1)
    df = get_bar_data(timeframe="1d", columns=["ticker", "timestamp", "open", "close"], min_bars=1, filters={}, start_date=start_dt, end_date=end_dt)
    if df is None or len(df) == 0:
        print("No daily bar data available.")
        return []

    # Convert timestamp to datetime and filter by target date
    df["dt"] = pd.to_datetime(df["timestamp"], unit="s")
    df["date"] = df["dt"].dt.date
    df_date = df[df["date"] == target_date]
    if df_date.empty:
        print(f"No data for date {target_date.strftime('%d/%m/%Y')}.")
        return []

    # Calculate percentage change
    df_date["open"] = pd.to_numeric(df_date["open"], errors="coerce")
    df_date["close"] = pd.to_numeric(df_date["close"], errors="coerce")
    df_date = df_date.dropna(subset=["open", "close"])
    df_date = df_date[df_date["open"] != 0]
    df_date["change_pct"] = (df_date["close"] - df_date["open"]) / df_date["open"]

    # Count stocks down 4% or more
    down_mask = df_date["change_pct"] <= -0.04
    count_down = int(down_mask.sum())
    count_other = int((~down_mask).sum())

    # Print results
    print(f"Date: {target_date.strftime('%d/%m/%Y')}")
    print(f"Total stocks analyzed: {len(df_date)}")
    print(f"Stocks down 4% or more: {count_down}")

    # Prepare bar chart
    categories = ["Down ≥ 4%", "Other"]
    counts = [count_down, count_other]
    colors = ["red", "grey"]

    fig = go.Figure(data=[
        go.Bar(
            x=categories,
            y=counts,
            marker_color=colors,
            name="Count of Stocks"
        )
    ])
    fig.update_layout(
        title="[ALL] Stocks Down ≥4% on 07/15/2023",
        xaxis_title="Category",
        yaxis_title="Number of Stocks",
        template="plotly_white"
    )
    fig.show()

    return []'''

async def run_test():
    """Run the test with actual Conn and Context objects"""
    
    print("🚀 Starting Agent Test...")
    print("=" * 50)
    
    # Step 1: Create connections
    print("📡 Initializing connections...")
    try:
        conn = Conn()
        print("✅ Connections initialized successfully")
    except Exception as e:
        print(f"❌ Failed to initialize connections: {e}")
        return
    
    # Step 2: Create context
    print("🌐 Creating context...")
    try:
        # Create context with test parameters
        execution_serial = int(time.time())
        task_id = f"test_task_{execution_serial}"
        status_id = f"test_status_{execution_serial}"
        
        ctx = Context(
            conn=conn,
            task_id=task_id,
            status_id=status_id,
            heartbeat_interval=30,
            queue_type="test",
            priority="normal",
            worker_id="test_worker",
            skip_heartbeat=True  # Skip heartbeat for test
        )
        print("✅ Context created successfully")
    except Exception as e:
        print(f"❌ Failed to create context: {e}")
        return
    
    # Step 3: Validate code
    print("🔍 Validating Python code...")
    try:
        is_valid = validate_code(TEST_CODE, allow_none_return=True, allowed_entrypoints={"code"})
        if is_valid:
            print("✅ Code validation passed")
        else:
            print("❌ Code validation failed")
            return
    except Exception as e:
        print(f"❌ Code validation error: {e}")
        return
    
    # Step 4: Execute code
    print("⚡ Executing Python code...")
    try:
        # Create sandbox with unique execution ID
        execution_id = f"test_{execution_serial}"
        python_sandbox = PythonSandbox(create_default_config(), execution_id=execution_id)
        
        # Execute the code
        result = await python_sandbox.execute_code(ctx, TEST_CODE)
        
        # Check results
        if result.success:
            print("✅ Code execution successful!")
            print("=" * 50)
            print("📊 RESULTS:")
            print("=" * 50)
            
            if result.result:
                print(f"Return value: {result.result}")
            
            if result.prints:
                print("📝 Printed output:")
                print(result.prints)
            
            if result.plots:
                print(f"📈 Generated {len(result.plots)} plot(s)")
                for i, plot in enumerate(result.plots):
                    print(f"  Plot {i+1}: {plot.get('title', 'Untitled')}")
            
            if result.response_images:
                print(f"🖼️ Generated {len(result.response_images)} image(s)")
                
        else:
            print("❌ Code execution failed!")
            print(f"Error: {result.error}")
            
            if result.error_details:
                print("Error details:")
                for key, value in result.error_details.items():
                    print(f"  {key}: {value}")
    
    except Exception as e:
        print(f"❌ Execution error: {e}")
        logger.error("Execution error", exc_info=True)
    
    finally:
        # Step 5: Cleanup
        print("🧹 Cleaning up...")
        try:
            ctx.destroy()
            conn.close_connections()
            print("✅ Cleanup completed")
        except Exception as e:
            print(f"⚠️ Cleanup warning: {e}")

def main():
    """Main entry point"""
    try:
        asyncio.run(run_test())
    except KeyboardInterrupt:
        print("\n⏹️ Test interrupted by user")
    except Exception as e:
        print(f"❌ Test failed with error: {e}")
        logger.error("Test failed", exc_info=True)

if __name__ == "__main__":
    main() 