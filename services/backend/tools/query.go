package tools

// MRKL‑style agent with full Plan → Execute → Reflect loop **and** the
// compatibility features from the original `GetQuery` implementation:
//
//   •  Returns a `QueryResponse` struct so existing HTTP handlers remain intact.
//   •  Persists / restores conversation history in Redis through
//      `utils.Conn` (same helpers that existed before).
//   •  Re‑implements security‑ID injection so patterns like
//      `$$$AAPL-0$$$` become `$$$AAPL-12345-0$$$`.
//
// The code is written as a single Go file for clarity, but nothing prevents
// you from splitting it into smaller files.

import (
    "backend/utils"
    "runtime"
    "path/filepath"
    "os"
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "regexp"
    "sort"
    "strconv"
    "strings"
    "time"
    "bytes"

    "google.golang.org/genai"
)

// ─────────────────────────────────────────────────────────────────────────────
//  Shared models (identical to the original prototype where possible)
// ─────────────────────────────────────────────────────────────────────────────

type FunctionCall struct {
    Name string          `json:"name"`
    Args json.RawMessage `json:"args,omitempty"`
}

type ExecuteResult struct {
    FunctionName string      `json:"function_name"`
    Result       any `json:"result,omitempty"`
    Error        string      `json:"error,omitempty"`
    Args         any `json:"args,omitempty"`
}

type ContentChunk struct {
    Type    string      `json:"type"`
    Content any `json:"content"`
}

type ChatMessage struct {
    Role          string          `json:"role"` // "user", "assistant", "tool"
    ContentChunks []ContentChunk  `json:"content_chunks,omitempty"`
    Timestamp     time.Time       `json:"timestamp"`
    ExpiresAt     time.Time       `json:"expires_at"`
}

type ConversationData struct {
    Messages  []ChatMessage `json:"messages"`
    Timestamp time.Time     `json:"timestamp"`
}

