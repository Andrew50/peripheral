import requests
import json


# Your API key
API_KEY = "YOUR_API_KEY"

def gemini_query(query, system_instruction, model_name="gemini-2.0-flash-001", max_tokens=8192):


    # API endpoint
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
        result = response.json()
        # Process the result as needed
        # For example, if you want to extract the generated text:
        # generated_text = result.get("candidates", [{}])[0].get("content", {}).get("parts", [{}])[0].get("text", "")
        # print(generated_text)
