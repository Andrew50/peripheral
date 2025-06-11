package server

import (
	"backend/internal/data"
	"backend/internal/services/socket"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"time"

	"github.com/gorilla/websocket"

	"backend/internal/app/account"
	"backend/internal/app/agent"
	"backend/internal/app/alerts"
	"backend/internal/app/chart"
	"backend/internal/app/filings"
	"backend/internal/app/helpers"
	"backend/internal/app/screensaver"
	"backend/internal/app/settings"
	"backend/internal/app/strategy"
	"backend/internal/app/watchlist"
	"context"
)

var publicFunc = map[string]func(*data.Conn, json.RawMessage) (interface{}, error){
	"signup":         Signup,
	"login":          Login,
	"googleLogin":    GoogleLogin,
	"googleCallback": GoogleCallback,
}

// Wrapper functions to adapt existing functions to the old signature for HTTP handlers
func wrapContextFunc(fn func(context.Context, *data.Conn, int, json.RawMessage) (interface{}, error)) func(*data.Conn, int, json.RawMessage) (interface{}, error) {
	return func(conn *data.Conn, userID int, args json.RawMessage) (interface{}, error) {
		// Create a background context for non-cancellable functions
		ctx := context.Background()
		return fn(ctx, conn, userID, args)
	}
}

// Private functions for /private endpoint that use the old signature
var privateFunc = map[string]func(*data.Conn, int, json.RawMessage) (interface{}, error){

	// --- chat / conversation --------------------------------------------------
	//"getSimilarInstances": helpers.GetSimilarInstances,
	"getInstancesByTickers":            screensaver.GetInstancesByTickers,
	"getCurrentSecurityID":             helpers.GetCurrentSecurityID,
	"getSecurityIDFromTickerTimestamp": helpers.GetSecurityIDFromTickerTimestamp,
	"getSecuritiesFromTicker":          helpers.GetSecuritiesFromTicker,
	"getCurrentTicker":                 helpers.GetCurrentTicker,
	"getTickerMenuDetails":             helpers.GetTickerMenuDetails,
	"getIcons":                         helpers.GetIcons,
	"getPrevClose":                     helpers.GetPrevClose,
	"getExchanges":                     helpers.GetExchanges,
	"getSecurityClassifications":       helpers.GetSecurityClassifications,

	"getLatestEdgarFilings": filings.GetLatestEdgarFilings,
	"getStockEdgarFilings":  filings.GetStockEdgarFilings,
	"getEarningsText":       filings.GetEarningsText,
	"getFilingText":         filings.GetFilingText,
	"getChartData":          chart.GetChartData,
	/*"getStudies":        chart.GetStudies,
	"newStudy":          chart.NewStudy,
	"saveStudy":         chart.SaveStudy,
	"deleteStudy":       chart.DeleteStudy,
	"getStudyEntry":     chart.GetStudyEntry,
	"completeStudy":     chart.CompleteStudy,
	"setStudyStrategy":  chart.SetStudyStrategy,*/
	"getChartEvents":       chart.GetChartEvents,
	"setHorizontalLine":    chart.SetHorizontalLine,
	"getHorizontalLines":   chart.GetHorizontalLines,
	"deleteHorizontalLine": chart.DeleteHorizontalLine,
	"updateHorizontalLine": chart.UpdateHorizontalLine,

	// --- screensavers ---------------------------------------------------------
	"getScreensavers": screensaver.GetScreensavers,

	// --- watchlists -----------------------------------------------------------
	"getWatchlists":       watchlist.GetWatchlists,
	"deleteWatchlist":     watchlist.DeleteWatchlist,
	"newWatchlist":        watchlist.NewWatchlist,
	"getWatchlistItems":   watchlist.GetWatchlistItems,
	"deleteWatchlistItem": watchlist.DeleteWatchlistItem,
	"newWatchlistItem":    watchlist.NewWatchlistItem,

	// --- user settings / profile ---------------------------------------------
	"getSettings":          settings.GetSettings,
	"setSettings":          settings.SetSettings,
	"updateProfilePicture": settings.UpdateProfilePicture,

	// --- alerts ---------------------------------------------------------------
	"getAlerts":    alerts.GetAlerts,
	"getAlertLogs": alerts.GetAlertLogs,
	"newAlert":     alerts.NewAlert,
	"deleteAlert":  alerts.DeleteAlert,

	// --- trades / statistics --------------------------------------------------
	"grab_user_trades":       account.GrabUserTrades,
	"get_trade_statistics":   account.GetTradeStatistics,
	"get_ticker_performance": account.GetTickerPerformance,
	"delete_all_user_trades": account.DeleteAllUserTrades,
	"handle_trade_upload":    account.HandleTradeUpload,
	"get_daily_trade_stats":  account.GetDailyTradeStats,

	// --- strategy / back-testing ---------------------------------------------
	"run_backtest":                   wrapContextFunc(strategy.RunBacktest),
	"getStrategies":                  strategy.GetStrategies,
	"newStrategy":                    strategy.NewStrategy,
	"setStrategy":                    strategy.SetStrategy,
	"deleteStrategy":                 strategy.DeleteStrategy,
	"getStrategyFromNaturalLanguage": strategy.CreateStrategyFromNaturalLanguage,
	"getStrategySpec":                strategy.GetStrategySpec,

	// --- misc / auth helpers --------------------------------------------------
	"verifyAuth": func(*data.Conn, int, json.RawMessage) (interface{}, error) {
		// TODO: replace with real auth logic
		return nil, nil
	},
	"getUserConversation":        agent.GetUserConversation,
	"getSuggestedQueries":        agent.GetSuggestedQueries,
	"getInitialQuerySuggestions": agent.GetInitialQuerySuggestions,
	"getQuery":                   wrapContextFunc(agent.GetChatRequest),

	// Multiple conversations management
	"getUserConversations": agent.GetUserConversations,
	"switchConversation":   agent.SwitchConversation,
	"deleteConversation":   agent.DeleteConversation,
	"cancelPendingMessage": agent.CancelPendingMessage,
	"editMessage":          agent.EditMessage,
	"getWhyMoving":         agent.GetWhyMoving,
}

