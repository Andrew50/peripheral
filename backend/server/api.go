package server

import (
	"backend/jobs"
	"backend/tasks"

	"backend/socket"
	"backend/utils"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var publicFunc = map[string]func(*utils.Conn, json.RawMessage) (interface{}, error){
	"signup":         Signup,
	"login":          Login,
	"googleLogin":    GoogleLogin,
	"googleCallback": GoogleCallback,
}

var privateFunc = map[string]func(*utils.Conn, int, json.RawMessage) (interface{}, error){
	"verifyAuth": verifyAuth,
	//securities
	"getSimilarInstances":     tasks.GetSimilarInstances,
	"getSecuritiesFromTicker": tasks.GetSecuritiesFromTicker,
	"getCurrentTicker":        tasks.GetCurrentTicker,
	//"getTickerDetails":        tasks.GetTickerDetails,
	"getTickerMenuDetails": tasks.GetTickerMenuDetails,

	//chart
	"getChartData": tasks.GetChartData,
	//study
	"getStudies": tasks.GetStudies,

	"newStudy":      tasks.NewStudy,
	"saveStudy":     tasks.SaveStudy,
	"deleteStudy":   tasks.DeleteStudy,
	"getStudyEntry": tasks.GetStudyEntry,
	"completeStudy": tasks.CompleteStudy,
	"setStudySetup": tasks.SetStudySetup,
	//journal
	"getJournals":     tasks.GetJournals,
	"saveJournal":     tasks.SaveJournal,
	"deleteJournal":   tasks.DeleteJournal,
	"getJournalEntry": tasks.GetJournalEntry,
	"completeJournal": tasks.CompleteJournal,
	//screensaver
	"getScreensavers": tasks.GetScreensavers,
	//watchlist
	"getWatchlists":       tasks.GetWatchlists,
	"deleteWatchlist":     tasks.DeleteWatchlist,
	"newWatchlist":        tasks.NewWatchlist,
	"getWatchlistItems":   tasks.GetWatchlistItems,
	"deleteWatchlistItem": tasks.DeleteWatchlistItem,
	"newWatchlistItem":    tasks.NewWatchlistItem,
	//singles
	"getPrevClose": tasks.GetPrevClose,
	//"getMarketCap": tasks.GetMarketCap,
	//settings
	"getSettings": tasks.GetSettings,
	"setSettings": tasks.SetSettings,
	//exchanges
	"getExchanges": tasks.GetExchanges,
	//setups
	"getSetups":   tasks.GetSetups,
	"newSetup":    tasks.NewSetup,
	"setSetup":    tasks.SetSetup,
	"deleteSetup": tasks.DeleteSetup,
	//algos
	//"getAlgos": tasks.GetAlgos,
	//samples
	"labelTrainingQueueInstance": tasks.LabelTrainingQueueInstance,
	"getTrainingQueue":           tasks.GetTrainingQueue,
	"setSample":                  tasks.SetSample,
	//telegram
	//	"sendMessage": telegram.SendMessage,
	//alerts
	"getAlerts":    tasks.GetAlerts,
	"getAlertLogs": tasks.GetAlertLogs,
	"newAlert":     tasks.NewAlert,
	"deleteAlert":  tasks.DeleteAlert,
	//"setAlert":tasks.SetAlert,

	// deprecated
	// "getTradeData":            tasks.GetTradeData,
	//
	//	"getLastTrade":            tasks.GetLastTrade,
	//
	// "getQuoteData":            tasks.GetQuoteData,
	// "getSecurityDateBounds":   tasks.GetSecurityDateBounds,
	"setHorizontalLine":    tasks.SetHorizontalLine,
	"getHorizontalLines":   tasks.GetHorizontalLines,
	"deleteHorizontalLine": tasks.DeleteHorizontalLine,
	//"updateHorizontalLine": tasks.UpdateHorizontalLine,
	//active

	"getActive": tasks.GetActive,
	//sector, industry
	"getSecurityClassifications": tasks.GetSecurityClassifications,
}

func verifyAuth(_ *utils.Conn, _ int, _ json.RawMessage) (interface{}, error) { return nil, nil }

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

func public_handler(conn *utils.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addCORSHeaders(w)
		if r.Method == "OPTIONS" {
			return
		}
		//fmt.Println("debug: got public request")
		var req Request
		err := json.NewDecoder(r.Body).Decode(&req)
		if handleError(w, err, "decoding request") {
			return
		}
		fmt.Println(req.Function)
		if function, ok := publicFunc[req.Function]; ok {
			result, err := function(conn, req.Arguments)
			if handleError(w, err, req.Function) {
				return
			}
			err = json.NewEncoder(w).Encode(result)
			if handleError(w, err, "encoding response") {
				return
			}
			return
		} else {
			http.Error(w, fmt.Sprintf("invalid function: %s", req.Function), http.StatusBadRequest)
			fmt.Printf("invalid function: %s", req.Function)
			return
		}
	}
}

