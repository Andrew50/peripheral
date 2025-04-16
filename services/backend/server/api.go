package server

import (
	"backend/jobs"
	"backend/socket"
	"backend/tools"
	"backend/utils"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"regexp"

	"github.com/gorilla/websocket"
)

var publicFunc = map[string]func(*utils.Conn, json.RawMessage) (interface{}, error){
	"signup":         Signup,
	"login":          Login,
	"googleLogin":    GoogleLogin,
	"googleCallback": GoogleCallback,
	"guestLogin":     GuestLogin,
}

// Define privateFunc as an alias to Tools
var privateFunc = tools.GetTools(true)

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
		fmt.Println(logMessage)
		if context == "auth" {
			http.Error(w, logMessage, http.StatusUnauthorized)
		} else {
			http.Error(w, logMessage, http.StatusBadRequest)
		}
		return true
	}
	return false
}

func publicHandler(conn *utils.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addCORSHeaders(w)
		if r.Method == "OPTIONS" {
			return
		}
		fmt.Println("debug: got public request")
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
			// Limit error information in public endpoints
			log.Printf("public_handler error: %s - %v", req.Function, err)
			http.Error(w, "Request processing failed", http.StatusBadRequest)
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

func privateUploadHandler(conn *utils.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addCORSHeaders(w)
		if r.Method != "POST" {
			return
		}
		token_string := r.Header.Get("Authorization")
		userId, err := validateToken(token_string)
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
			result, err := tools.HandleTradeUpload(conn, userId, argsBytes)
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

		// For other functions, use the queue as before
		args := map[string]interface{}{
			"file_content":    encodedContent,
			"additional_args": additionalArgs,
			"user_id":         userId,
		}

		taskId, err := utils.Queue(conn, funcName, args)
		if handleError(w, err, "queuing task") {
			return
		}

		response := map[string]string{
			"taskId": taskId,
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			handleError(w, err, "encoding response")
			return
		}
	}
}

func privateHandler(conn *utils.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addCORSHeaders(w)
		if r.Method == "OPTIONS" {
			return
		}

		token_string := r.Header.Get("Authorization")
		_, err := validateToken(token_string)
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
		if _, exists := privateFunc[req.Function]; !exists {
			http.Error(w, "Unknown function", http.StatusBadRequest)
			return
		}

		// Get user ID from token
		userId, err := validateToken(token_string)
		if handleError(w, err, "private_handler: validateToken") {
			return
		}

		// Execute the requested function with sanitized input
		result, err := privateFunc[req.Function].Function(conn, userId, req.Arguments)
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

type QueueRequest struct {
	Function  string      `json:"func"`
	Arguments interface{} `json:"args"`
}

func queueHandler(conn *utils.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addCORSHeaders(w)
		if r.Method != "POST" {
			return
		}
		fmt.Println("debug: got queue request")
		token_string := r.Header.Get("Authorization")
		userId, err := validateToken(token_string)
		if handleError(w, err, "auth") {
			return
		}
		var req Request
		if handleError(w, json.NewDecoder(r.Body).Decode(&req), "decoding request") {
			return
		}

		// Create a map for the combined arguments
		var args map[string]interface{}

		// If req.Arguments is not empty, unmarshal it into the args map
		if len(req.Arguments) > 0 {
			if err := json.Unmarshal(req.Arguments, &args); err != nil {
				handleError(w, err, "parsing arguments")
				return
			}
		} else {
			// Initialize empty map if no arguments were provided
			args = make(map[string]interface{})
		}

		// Add userId to the arguments
		args["user_id"] = userId

		taskId, err := utils.Queue(conn, req.Function, args)
		if handleError(w, err, "queue") {
			return
		}
		response := map[string]string{
			"taskId": taskId,
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			handleError(w, err, "190v0id")
			return
		}
	}
}

// PollRequest represents a structure for handling PollRequest data.
type PollRequest struct {
	TaskID string `json:"taskId"`
}

func pollHandler(conn *utils.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addCORSHeaders(w)
		if r.Method != "POST" {
			return
		}
		token_string := r.Header.Get("Authorization")
		_, err := validateToken(token_string)
		if handleError(w, err, "auth") {
			return
		}
		var req PollRequest
		if handleError(w, json.NewDecoder(r.Body).Decode(&req), "1m99c") {
			return
		}

		// Get the task and print its logs
		task, err := utils.GetTask(conn, req.TaskID)
		if err != nil {
			handleError(w, err, fmt.Sprintf("getting task %s", req.TaskID))
			return
		}

		// Print logs to server console
		if len(task.Logs) > 0 {
			fmt.Printf("Task %s (%s) logs:\n", req.TaskID, task.Function)
			for _, logEntry := range task.Logs {
				fmt.Printf("[%s] %s: %s\n",
					logEntry.Timestamp.Format("2006-01-02 15:04:05"),
					logEntry.Level,
					logEntry.Message)
			}
		}

		// Serialize task to JSON
		result, err := json.Marshal(task)
		if err != nil {
			handleError(w, err, fmt.Sprintf("serializing task %s", req.TaskID))
			return
		}

		if err := json.NewEncoder(w).Encode(json.RawMessage(result)); err != nil {
			handleError(w, err, "19inv0id")
			return
		}
	}
}

// WSHandler performs operations related to WSHandler functionality.
func WSHandler(conn *utils.Conn) http.HandlerFunc {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
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
			fmt.Println("Failed to upgrade to WebSocket:", err)
			return
		}

		// Call the slimmed-down version of WsHandler in socket.go
		socket.HandleWebSocket(conn, ws, userID)
	}
}

// Health check endpoint handler
func healthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Create a response object
		response := map[string]string{
			"status":  "healthy",
			"service": "backend",
		}

		// Set content type header
		w.Header().Set("Content-Type", "application/json")

		// Write the response
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Error encoding health response: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	}
}

// StartServer performs operations related to StartServer functionality.
func StartServer() {
	conn, cleanup := utils.InitConn(true)
	defer cleanup()
	stopScheduler := jobs.StartScheduler(conn)
	defer close(stopScheduler)
	http.HandleFunc("/public", publicHandler(conn))
	http.HandleFunc("/private", privateHandler(conn))
	http.HandleFunc("/queue", queueHandler(conn))
	http.HandleFunc("/poll", pollHandler(conn))
	http.HandleFunc("/ws", WSHandler(conn))
	http.HandleFunc("/private-upload", privateUploadHandler(conn))
	http.HandleFunc("/health", healthHandler())
	//http.HandleFunc("/backend/health", healthHandler())
	fmt.Println("debug: Server running on port 5058 ----------------------------------------------------------")
	if err := http.ListenAndServe(":5058", nil); err != nil {
		log.Fatal(err)
	}
}