// Private functions that support context cancellation
var privateFuncWithContext = map[string]func(context.Context, *data.Conn, int, json.RawMessage) (interface{}, error){
	"getQuery": agent.GetChatRequest,
}

// Request represents a structure for handling Request data.
type Request struct {
	Function  string          `json:"func"`
	Arguments json.RawMessage `json:"args"`
}

func addCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
}

func handleError(w http.ResponseWriter, err error, context string) bool {
	if err != nil {
		logMessage := fmt.Sprintf("%s: %v", context, err)
		if context == "auth" {
			http.Error(w, logMessage, http.StatusUnauthorized)
		} else {
			http.Error(w, logMessage, http.StatusBadRequest)
		}
		return true
	}
	return false
}

func publicHandler(conn *data.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addCORSHeaders(w)
		if r.Method == "OPTIONS" {
			return
		}
		////fmt.Println("debug: got public request")
		// Validate content type to prevent content-type sniffing attacks
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			http.Error(w, "Unsupported Media Type", http.StatusUnsupportedMediaType)
			return
		}

		// Set security headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")

		// Read the request body with size limit to prevent DoS attacks
		bodySize := r.ContentLength
		if bodySize > 1024*1024 { // 1MB limit
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}

		var req Request
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields() // Prevent JSON pollution attacks
		err := decoder.Decode(&req)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid request format: %v", err), http.StatusBadRequest)
			return
		}

		// Validate the function name
		if _, exists := publicFunc[req.Function]; !exists {
			http.Error(w, "Unknown function", http.StatusBadRequest)
			return
		}

		// Execute the requested function with sanitized input
		result, err := publicFunc[req.Function](conn, req.Arguments)
		if err != nil {
			// Log the detailed error on the server
			//log.Printf("public_handler error: %s - %v", req.Function, err)
			// Send the specific error message back to the client
			// Use StatusBadRequest for general input/logic errors from Login/Signup
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		encoder := json.NewEncoder(w)
		encoder.SetEscapeHTML(true) // Escape HTML in JSON responses
		if err := encoder.Encode(result); err != nil {
			http.Error(w, "Error encoding response", http.StatusInternalServerError)
		}
	}
}

