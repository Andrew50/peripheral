package server

import (
	"backend/jobs"
	"backend/tasks"
	"backend/utils"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

var publicFunc = map[string]func(*utils.Conn, json.RawMessage) (interface{}, error){
	"signup": Signup,
	"login":  Login,
}

var privateFunc = map[string]func(*utils.Conn, int, json.RawMessage) (interface{}, error){
	"verifyAuth":              verifyAuth,
	"getSimilarInstances":     tasks.GetSimilarInstances,
	"getSecuritiesFromTicker": tasks.GetSecuritiesFromTicker,
	"getChartData":            tasks.GetChartData,
	"getSecurityDateBounds":   tasks.GetSecurityDateBounds,
	"getStudies":              tasks.GetStudies,
	"newStudy":                tasks.NewStudy,
	"saveStudy":               tasks.SaveStudy,
	"deleteStudy":             tasks.DeleteStudy,
	"getStudyEntry":           tasks.GetStudyEntry,
	"completeStudy":           tasks.CompleteStudy,
	"getSetups":               tasks.GetSetups,
	"getTradeData":            tasks.GetTradeData,
	"getJournals":             tasks.GetJournals,
	"saveJournal":             tasks.SaveJournal,
	"deleteJournal":           tasks.DeleteJournal,
	"getJournalEntry":         tasks.GetJournalEntry,
	"completeJournal":         tasks.CompleteJournal,
	"getScreensavers":         tasks.GetScreensavers,
	"getWatchlists":           tasks.GetWatchlists,
	"deleteWatchlist":         tasks.DeleteWatchlist,
	"newWatchlist":            tasks.NewWatchlist,
	"getWatchlistItems":       tasks.GetWatchlistItems,
	"deleteWatchlistItem":     tasks.DeleteWatchlistItem,
	"newWatchlistItem":        tasks.NewWatchlistItem,
	"getPrevClose":            tasks.GetPrevClose,
	"getLastTrade":            tasks.GetLastTrade,
	"getQuoteData":            tasks.GetQuoteData,
    "getSettings": tasks.GetSettings,
    "setSettings": tasks.SetSettings,
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
		http.Error(w, logMessage, http.StatusBadRequest)
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
		fmt.Println("got public request")
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

func private_handler(conn *utils.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addCORSHeaders(w)
		if r.Method != "POST" {
			return
		}
		fmt.Println("got private request")
		token_string := r.Header.Get("Authorization")
		user_id, err := validate_token(token_string)
		if handleError(w, err, "validating token") {
			return
		}
		var req Request
		if handleError(w, json.NewDecoder(r.Body).Decode(&req), "decoding request") {
			return
		}
		fmt.Println(req.Function)

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
		fmt.Println("got queue request")
		token_string := r.Header.Get("Authorization")
		_, err := validate_token(token_string)
		if handleError(w, err, "validating token") {
			return
		}
		var req Request
		if handleError(w, json.NewDecoder(r.Body).Decode(&req), "decoding request") {
			return
		}
		taskId, err := utils.Queue(conn, req.Function, req.Arguments)
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
		if handleError(w, err, "validating token") {
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

func StartServer() {
	conn, cleanup := utils.InitConn(true)
	defer cleanup()
	stopScheduler := jobs.StartScheduler(conn)
	defer close(stopScheduler)
	http.HandleFunc("/public", public_handler(conn))
	http.HandleFunc("/private", private_handler(conn))
	http.HandleFunc("/queue", queueHandler(conn))
	http.HandleFunc("/poll", pollHandler(conn))
	http.HandleFunc("/ws", utils.WsFrontendHandler(conn))

	fmt.Println("Server running on port 5057")
	if err := http.ListenAndServe(":5057", nil); err != nil {
		log.Fatal(err)
	}
}
