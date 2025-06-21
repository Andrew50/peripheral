#!/usr/bin/env python3
"""
Test script to create a strategy and verify logging
"""

import requests
import json

# Test strategy creation
def test_strategy_creation():
    url = "http://localhost:5058/private"
    
    # Test payload - create a simple NVDA gap up strategy
    payload = {
        "func": "createStrategyFromPrompt",
        "args": {
            "query": "create a strategy for when NVDA gaps up more than 2%",
            "strategyId": -1  # -1 means create new strategy
        }
    }
    
    headers = {
        "Content-Type": "application/json",
        # Note: In a real scenario, you'd need proper authentication
        # For testing, we'll assume the backend allows this request
        "Authorization": "Bearer dummy-token-for-testing"
    }
    
    print("Testing strategy creation...")
    print(f"URL: {url}")
    print(f"Payload: {json.dumps(payload, indent=2)}")
    
    try:
        response = requests.post(url, json=payload, headers=headers)
        print(f"\nResponse Status: {response.status_code}")
        print(f"Response Headers: {dict(response.headers)}")
        
        if response.status_code == 200:
            result = response.json()
            print(f"Success! Strategy created:")
            print(json.dumps(result, indent=2))
        else:
            print(f"Error: {response.status_code}")
            print(f"Response: {response.text}")
            
    except Exception as e:
        print(f"Request failed: {e}")

if __name__ == "__main__":
    test_strategy_creation() 