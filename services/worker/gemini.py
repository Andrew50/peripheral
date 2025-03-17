import requests
import time
import threading
from datetime import datetime, timedelta
import os

class GeminiKeyInfo:
    """Information about a Gemini API key including usage metrics."""
    def __init__(self, key, is_paid, rate_limit):
        self.key = key
        self.is_paid = is_paid
        self.request_count = 0
        self.last_reset = datetime.now()
        self.rate_limit_total = rate_limit
        self.lock = threading.Lock()


class GeminiKeyPool:
    """Manages a pool of Gemini API keys with rotation and rate limiting."""
    def __init__(self):
        self.keys = []
        self.lock = threading.RLock()
        self.last_used_idx = -1
        
        # Get keys from environment variables
        free_keys_str = os.environ.get("GEMINI_FREE_KEYS", "")
        paid_key = os.environ.get("GEMINI_PAID_KEY", "")
        free_rate_limit = 15 
        paid_rate_limit = 1000

        # Add free keys
        if free_keys_str:
            free_keys = [k.strip() for k in free_keys_str.split(',') if k.strip()]
            for key in free_keys:
                self.keys.append(GeminiKeyInfo(key, False, free_rate_limit))
        
        # Add paid key
        if paid_key:
            self.keys.append(GeminiKeyInfo(paid_key, True, paid_rate_limit))
            
        # Start a background thread to reset counts every minute
        self._start_reset_thread()
    
    def _start_reset_thread(self):
        """Start a thread to reset request counts every minute."""
        def reset_loop():
            while True:
                time.sleep(60)  # Sleep for 1 minute
                self.reset_counts()
        
        thread = threading.Thread(target=reset_loop, daemon=True)
        thread.start()
    
    def reset_counts(self):
        """Reset request counts for all keys that have exceeded their minute."""
        with self.lock:
            now = datetime.now()
            for key_info in self.keys:
                with key_info.lock:
                    # Only reset if a minute has passed
                    if (now - key_info.last_reset).total_seconds() >= 60:
                        key_info.request_count = 0
                        key_info.last_reset = now
    
    def get_next_key(self):
        """Get the next available API key based on rotation strategy."""
        with self.lock:
            if not self.keys:
                raise ValueError("No API keys available in the pool")
            
            # First try to find a non-paid key under the rate limit
            for i in range(len(self.keys)):
                # Round-robin selection starting after the last used index
                idx = (self.last_used_idx + 1 + i) % len(self.keys)
                key_info = self.keys[idx]
                
                # Skip paid keys on the first pass
                if key_info.is_paid:
                    continue
                
                with key_info.lock:
                    # Check if this key is under its rate limit
                    if key_info.request_count < key_info.rate_limit_total:
                        key_info.request_count += 1
                        self.last_used_idx = idx
                        return key_info.key
            
            # If all free keys are at their limit, try paid keys
            for i in range(len(self.keys)):
                idx = (self.last_used_idx + 1 + i) % len(self.keys)
                key_info = self.keys[idx]
                
                # Only consider paid keys now
                if not key_info.is_paid:
                    continue
                
                with key_info.lock:
                    # Check if this paid key is under its rate limit
                    if key_info.request_count < key_info.rate_limit_total:
                        key_info.request_count += 1
                        self.last_used_idx = idx
                        return key_info.key
            
            # If we get here, all keys (including paid ones) are at their rate limit
            raise ValueError("All API keys have reached their rate limits")

def gemini_query(conn, query, system_instruction, model_name="gemini-2.0-flash-001", max_tokens=8192):
    # API endpoint
    API_KEY = conn.get_gemini_key()
    if not API_KEY: 
        print("No available Gemini API keys")
        return None
    url = f"https://generativelanguage.googleapis.com/v1beta/models/{model_name}:generateContent?key={API_KEY}"

    # Headers
    headers = {
        "Content-Type": "application/json"
    }

    # Payload data
    payload = {
        "contents": [
            {
                "role": "user",
                "parts": [
                    {
                        "text": query
                    }
                ]
            }
        ],
        "systemInstruction": {
            "role": "user",
            "parts": [
                {
                    "text": system_instruction
                }
            ]
        },
        "generationConfig": {
            "temperature": 1,
            "topK": 40,
            "topP": 0.95,
            "maxOutputTokens": max_tokens,
            "responseMimeType": "text/plain"
        }
    }

    # Make the POST request
    response = requests.post(url, headers=headers, json=payload)

    # Print the response
    print(response.status_code)
    print(response.text)

    # If you want to work with the JSON response
    if response.status_code == 200:
        # Process the response as needed
        # For example, if you want to extract the generated text:
        # generated_text = result.get("candidates", [{}])[0].get("content", {}).get("parts", [{}])[0].get("text", "")
        # print(generated_text)
        return response.text
    
    return None