func privateUploadHandler(conn *data.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addCORSHeaders(w)
		// Handle CORS preflight request
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		// Original check was here, moved after OPTIONS check.
		if r.Method != "POST" {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		tokenString := r.Header.Get("Authorization")
		userID, err := validateToken(tokenString)
		if handleError(w, err, "auth") {
			return
		}

		if err := r.ParseMultipartForm(32 << 20); err != nil {
			handleError(w, err, "parsing multipart form")
			return
		}

		// Get function name
		funcName := r.FormValue("func")
		if funcName == "" {
			handleError(w, fmt.Errorf("missing function name"), "function name")
			return
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			handleError(w, err, "file")
			return
		}
		defer file.Close()

		fileContent, err := io.ReadAll(file)
		if err != nil {
			handleError(w, err, "reading file")
			return
		}
		encodedContent := base64.StdEncoding.EncodeToString(fileContent)

		// Parse additional arguments
		var additionalArgs map[string]interface{}
		if argsStr := r.FormValue("args"); argsStr != "" {
			if err := json.Unmarshal([]byte(argsStr), &additionalArgs); err != nil {
				handleError(w, err, "parsing additional arguments")
				return
			}
		}

		// Handle trade upload directly in Go instead of queueing it
		if funcName == "handle_trade_upload" {
			// Create args directly for HandleTradeUpload
			argsBytes, err := json.Marshal(map[string]interface{}{
				"file_content": encodedContent,
				"extra":        additionalArgs,
			})
			if err != nil {
				handleError(w, err, "marshaling arguments")
				return
			}

			// Call the Go implementation directly
			result, err := account.HandleTradeUpload(conn, userID, argsBytes)
			if handleError(w, err, "processing trade upload") {
				return
			}

			// Return the result directly
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(result); err != nil {
				handleError(w, err, "encoding response")
				return
			}
			return
		}
	}
}

func privateHandler(conn *data.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addCORSHeaders(w)
		if r.Method == "OPTIONS" {
			return
		}

		tokenString := r.Header.Get("Authorization")
		userID, err := validateToken(tokenString)
		if handleError(w, err, "auth") {
			return
		}

		// Validate content type to prevent content-type sniffing attacks
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			http.Error(w, "Unsupported Media Type", http.StatusUnsupportedMediaType)
			return
		}

		// Set security headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")

		// Read the request body with size limit to prevent DoS attacks
		bodySize := r.ContentLength
		if bodySize > 1024*1024 { // 1MB limit
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}

		var req Request
		if handleError(w, json.NewDecoder(r.Body).Decode(&req), "decoding request") {
			return
		}

		// Sanitize the JSON input to prevent injection attacks
		sanitizedArgs, err := sanitizeJSON(req.Arguments)
		if err != nil {
			handleError(w, err, "sanitizing input")
			return
		}
		req.Arguments = sanitizedArgs

		// Validate the function name
		if _, exists := privateFunc[req.Function]; !exists && privateFuncWithContext[req.Function] == nil {
			http.Error(w, "Unknown function", http.StatusBadRequest)
			return
		}

		// Execute the requested function with sanitized input and request context
		var result interface{}

		// Try context-aware function first
		if contextFunc, exists := privateFuncWithContext[req.Function]; exists {
			result, err = contextFunc(r.Context(), conn, userID, req.Arguments)

			// Handle context cancellation gracefully
			if err != nil && r.Context().Err() == context.Canceled {
				// Return a structured cancellation response instead of an error
				cancelResponse := map[string]interface{}{
					"type":    "cancelled",
					"message": "Request was cancelled by user",
				}
				w.Header().Set("Content-Type", "application/json")
				encoder := json.NewEncoder(w)
				encoder.SetEscapeHTML(true)
				if err := encoder.Encode(cancelResponse); err != nil {
					http.Error(w, fmt.Sprintf("Error encoding cancellation response: %v", err), http.StatusInternalServerError)
				}
				return
			}
		} else if regularFunc, exists := privateFunc[req.Function]; exists {
			// Fallback to regular function for functions not yet updated
			result, err = regularFunc(conn, userID, req.Arguments)
		} else {
			http.Error(w, "Unknown function", http.StatusBadRequest)
			return
		}

		if handleError(w, err, fmt.Sprintf("private_handler: %s", req.Function)) {
			return
		}

		w.Header().Set("Content-Type", "application/json")
		encoder := json.NewEncoder(w)
		encoder.SetEscapeHTML(true) // Escape HTML in JSON responses
		if err := encoder.Encode(result); err != nil {
			http.Error(w, fmt.Sprintf("Error encoding response: %v", err), http.StatusInternalServerError)
		}
	}
}

