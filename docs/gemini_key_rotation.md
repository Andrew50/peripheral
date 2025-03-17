# Gemini API Key Rotation System

This document explains how to use the Gemini API key rotation system that has been integrated into our conn system. This system allows distributing API requests across multiple Gemini API keys, automatically handling rate limits and providing fallback to paid keys when necessary.

## Overview

The key rotation system:

1. Maintains a pool of API keys, categorized as free or paid
2. Tracks the number of requests made with each key in the last minute
3. Rotates through keys in a round-robin fashion to distribute load evenly
4. Falls back to paid keys only when all free keys have reached their rate limits
5. Automatically resets usage counters every minute
6. Is thread-safe and process-safe

## Configuration

### Environment Variables

Set these environment variables to configure the key pool:

| Variable | Description | Default |
|----------|-------------|---------|
| `GEMINI_FREE_KEYS` | Comma-separated list of free API keys | None |
| `GEMINI_PAID_KEY` | A single paid API key used as fallback | None |
| `GEMINI_FREE_RATE_LIMIT` | Rate limit per minute for free keys | 60 |
| `GEMINI_PAID_RATE_LIMIT` | Rate limit per minute for paid key | 1000 |

Example:
```bash
export GEMINI_FREE_KEYS="key1,key2,key3"
export GEMINI_PAID_KEY="paid-key"
```

## Usage

### Go Backend

```go
// 1. Get the connection instance (which already contains the key pool)
conn, cleanup := utils.InitConn(inContainer)
defer cleanup()

// 2. Create a Gemini client
geminiClient := utils.NewGeminiClient(conn)

// 3. Make requests using the client (keys are automatically rotated)
ctx := context.Background()
response, err := geminiClient.SimpleTextQuery(ctx, "Your prompt here")
if err != nil {
    // Handle error
}

// For more advanced usage:
req := &utils.GeminiRequest{
    Contents: []utils.GeminiContent{
        {
            Parts: []utils.GeminiPart{
                {
                    Text: "Your prompt here",
                },
            },
        },
    },
    // Optional configurations
    GenerationConfig: &utils.GeminiGenerationConfig{
        Temperature: utils.Float64Ptr(0.7),
        MaxOutputTokens: utils.IntPtr(1000),
    },
}
response, err := geminiClient.GenerateContent(ctx, req)
```

### Python Worker

```python
# 1. Get the connection instance (which already contains the key pool)
conn = Conn(inside_container=True)

# 2. Create a Gemini client
gemini = GeminiClient(conn)

# 3. Make requests using the client (keys are automatically rotated)
try:
    response = gemini.simple_query("Your prompt here")
except Exception as e:
    # Handle error
    pass

# For more advanced usage:
try:
    response = gemini.generate_content(
        prompt="Your prompt here",
        temperature=0.7,
        max_output_tokens=1000,
        top_p=0.9
    )
except Exception as e:
    # Handle error
    pass
```

## How It Works

### Key Selection Strategy

1. When a request is made, the system first tries to find an available free key that hasn't reached its rate limit.
2. If all free keys are at their limit, it falls back to the paid key.
3. If all keys (including the paid key) are at their limits, an error is returned.

### Rate Limit Tracking

- Each key maintains a counter of how many requests have been made in the current minute.
- A background goroutine/thread resets these counters every minute.
- This ensures we stay within the API limits for each key.

## Extending the System

### Adding Support for More Models

The current implementation focuses on the `gemini-pro` model. To support additional models:

1. Add new endpoint constants in the client classes
2. Create new methods that use those endpoints
3. The key rotation system will work automatically with any endpoint

### Monitoring and Metrics

Consider adding metrics tracking to monitor:

- Key usage distribution
- Error rates per key
- Rate limit hits
- Fallback frequency to paid keys

## Troubleshooting

### All Keys Reaching Limits

If you're receiving "all API keys have reached their rate limits" errors:

1. Check if your request volume is exceeding your combined key capacity
2. Consider adding more free keys to the pool
3. Implement backoff and retry strategies in your application
4. Consider upgrading to higher-tier API plans

### Rate Limit Errors

If you're still receiving rate limit errors from Gemini API despite using the rotation system:

1. Verify the actual rate limits for your API keys (they may be lower than default)
2. Adjust the `GEMINI_FREE_RATE_LIMIT` and `GEMINI_PAID_RATE_LIMIT` environment variables
3. Check if multiple processes are using the same keys without coordinating

## Performance Considerations

- The key rotation system adds minimal overhead to API requests.
- The memory footprint is negligible, even with hundreds of keys.
- Thread safety is ensured through mutexes/locks, allowing concurrent requests. 