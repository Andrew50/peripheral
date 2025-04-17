package agent

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/tmc/langchaingo/agents"
    "github.com/tmc/langchaingo/chains"
    "github.com/tmc/langchaingo/llms/googleai" // Gemini provider  ([eli.thegreenplace.net](https://eli.thegreenplace.net/2024/using-gemini-models-in-go-with-langchaingo/?utm_source=chatgpt.com))
    ltools "github.com/tmc/langchaingo/tools"

    "backend/utils"
)

// Runner wraps an agents.Executor so callers only care about Run(question).
// It is cheap to create; reuse it per request for statelessness.

type Runner struct {
    exec *agents.Executor
}

func NewRunner(conn *utils.Conn, userID int) (*Runner, error) {
    // LangChain‑Go constructs LLMs with functional options.
    llm, err := googleai.New(
        googleai.WithAPIKey(conn.GoogleAPIKey),
        googleai.WithModel("gemini‑1.5‑pro‑latest"),
    )
    if err != nil {
        return nil, fmt.Errorf("init gemini: %w", err)
    }

    tools := []ltools.Tool{
        NewRedisHistoryTool(conn, userID), // ⇣ defined in tools_adapters.go
        ltools.Calculator{},               // built‑in calc tool
        // …wrap any other legacy functions here…
    }

    agent := agents.NewOneShotAgent(llm, tools, agents.WithMaxIterations(5))
    return &Runner{exec: agents.NewExecutor(agent)}, nil
}

// Run executes a one‑shot MRKL chain.
func (r *Runner) Run(ctx context.Context, question string) (string, error) {
    return chains.Run(ctx, r.exec, question)
}
