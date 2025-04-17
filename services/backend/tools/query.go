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
    Result       interface{} `json:"result,omitempty"`
    Error        string      `json:"error,omitempty"`
    Args         interface{} `json:"args,omitempty"`
}

type ContentChunk struct {
    Type    string      `json:"type"`
    Content interface{} `json:"content"`
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
func GetQuery(conn *utils.Conn, userID int, args json.RawMessage) (interface{}, error) {
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

func (a *Agent) loop() error {
    var allResults []ExecuteResult

    for round := 0; round < a.MaxRounds; round++ {
        // PLAN
        calls, err := a.plan()
        if err != nil {
            return err
        }
        if len(calls) == 0 {
            break
        }

        // EXECUTE
        results := a.execute(calls)
        allResults = append(allResults, results...)

        // REFLECT — decide to stop or continue
        done, reflectText, err := a.reflect(results)
        if err != nil {
            return err
        }
        if done {
            // Build the final QueryResponse matching old behaviour.
            a.finalResponse = &QueryResponse{
                Type:          "function_calls",
                Text:          reflectText,
                Results:       allResults,
                History:       a.History,
            }
            return nil
        }
    }

    // Fallback text if max rounds hit.
    a.finalResponse = &QueryResponse{
        Type:    "text",
        Text:    "Reached maximum reasoning rounds without completion.",
        History: a.History,
    }
    return nil
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

func (a *Agent) plan() ([]FunctionCall, error) {
    const model = "gemini-2.0-flash-thinking-exp-01-21"

    // Put the whole history *after* the tool list so the model sees definitions first.
    prompt := buildToolPlaintext() + "\n\n" +
        buildHistoryPrompt(a.History) + "\nUser: "

    // ⚠️  NO function‑call metadata – the model rejects it.  We therefore
    //     pass withTools=false.
    resp, err := a.callGemini(model, mustSystemPrompt("plan"), prompt, false)
    if err != nil {
        return nil, err
    }

    // If function‑calling were somehow enabled we still honour it,
    // otherwise fall back to the JSON list in `resp.Text`.
    if len(resp.FunctionCalls) > 0 {
        return resp.FunctionCalls, nil
    }

    // Parse a JSON array inside the assistant text.
    start, end := strings.Index(resp.Text, "["), strings.LastIndex(resp.Text, "]")
    if start == -1 || end == -1 || end <= start {
        return nil, fmt.Errorf("plan: no function calls returned")
    }

    var rawCalls []struct {
        Name string          `json:"name"`
        Args json.RawMessage `json:"args"`
    }
    if err := json.Unmarshal([]byte(resp.Text[start:end+1]), &rawCalls); err != nil {
        return nil, fmt.Errorf("plan: cannot parse JSON tool list: %w", err)
    }

    out := make([]FunctionCall, len(rawCalls))
    for i, c := range rawCalls {
        out[i] = FunctionCall{Name: c.Name, Args: c.Args}
    }
    return out, nil
}

func (a *Agent) execute(calls []FunctionCall) []ExecuteResult {
    tools := GetTools(false)
    var results []ExecuteResult
    for _, c := range calls {
        tool, ok := tools[c.Name]
        if !ok {
            results = append(results, ExecuteResult{FunctionName: c.Name, Error: "tool not found", Args: c.Args})
            continue
        }
        res, err := tool.Function(a.Conn, a.UserID, c.Args)
        if err != nil {
            results = append(results, ExecuteResult{FunctionName: c.Name, Error: err.Error(), Args: c.Args})
        } else {
            results = append(results, ExecuteResult{FunctionName: c.Name, Result: res, Args: c.Args})
        }
    }
    // Add a tool message to history (JSON chunk)
    a.History.Messages = append(a.History.Messages, ChatMessage{
        Role:          "tool",
        ContentChunks: []ContentChunk{{Type: "json", Content: results}},
        Timestamp:     time.Now(),
        ExpiresAt:     time.Now().Add(24 * time.Hour),
    })
    return results
}

func (a *Agent) reflect(results []ExecuteResult) (bool, string, error) {
    payload, _ := json.Marshal(results)
    prompt := buildHistoryPrompt(a.History) + "\nTool results:\n```json\n" + string(payload) + "\n```"
    resp, err := a.callGemini("gemini-2.0-flash-001", mustSystemPrompt("reflect"), prompt, true)
    if err != nil {
        return false, "", err
    }

    text := strings.TrimSpace(resp.Text)
    text = injectSecurityIDs(a.Conn, text)

    a.appendAssistant(text)

    lower := strings.ToLower(text)
    done := strings.Contains(lower, "no more rounds") || strings.HasPrefix(lower, "done")
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
    ts := GetTools(false)
    names := make([]string, 0, len(ts))
    for n := range ts {
        names = append(names, n)
    }
    sort.Strings(names)

    var out []*genai.Tool
    for _, n := range names {
        t := ts[n]
        out = append(out, &genai.Tool{FunctionDeclarations: []*genai.FunctionDeclaration{t.FunctionDeclaration}})
    }
    return out
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
		return "", fmt.Errorf("error reading query.txt: %w", err)
	}

	// Replace the {{CURRENT_TIME}} placeholder with the actual current time
	currentTime := time.Now().Format(time.RFC3339)
	currentTimeMilliseconds := time.Now().UnixMilli()
	instruction := strings.Replace(string(content), "{{CURRENT_TIME}}", currentTime, -1)
	instruction = strings.Replace(instruction, "{{CURRENT_TIME_MILLISECONDS}}", fmt.Sprintf("%d", currentTimeMilliseconds), -1)

	return instruction, nil
}
func GetUserConversation(conn *utils.Conn, userID int, args json.RawMessage) (interface{}, error) {
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
    res, err := agent.callGemini("gemini-2.0-flash-001", mustSystemPrompt("suggestedQueries"), hist, false)
    if err != nil { return nil, err }

    start, end := strings.Index(res.Text, "{"), strings.LastIndex(res.Text, "}")
    if start == -1 || end == -1 { return GetSuggestedQueriesResponse{}, nil }

    var out GetSuggestedQueriesResponse
    _ = json.Unmarshal([]byte(res.Text[start:end+1]), &out) // ignore err → empty
    return out, nil
}

func ClearConversationHistory(conn *utils.Conn, userID int, _ json.RawMessage) (interface{}, error) {
    ctx := context.Background()
    key := fmt.Sprintf("user:%d:conversation", userID)
    if err := conn.Cache.Del(ctx, key).Err(); err != nil {
        return nil, fmt.Errorf("failed to clear conversation history: %w", err)
    }
    return map[string]string{"message": "Conversation history cleared successfully"}, nil
}