func private_upload_handler(conn *utils.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addCORSHeaders(w)
		if r.Method != "POST" {
			return
		}
		token_string := r.Header.Get("Authorization")
		userId, err := validate_token(token_string)
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

		// Include userId in the arguments
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

func private_handler(conn *utils.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addCORSHeaders(w)
		if r.Method != "POST" {
			return
		}
		//fmt.Println("debug: got private request")
		token_string := r.Header.Get("Authorization")
		user_id, err := validate_token(token_string)
		if handleError(w, err, "auth") {
			return
		}
		var req Request
		if handleError(w, json.NewDecoder(r.Body).Decode(&req), "decoding request") {
			return
		}
		//fmt.Printf("debug: %s\n", req.Function)

		if function, ok := privateFunc[req.Function]; ok {
			result, err := function(conn, user_id, req.Arguments)
			if handleError(w, err, req.Function) {
				return
			}
			err = json.NewEncoder(w).Encode(result)
			if handleError(w, err, "encoding response") {
				return
			}
		} else {
			http.Error(w, fmt.Sprintf("invalid function: %s", req.Function), http.StatusBadRequest)
			fmt.Printf("invalid function: %s", req.Function)
			return
		}
	}
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
		userId, err := validate_token(token_string)
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
		err = json.NewEncoder(w).Encode(response)
		if handleError(w, err, "190v0id") {
			return
		}
	}
}

type PollRequest struct {
	TaskId string `json:"taskId"`
}

func pollHandler(conn *utils.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addCORSHeaders(w)
		if r.Method != "POST" {
			return
		}
		token_string := r.Header.Get("Authorization")
		_, err := validate_token(token_string)
		if handleError(w, err, "auth") {
			return
		}
		var req PollRequest
		if handleError(w, json.NewDecoder(r.Body).Decode(&req), "1m99c") {
			return
		}
		result, err := utils.Poll(conn, req.TaskId)
		if handleError(w, err, fmt.Sprintf("executing function %s", req.TaskId)) {
			return
		}
		err = json.NewEncoder(w).Encode(result)
		if handleError(w, err, "19inv0id") {
			return
		}
	}
}

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
		userID, err := validate_token(token)
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

func StartServer() {
	conn, cleanup := utils.InitConn(true)
	defer cleanup()
	stopScheduler := jobs.StartScheduler(conn)
	defer close(stopScheduler)
	http.HandleFunc("/public", public_handler(conn))
	http.HandleFunc("/private", private_handler(conn))
	http.HandleFunc("/queue", queueHandler(conn))
	http.HandleFunc("/poll", pollHandler(conn))
	http.HandleFunc("/ws", WSHandler(conn))
	http.HandleFunc("/private-upload", private_upload_handler(conn))
	fmt.Println("debug: Server running on port 5057 ----------------------------------------------------------")
	if err := http.ListenAndServe(":5057", nil); err != nil {
		log.Fatal(err)
	}
}