// The response type expected by the existing REST layer.
type QueryResponse struct {
    Type          string            `json:"type"` // text | mixed_content | function_calls
    ContentChunks []ContentChunk    `json:"content_chunks,omitempty"`
    Text          string            `json:"text,omitempty"`
    Results       []ExecuteResult   `json:"results,omitempty"`
    History       *ConversationData `json:"history,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
//  Agent                                                                     
// ─────────────────────────────────────────────────────────────────────────────

type Agent struct {
    Conn          *utils.Conn
    UserID        int
    Ctx           context.Context
    History       *ConversationData
    RedisKey      string
    MaxRounds     int
    finalResponse *QueryResponse // built when loop finishes
}

func NewAgent(ctx context.Context, conn *utils.Conn, userID int) (*Agent, error) {
    key := fmt.Sprintf("user:%d:conversation", userID)
    hist, _ := getConversationFromCache(ctx, conn, userID, key) // ignore err → new conv
    if hist == nil {
        hist = &ConversationData{Timestamp: time.Now()}
    }
    return &Agent{
        Conn:      conn,
        UserID:    userID,
        Ctx:       ctx,
        History:   hist,
        RedisKey:  key,
        MaxRounds: 8,
    }, nil
}

// Public entry point, mirrors signature of the former GetQuery.
func GetQuery(conn *utils.Conn, userID int, args json.RawMessage) (any, error) {
    var q struct{ Query string `json:"query"` }
    if err := json.Unmarshal(args, &q); err != nil {
        return nil, err
    }
    if strings.TrimSpace(q.Query) == "" {
        return nil, errors.New("query cannot be empty")
    }

    ctx := context.Background()
    ag, err := NewAgent(ctx, conn, userID)
    if err != nil {
        return nil, err
    }

    ag.appendUser(q.Query)

    if err := ag.loop(); err != nil {
        return nil, err
    }

    // Persist conversation for future requests.
    _ = saveConversationToCache(ctx, conn, userID, ag.RedisKey, ag.History)

    return ag.finalResponse, nil
}

// ─────────────────────────────────────────────────────────────────────────────
//  The Plan → Execute → Reflect loop
// ─────────────────────────────────────────────────────────────────────────────

// ───────────────────────────────────────────
// 1️⃣  LOOP
// ───────────────────────────────────────────
func (a *Agent) loop() error {
	var allResults []ExecuteResult

	for round := 0; round < a.MaxRounds; round++ {
		// PLAN ― guide only, not stored in history
		guide, err := a.plan()
		if err != nil { return err }

		// EXECUTE ― Flash picks & calls tools
        results, draftText,err := a.executeStage(guide)
		if err != nil { return err }
		allResults = append(allResults, results...)

		// REFLECT ― decide if finished
        done, txt, err := a.reflect(results, draftText)
		if err != nil { return err }
		if done {
			a.finalResponse = &QueryResponse{
				Type:    "function_calls",
				Text:    txt,
				Results: allResults,
				History: a.History,
			}
			logConversationHistory(a.History)
			return nil
		}
	}

	// fallback
	a.finalResponse = &QueryResponse{
		Type:    "text",
		Text:    "Reached maximum reasoning rounds without completion.",
		Results: allResults,
		History: a.History,
	}
	logConversationHistory(a.History)
	return nil
}

// logConversationHistory logs the entire conversation history for debugging
func logConversationHistory(history *ConversationData) {
    fmt.Println("\n==== CONVERSATION HISTORY LOG ====")
    fmt.Printf("Total messages: %d\n", len(history.Messages))
    
    for i, msg := range history.Messages {
        fmt.Printf("\n--- Message %d ---\n", i+1)
        fmt.Printf("Role: %s\n", msg.Role)
        fmt.Printf("Timestamp: %s\n", msg.Timestamp.Format(time.RFC3339))
        
        if len(msg.ContentChunks) > 0 {
            fmt.Printf("Content Type: %s\n", msg.ContentChunks[0].Type)
            
            switch content := msg.ContentChunks[0].Content.(type) {
            case string:
                fmt.Printf("Content: %s\n", content)
            default:
                jsonContent, _ := json.MarshalIndent(content, "", "  ")
                fmt.Printf("Content (JSON):\n%s\n", string(jsonContent))
            }
        } else {
            fmt.Println("No content chunks")
        }
    }
    
    fmt.Println("\n==== END CONVERSATION HISTORY LOG ====")
}

// ─────────────────────────────────────────────────────────────────────────────
//  Stage helpers
// ─────────────────────────────────────────────────────────────────────────────

// buildToolPlaintext turns the current registry into a human‑readable
// definition block the model can see **inside** the conversation.
func buildToolPlaintext() string {
    tools := GetTools(false)

    // Keep alphabetical order so the prompt is stable.
    names := make([]string, 0, len(tools))
    for n := range tools { names = append(names, n) }
    sort.Strings(names)

    var b bytes.Buffer


    for _, n := range names {
        t := tools[n]
        decl := t.FunctionDeclaration
        b.WriteString("— ")
        b.WriteString(decl.Name)
        if decl.Description != "" {
            b.WriteString(": ")
            b.WriteString(decl.Description)
        }
        if decl.Parameters != nil {
            // Pretty‑print the JSON schema for arguments.
            argBytes, _ := json.MarshalIndent(decl.Parameters, "  ", "  ")
            b.WriteString("\n  args schema:\n  ")
            b.Write(argBytes)
        }
        b.WriteByte('\n')
    }
    return b.String()
}
func (a *Agent) plan() (string, error) {
	const model = "gemini-2.0-flash-thinking-exp-01-21"

	prompt := buildToolPlaintext() + "\n\n" +
		buildHistoryPrompt(a.History) + "\nUser: "

	resp, err := a.callGemini(model, mustSystemPrompt("plan"), prompt, false)
	if err != nil { return "", err }

	return strings.TrimSpace(resp.Text), nil
}
// ───────────────────────────────────────────
// 3️⃣  EXECUTE STAGE  (Gemini‑Flash 2‑0)
// ───────────────────────────────────────────
func (a *Agent) executeStage(guide string) ([]ExecuteResult, string, error) {
    const model = "gemini-2.0-flash-001"

    prompt := buildHistoryPrompt(a.History) +
        "\nAssistant GUIDE:\n" + guide +
        "\n\nAssistant: Decide which, if any, tools to call next. " +
        "Return function calls or answer directly."

    fmt.Println("execute stage hit")
    resp, err := a.callGemini(model, mustSystemPrompt("execute"), prompt, true)
    if err != nil {
        return nil, "", err
    }

    // Capture Gemini’s draft answer even if it also returned tool calls
    draftText := strings.TrimSpace(resp.Text)

    // If no tool calls, just return the text – reflector will decide
    fmt.Println(resp)
    if len(resp.FunctionCalls) == 0 {
        fmt.Println("no tool calls")
        if draftText != "" {
            a.appendAssistant(draftText)
        }
        return nil, draftText, nil
    }

    // … run the tools exactly as before …
    tools := GetTools(false)
    var results []ExecuteResult
    for _, fc := range resp.FunctionCalls {
        tool, ok := tools[fc.Name]
        if !ok {
            results = append(results, ExecuteResult{FunctionName: fc.Name, Error: "tool not found"})
            continue
        }
        res, err := tool.Function(a.Conn, a.UserID, fc.Args)
        if err != nil {
            results = append(results, ExecuteResult{FunctionName: fc.Name, Error: err.Error(), Args: fc.Args})
        } else {
            results = append(results, ExecuteResult{FunctionName: fc.Name, Result: res, Args: fc.Args})
        }
    }

    // Store tool output in history
    a.History.Messages = append(a.History.Messages, ChatMessage{
        Role:          "tool",
        ContentChunks: []ContentChunk{{Type: "json", Content: results}},
        Timestamp:     time.Now(),
        ExpiresAt:     time.Now().Add(24 * time.Hour),
    })

    // Also keep Gemini’s explanatory text (if any) so the reflector can check it
    if draftText != "" {
        a.History.Messages = append(a.History.Messages, ChatMessage{
            Role: "assistant",
            ContentChunks: []ContentChunk{{Type: "text", Content: draftText}},
            Timestamp: time.Now(),
            ExpiresAt: time.Now().Add(24 * time.Hour),
        })
    }

    return results, draftText, nil
}


func (a *Agent) reflect(results []ExecuteResult, draft string) (bool, string, error) {
    // Fetch the latest user utterance for the reflector prompt
    var userQuery string
    for i := len(a.History.Messages) - 1; i >= 0; i-- {
        if a.History.Messages[i].Role == "user" {
            userQuery = a.History.Messages[i].ContentChunks[0].Content.(string)
            break
        }
    }

    execTrace, _ := json.Marshal(results)
    prompt := fmt.Sprintf(
        "Original user query: %s\n"+
        "Plan + observations: %s\n"+
        "Draft answer: %s\n",
        userQuery, string(execTrace), draft)

    resp, err := a.callGemini(
        "gemini-2.0-flash-001",
        mustSystemPrompt("reflect"),
        prompt,
        /*withTools=*/false)        // reflector never calls tools
    if err != nil {
        return false, "", err
    }

    text := strings.TrimSpace(resp.Text)
    text = injectSecurityIDs(a.Conn, text)
    a.appendAssistant(text)

    done := strings.HasPrefix(text, "ANSWER:")
    return done, text, nil
}


// ─────────────────────────────────────────────────────────────────────────────
//  History helpers & utilities
// ─────────────────────────────────────────────────────────────────────────────

func (a *Agent) appendUser(q string) {
    a.History.Messages = append(a.History.Messages, ChatMessage{
        Role:          "user",
        ContentChunks: []ContentChunk{{Type: "text", Content: q}},
        Timestamp:     time.Now(),
        ExpiresAt:     time.Now().Add(24 * time.Hour),
    })
}

func (a *Agent) appendAssistant(t string) {
    a.History.Messages = append(a.History.Messages, ChatMessage{
        Role:          "assistant",
        ContentChunks: []ContentChunk{{Type: "text", Content: t}},
        Timestamp:     time.Now(),
        ExpiresAt:     time.Now().Add(24 * time.Hour),
    })
}

func buildHistoryPrompt(c *ConversationData) string {
    var sb strings.Builder
    for _, m := range c.Messages {
        if m.Role == "user" {
            sb.WriteString("User: ")
        } else if m.Role == "assistant" {
            sb.WriteString("Assistant: ")
        } else { // tool
            sb.WriteString("Tool: ")
        }
        if len(m.ContentChunks) > 0 {
            switch v := m.ContentChunks[0].Content.(type) {
            case string:
                sb.WriteString(v)
            default:
                b, _ := json.Marshal(v)
                sb.WriteString(string(b))
            }
        }
        sb.WriteString("\n")
    }
    return sb.String()
}

// ─────────────────────────────────────────────────────────────────────────────
//  Gemini wrapper
// ─────────────────────────────────────────────────────────────────────────────

type GeminiFunctionResponse struct {
    FunctionCalls []FunctionCall
    Text          string
}

func (a *Agent) callGemini(model, systemPrompt, prompt string, withTools bool) (*GeminiFunctionResponse, error) {
    apiKey, err := a.Conn.GetGeminiKey()
    if err != nil {
        return nil, err
    }
    client, err := genai.NewClient(a.Ctx, &genai.ClientConfig{APIKey: apiKey, Backend: genai.BackendGeminiAPI})
    if err != nil {
        return nil, err
    }

    cfg := &genai.GenerateContentConfig{SystemInstruction: &genai.Content{Parts: []*genai.Part{{Text: systemPrompt}}}}
    if withTools {
        cfg.Tools = convertTools()
        fmt.Println("tools")
        fmt.Println(cfg.Tools)
    }

    res, err := client.Models.GenerateContent(a.Ctx, model, genai.Text(prompt), cfg)
    if err != nil {
        return nil, err
    }

    out := &GeminiFunctionResponse{}
    if len(res.Candidates) > 0 && res.Candidates[0].Content != nil {
        for _, p := range res.Candidates[0].Content.Parts {
            if p.Text != "" {
                out.Text = p.Text
            }
            if fc := p.FunctionCall; fc != nil {
                args, _ := json.Marshal(fc.Args)
                out.FunctionCalls = append(out.FunctionCalls, FunctionCall{fc.Name, args})
            }
        }
    }
    return out, nil
}

func convertTools() []*genai.Tool {
    ts := GetTools(false) // query‑mode tools

    // Collect function declarations that are actually present.
    var fds []*genai.FunctionDeclaration
    for _, t := range ts {
        if t.FunctionDeclaration != nil {
            fds = append(fds, t.FunctionDeclaration)
        }
    }
    // Keep a stable order (helps with caching / tests).
    sort.Slice(fds, func(i, j int) bool { return fds[i].Name < fds[j].Name })

    if len(fds) == 0 {
        return nil // nothing to expose this turn
    }
    return []*genai.Tool{
        {FunctionDeclarations: fds}, // **one** Tool, many declarations
    }
}

// ─────────────────────────────────────────────────────────────────────────────
//  Security‑ID injection (restored)
// ─────────────────────────────────────────────────────────────────────────────

var tickerPlaceholder = regexp.MustCompile(`\$\$\$([A-Z]{1,5})-(\d+)\$\$\$`)

func injectSecurityIDs(conn *utils.Conn, text string) string {
    return tickerPlaceholder.ReplaceAllStringFunc(text, func(m string) string {
        sub := tickerPlaceholder.FindStringSubmatch(m)
        if len(sub) != 3 {
            return m
        }
        ticker, tsStr := sub[1], sub[2]
        ts, err := strconv.ParseInt(tsStr, 10, 64)
        if err != nil {
            return m
        }
        var t time.Time
        if ts == 0 {
            t = time.Now()
        } else {
            t = time.UnixMilli(ts)
        }
        id, err := utils.GetSecurityID(conn, ticker, t)
        if err != nil {
            return m
        }
        return fmt.Sprintf("$$$%s-%d-%s$$$", ticker, id, tsStr)
    })
}

// ─────────────────────────────────────────────────────────────────────────────
//  Redis conversation persistence (reused from original)
// ─────────────────────────────────────────────────────────────────────────────

func saveConversationToCache(ctx context.Context, conn *utils.Conn, userID int, key string, data *ConversationData) error {
    if data == nil {
        return errors.New("nil conversation")
    }

    now := time.Now()
    var keep []ChatMessage
    for _, m := range data.Messages {
        if m.ExpiresAt.After(now) {
            keep = append(keep, m)
        }
    }
    data.Messages = keep
    data.Timestamp = now

    b, _ := json.Marshal(data)
    return conn.Cache.Set(ctx, key, b, 0).Err()
}

func getConversationFromCache(ctx context.Context, conn *utils.Conn, userID int, key string) (*ConversationData, error) {
    val, err := conn.Cache.Get(ctx, key).Result()
    if err != nil {
        return nil, err
    }
    var c ConversationData
    if err := json.Unmarshal([]byte(val), &c); err != nil {
        return nil, err
    }
    return &c, nil
}

// ─────────────────────────────────────────────────────────────────────────────
//  Prompt loader (unchanged)
// ─────────────────────────────────────────────────────────────────────────────

func mustSystemPrompt(name string) string {
    p, err := getSystemInstruction(name)
    if err != nil {
        panic(err)
    }
    return p
}
type GetSuggestedQueriesResponse struct {
    Suggestions []string `json:"suggestions"`
}

func getSystemInstruction(systemPrompt string) (string, error) {
	// Get the directory of the current file (gemini.go)
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("error getting current file path")
	}
	currentDir := filepath.Dir(filename)

	systemPrompt = "prompts/" + systemPrompt + ".txt"
	// Construct path to query.txt
	queryFilePath := filepath.Join(currentDir, systemPrompt)

	// Read the content of query.txt
	content, err := os.ReadFile(queryFilePath)
	if err != nil {
		return "", fmt.Errorf("error reading %s: %w",queryFilePath, err)
	}

	// Replace the {{CURRENT_TIME}} placeholder with the actual current time
	currentTime := time.Now().Format(time.RFC3339)
	currentTimeMilliseconds := time.Now().UnixMilli()
	instruction := strings.Replace(string(content), "{{CURRENT_TIME}}", currentTime, -1)
	instruction = strings.Replace(instruction, "{{CURRENT_TIME_MILLISECONDS}}", fmt.Sprintf("%d", currentTimeMilliseconds), -1)

	return instruction, nil
}
func GetUserConversation(conn *utils.Conn, userID int, args json.RawMessage) (any, error) {
	ctx := context.Background()

	// Test Redis connectivity before attempting to retrieve conversation
	success, message := conn.TestRedisConnectivity(ctx, userID)
	if !success {
		fmt.Printf("WARNING: %s\n", message)
	} else {
		fmt.Println(message)
	}

	conversationKey := fmt.Sprintf("user:%d:conversation", userID)
	fmt.Println("GetUserConversation", conversationKey)

	conversation, err := getConversationFromCache(ctx, conn, userID, conversationKey)
	if err != nil {
		// Handle the case when conversation doesn't exist in cache
		if strings.Contains(err.Error(), "redis: nil") {
			fmt.Println("No conversation found in cache, returning empty history")
			// Return empty conversation history instead of error
			return &ConversationData{
				Messages:  []ChatMessage{},
				Timestamp: time.Now(),
			}, nil
		}
		return nil, fmt.Errorf("failed to get user conversation: %w", err)
	}

	// Log the conversation data for debugging
	fmt.Printf("Retrieved conversation: %+v\n", conversation)
	if conversation != nil {
		fmt.Printf("Number of messages: %d\n", len(conversation.Messages))
	}

	// Ensure we're returning valid data
	if conversation == nil || len(conversation.Messages) == 0 {
		fmt.Println("Conversation was retrieved but has no messages, returning empty history")
		return &ConversationData{
			Messages:  []ChatMessage{},
			Timestamp: time.Now(),
		}, nil
	}

	return conversation, nil
}

func GetSuggestedQueries(conn *utils.Conn, userID int, args json.RawMessage) (any, error) {
    ctx := context.Background()
    success, msg := conn.TestRedisConnectivity(ctx, userID)
    if !success { fmt.Println("WARNING:", msg) }

    key := fmt.Sprintf("user:%d:conversation", userID)
    conv, _ := getConversationFromCache(ctx, conn, userID, key)
    if conv == nil { return GetSuggestedQueriesResponse{}, nil }

    // Create a temporary agent to use its callGemini method
    agent, err := NewAgent(ctx, conn, userID)
    if err != nil {
        return nil, fmt.Errorf("failed to create agent: %w", err)
    }

    hist := buildHistoryPrompt(conv)
    res, err := agent.callGemini("gemini-2.0-flash-001", mustSystemPrompt("suggestions"), hist, false)
    if err != nil { return nil, err }

    start, end := strings.Index(res.Text, "{"), strings.LastIndex(res.Text, "}")
    if start == -1 || end == -1 { return GetSuggestedQueriesResponse{}, nil }

    var out GetSuggestedQueriesResponse
    _ = json.Unmarshal([]byte(res.Text[start:end+1]), &out) // ignore err → empty
    return out, nil
}

func ClearConversationHistory(conn *utils.Conn, userID int, _ json.RawMessage) (any, error) {
    ctx := context.Background()
    key := fmt.Sprintf("user:%d:conversation", userID)
    if err := conn.Cache.Del(ctx, key).Err(); err != nil {
        return nil, fmt.Errorf("failed to clear conversation history: %w", err)
    }
    return map[string]string{"message": "Conversation history cleared successfully"}, nil
}

