package query


import (
	"encoding/json"
	"fmt"
	"backend/utils"
	
)
type Query struct {
	Query string `json:"query"`
}




func GetQuery(conn *utils.Conn, userID int, args json.RawMessage) (interface{}, error) {
	var query Query
	if err := json.Unmarshal(args, &query); err != nil {
		return nil, fmt.Errorf("error parsing request: %w", err)
	}

	llmResponse, err := GetLLMParsedQuery(conn, query)
	if err != nil {
		return nil, fmt.Errorf("error getting llm response: %w", err)
	}

	return llmResponse, nil
}

func GetLLMParsedQuery(conn *utils.Conn, query Query) (string, error) {
	llmResponse, err := getGeminiResponse(conn, query.Query)
	if err != nil {
		return "", fmt.Errorf("error getting gemini response: %w", err)
	}

	return llmResponse, nil
}