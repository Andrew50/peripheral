package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"backend/internal/data"

	"github.com/go-redis/redis/v8"
)

type RateLimitConfig struct {
	RequestsPerMinute int
	WindowSeconds     int
}

type RateLimiter struct {
	redis  *redis.Client
	config map[string]RateLimitConfig
}

func NewRateLimiter(conn *data.Conn) *RateLimiter {
	return &RateLimiter{
		redis: conn.Cache,
		config: map[string]RateLimitConfig{
			// Public endpoint limits (IP-based)
			"public:default":                 {RequestsPerMinute: 60, WindowSeconds: 60},
			"public:login":                   {RequestsPerMinute: 5, WindowSeconds: 60},
			"public:signup":                  {RequestsPerMinute: 5, WindowSeconds: 60},
			"public:googleLogin":             {RequestsPerMinute: 10, WindowSeconds: 60},
			"public:googleCallback":          {RequestsPerMinute: 10, WindowSeconds: 60},
			"public:getSecuritiesFromTicker": {RequestsPerMinute: 2000, WindowSeconds: 60},
			"public:getPopularTickers":       {RequestsPerMinute: 2000, WindowSeconds: 60},
			"public:getPublicConversation":   {RequestsPerMinute: 60, WindowSeconds: 60},
			"public:getConversationSnippet":  {RequestsPerMinute: 10000, WindowSeconds: 60},

			// Private endpoint limits (user-based)
			"private:default":  {RequestsPerMinute: 5000, WindowSeconds: 60},
			"private:chat":     {RequestsPerMinute: 1000, WindowSeconds: 60},
			"private:backtest": {RequestsPerMinute: 1000, WindowSeconds: 60},
			"private:upload":   {RequestsPerMinute: 1000, WindowSeconds: 60},
		},
	}
}

func (rl *RateLimiter) CheckRateLimit(ctx context.Context, identifier, function string, isPublic bool) (*RateLimitResult, error) {
	var configKey string
	if isPublic {
		if _, exists := rl.config["public:"+function]; exists {
			configKey = "public:" + function
		} else {
			configKey = "public:default"
		}
	} else {
		// Check for function-specific private limits
		if strings.Contains(function, "chat") || strings.Contains(function, "Chat") {
			configKey = "private:chat"
		} else if strings.Contains(function, "backtest") || strings.Contains(function, "Backtest") {
			configKey = "private:backtest"
		} else if function == "upload" {
			configKey = "private:upload"
		} else {
			configKey = "private:default"
		}
	}

	config := rl.config[configKey]
	return rl.slidingWindowRateLimit(ctx, identifier, function, config)
}

type RateLimitResult struct {
	Allowed       bool
	Remaining     int
	ResetTime     time.Time
	RetryAfter    int
	TotalRequests int
}

func (rl *RateLimiter) slidingWindowRateLimit(ctx context.Context, identifier, function string, config RateLimitConfig) (*RateLimitResult, error) {
	now := time.Now()
	windowStart := now.Add(-time.Duration(config.WindowSeconds) * time.Second)

	// Redis key for this identifier and function
	key := fmt.Sprintf("ratelimit:%s:%s", identifier, function)

	// Use Redis pipeline for atomic operations
	pipe := rl.redis.Pipeline()

	// Remove expired entries
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%.0f", float64(windowStart.Unix())))

	// Count current requests in window
	countCmd := pipe.ZCard(ctx, key)

	// Add current request
	pipe.ZAdd(ctx, key, &redis.Z{
		Score:  float64(now.Unix()),
		Member: fmt.Sprintf("%d", now.UnixNano()),
	})

	// Set expiration
	pipe.Expire(ctx, key, time.Duration(config.WindowSeconds)*time.Second)

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		// If Redis fails, allow the request but log the error
		return &RateLimitResult{
			Allowed:       true,
			Remaining:     config.RequestsPerMinute,
			ResetTime:     now.Add(time.Duration(config.WindowSeconds) * time.Second),
			RetryAfter:    0,
			TotalRequests: 0,
		}, nil
	}

	currentCount := int(countCmd.Val())

	// Check if limit exceeded
	allowed := currentCount <= config.RequestsPerMinute
	remaining := config.RequestsPerMinute - currentCount
	if remaining < 0 {
		remaining = 0
	}

	resetTime := now.Add(time.Duration(config.WindowSeconds) * time.Second)
	retryAfter := 0
	if !allowed {
		retryAfter = config.WindowSeconds
	}

	return &RateLimitResult{
		Allowed:       allowed,
		Remaining:     remaining,
		ResetTime:     resetTime,
		RetryAfter:    retryAfter,
		TotalRequests: currentCount,
	}, nil
}