// Helper function to sanitize JSON input and prevent injection attacks
func sanitizeJSON(input json.RawMessage) (json.RawMessage, error) {
	// Basic validation to ensure JSON is well-formed
	var jsonObj interface{}
	err := json.Unmarshal(input, &jsonObj)
	if err != nil {
		return nil, fmt.Errorf("invalid JSON format: %v", err)
	}

	// Apply recursive sanitization on the object
	sanitized, err := sanitizeValue(jsonObj)
	if err != nil {
		return nil, err
	}

	// Convert back to JSON
	result, err := json.Marshal(sanitized)
	if err != nil {
		return nil, fmt.Errorf("error marshaling sanitized JSON: %v", err)
	}

	return result, nil
}

// Recursively sanitize values in JSON objects
func sanitizeValue(val interface{}) (interface{}, error) {
	switch v := val.(type) {
	case string:
		// Prevent common injection patterns in strings
		if containsInjectionPattern(v) {
			return nil, fmt.Errorf("potentially malicious input detected")
		}
		return v, nil
	case map[string]interface{}:
		sanitizedMap := make(map[string]interface{})
		for k, subVal := range v {
			// Prevent injection in keys
			if containsInjectionPattern(k) {
				return nil, fmt.Errorf("potentially malicious key detected")
			}
			sanitizedSubVal, err := sanitizeValue(subVal)
			if err != nil {
				return nil, err
			}
			sanitizedMap[k] = sanitizedSubVal
		}
		return sanitizedMap, nil
	case []interface{}:
		sanitizedArr := make([]interface{}, len(v))
		for i, subVal := range v {
			sanitizedSubVal, err := sanitizeValue(subVal)
			if err != nil {
				return nil, err
			}
			sanitizedArr[i] = sanitizedSubVal
		}
		return sanitizedArr, nil
	default:
		// Numbers, booleans, and null values are safe
		return v, nil
	}
}

// Check for common injection patterns
func containsInjectionPattern(s string) bool {
	// Check for SQL injection patterns
	sqlPatterns := []string{
		"'--",
		"OR 1=1",
		"UNION SELECT",
		"DROP TABLE",
		"INSERT INTO",
		"DELETE FROM",
		"UPDATE.*SET",
	}

	// Check for XSS patterns
	xssPatterns := []string{
		"<script>",
		"javascript:",
		"onerror=",
		"onload=",
		"eval\\(",
	}

	patterns := append(sqlPatterns, xssPatterns...)

	for _, pattern := range patterns {
		matched, _ := regexp.MatchString("(?i)"+pattern, s)
		if matched {
			return true
		}
	}

	return false
}

// WSHandler performs operations related to WSHandler functionality.
func WSHandler(conn *data.Conn) http.HandlerFunc {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(_ *http.Request) bool {
			return true // Allow all origins
		},
	}

	return func(w http.ResponseWriter, r *http.Request) {
		addCORSHeaders(w)

		// Extract the token from the query parameters
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "Token is required", http.StatusBadRequest)
			return
		}

		// Validate the token and extract the user ID
		userID, err := validateToken(token)
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Upgrade the connection to a WebSocket
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			//		////fmt.Println("Failed to upgrade to WebSocket:", err)
			return
		}

		// Call the slimmed-down version of WsHandler in socket.go
		socket.HandleWebSocket(conn, ws, userID)
	}
}

func HealthCheck() http.HandlerFunc {
	type status struct {
		OK bool `json:"ok"`
	}

	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// If you need DB ping logic, insert it here and flip OK accordingly.
		_ = json.NewEncoder(w).Encode(status{OK: true})
	}
}

// StartServer performs operations related to StartServer functionality.
func StartServer(conn *data.Conn) {
	http.HandleFunc("/public", publicHandler(conn))
	http.HandleFunc("/private", privateHandler(conn))
	http.HandleFunc("/ws", WSHandler(conn))
	http.HandleFunc("/upload", privateUploadHandler(conn))
	http.HandleFunc("/healthz", HealthCheck())

	server := &http.Server{
		Addr:    ":5058",
		Handler: http.DefaultServeMux, // Use DefaultServeMux since HandleFunc registers globally
		// Good practice to set timeouts to prevent resource exhaustion.
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 240 * time.Second,
		IdleTimeout:  240 * time.Second,
	}

	log.Println("debug: Server running on port 5058")
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
