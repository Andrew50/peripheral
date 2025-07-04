#!/usr/bin/env python3
"""
Utility script to clean up stale worker heartbeats from Redis
"""

import redis
import json
import os
from datetime import datetime, timedelta
import sys

def cleanup_stale_heartbeats():
    """Clean up stale worker heartbeats from Redis"""
    
    # Connect to Redis
    redis_host = os.environ.get("REDIS_HOST", "cache")
    redis_port = int(os.environ.get("REDIS_PORT", "6379"))
    redis_password = os.environ.get("REDIS_PASSWORD", "")
    
    client = redis.Redis(
        host=redis_host,
        port=redis_port,
        password=redis_password if redis_password else None,
        decode_responses=True
    )
    
    try:
        # Test connection
        client.ping()
        print(f"✅ Connected to Redis at {redis_host}:{redis_port}")
        
        # Get all heartbeat keys
        heartbeat_keys = client.keys("worker_heartbeat:*")
        print(f"🔍 Found {len(heartbeat_keys)} worker heartbeat keys")
        
        if not heartbeat_keys:
            print("✅ No heartbeat keys to clean up")
            return
        
        current_time = datetime.utcnow()
        stale_threshold = timedelta(minutes=5)  # Consider heartbeats older than 5 minutes as stale
        
        cleaned_count = 0
        active_count = 0
        
        for key in heartbeat_keys:
            try:
                heartbeat_data = client.get(key)
                if not heartbeat_data:
                    # Key expired or doesn't exist
                    continue
                
                heartbeat = json.loads(heartbeat_data)
                worker_id = heartbeat.get("worker_id", "unknown")
                timestamp_str = heartbeat.get("timestamp", "")
                
                # Parse timestamp
                try:
                    # Try RFC3339 format with Z suffix
                    if timestamp_str.endswith('Z'):
                        heartbeat_time = datetime.fromisoformat(timestamp_str[:-1])
                    else:
                        heartbeat_time = datetime.fromisoformat(timestamp_str)
                except ValueError:
                    print(f"⚠️ Invalid timestamp format for {worker_id}: {timestamp_str}")
                    # Delete keys with invalid timestamps
                    client.delete(key)
                    cleaned_count += 1
                    continue
                
                time_since_heartbeat = current_time - heartbeat_time
                
                if time_since_heartbeat > stale_threshold:
                    print(f"🧹 Cleaning stale heartbeat: {worker_id} (last seen {time_since_heartbeat} ago)")
                    client.delete(key)
                    cleaned_count += 1
                else:
                    print(f"✅ Active worker: {worker_id} (last seen {time_since_heartbeat} ago)")
                    active_count += 1
                    
            except Exception as e:
                print(f"❌ Error processing key {key}: {e}")
                # Clean up problematic keys
                client.delete(key)
                cleaned_count += 1
        
        print(f"\n📊 Cleanup Summary:")
        print(f"   🧹 Cleaned up: {cleaned_count} stale heartbeats")
        print(f"   ✅ Active workers: {active_count}")
        print(f"   📈 Total processed: {len(heartbeat_keys)}")
        
    except Exception as e:
        print(f"❌ Redis connection or operation failed: {e}")
        sys.exit(1)

def cleanup_all_heartbeats():
    """Clean up ALL worker heartbeats (use with caution)"""
    
    redis_host = os.environ.get("REDIS_HOST", "cache")
    redis_port = int(os.environ.get("REDIS_PORT", "6379"))
    redis_password = os.environ.get("REDIS_PASSWORD", "")
    
    client = redis.Redis(
        host=redis_host,
        port=redis_port,
        password=redis_password if redis_password else None,
        decode_responses=True
    )
    
    try:
        client.ping()
        heartbeat_keys = client.keys("worker_heartbeat:*")
        
        if heartbeat_keys:
            deleted_count = client.delete(*heartbeat_keys)
            print(f"🧹 Forcefully cleaned up {deleted_count} heartbeat keys")
        else:
            print("✅ No heartbeat keys found")
            
    except Exception as e:
        print(f"❌ Cleanup failed: {e}")
        sys.exit(1)

if __name__ == "__main__":
    print("Redis Heartbeat Cleanup Utility")
    print("=" * 40)
    
    if len(sys.argv) > 1 and sys.argv[1] == "--force":
        print("⚠️ FORCE MODE: Cleaning ALL heartbeats")
        response = input("Are you sure? This will remove all worker heartbeats. (yes/no): ")
        if response.lower() == "yes":
            cleanup_all_heartbeats()
        else:
            print("❌ Cancelled")
    else:
        print("🔍 Cleaning stale heartbeats only")
        cleanup_stale_heartbeats()
    
    print("✅ Cleanup complete") 