func (rl *RateLimiter) RateLimitMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add CORS headers first, before any potential early returns
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

			// Skip rate limiting for health checks
			if r.URL.Path == "/healthz" {
				next.ServeHTTP(w, r)
				return
			}

			// Skip rate limiting for OPTIONS requests (CORS preflight)
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			// Determine if this is a public or private endpoint
			isPublic := strings.HasPrefix(r.URL.Path, "/public")

			// Get identifier for rate limiting
			var identifier string
			var function string

			if isPublic {
				// For public endpoints, use IP address
				identifier = getClientIP(r)

				// Extract function from request body if possible
				if r.Method == "POST" {
					function = extractFunctionFromRequest(r)
				}
				if function == "" {
					function = "default"
				}
			} else {
				// For private endpoints, extract user ID from JWT token
				userID := getUserIDFromRequest(r)
				if userID == -1 {
					// Reject immediately - no valid auth for private endpoint
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
					w.Write([]byte(`{"error": "Authentication required"}`))
					return
				}
				identifier = fmt.Sprintf("user:%d", userID)
				function = extractFunctionFromRequest(r)
				if function == "" {
					function = "default"
				}
			}

			// Check rate limit
			result, err := rl.CheckRateLimit(r.Context(), identifier, function, isPublic)
			if err != nil {
				// Log error but don't block request
				next.ServeHTTP(w, r)
				return
			}

			// Set rate limit headers
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.getLimit(function, isPublic)))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(result.ResetTime.Unix(), 10))

			if !result.Allowed {
				w.Header().Set("Retry-After", strconv.Itoa(result.RetryAfter))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error": "Rate limit exceeded", "retry_after": ` + strconv.Itoa(result.RetryAfter) + `}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (rl *RateLimiter) getLimit(function string, isPublic bool) int {
	if isPublic {
		if config, exists := rl.config["public:"+function]; exists {
			return config.RequestsPerMinute
		}
		return rl.config["public:default"].RequestsPerMinute
	} else {
		if strings.Contains(function, "chat") || strings.Contains(function, "Chat") {
			return rl.config["private:chat"].RequestsPerMinute
		} else if strings.Contains(function, "backtest") || strings.Contains(function, "Backtest") {
			return rl.config["private:backtest"].RequestsPerMinute
		} else if function == "upload" {
			return rl.config["private:upload"].RequestsPerMinute
		}
		return rl.config["private:default"].RequestsPerMinute
	}
}

func getClientIP(r *http.Request) string {
	// Check for X-Forwarded-For header (from load balancers/proxies)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check for X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}

func extractFunctionFromRequest(r *http.Request) string {
	// Parse JSON body to extract function name
	if r.Body == nil {
		return ""
	}

	// Read body into a buffer so we can read it again
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return ""
	}

	// Restore the body for the next handler
	r.Body = io.NopCloser(strings.NewReader(string(body)))

	// Parse JSON to extract function name
	var req struct {
		Function string `json:"func"`
	}

	if err := json.Unmarshal(body, &req); err != nil {
		return ""
	}

	return req.Function
}

func getUserIDFromRequest(r *http.Request) int {
	// Extract user ID from JWT token in Authorization header
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		return -1
	}

	// Use the existing validateToken function to get user ID
	userID, err := validateToken(tokenString)
	if err != nil {
		return -1
	}

	return userID
}
