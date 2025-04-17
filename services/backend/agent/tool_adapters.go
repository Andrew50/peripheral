package agent

import (
    "context"
    "encoding/json"

    ltools "github.com/tmc/langchaingo/tools"

    "backend/utils"
)

// RedisHistoryTool exposes cached conversation context as a LangChain tool.
// Returns JSON so the LLM can parse or pass on.

type RedisHistoryTool struct {
    Conn   *utils.Conn
    UserID int
}

func (t RedisHistoryTool) Name() string        { return "redis_history" }
func (t RedisHistoryTool) Description() string { return "Return the user\'s last 10 conversation messages as JSON" }

func (t RedisHistoryTool) Call(ctx context.Context, _ string) (string, error) {
    convKey := "user:" + fmt.Sprint(t.UserID) + ":conversation"
    data, err := t.Conn.Cache.Get(ctx, convKey).Result()
    if err != nil {
        return "{}", nil // empty history is OK
    }
    return data, nil
}

// Generic wrapper for legacy tool funcs so you don’t have to hand‑code every schema.

func WrapFunc(name, desc string, fn func(*utils.Conn, int, json.RawMessage) (interface{}, error), conn *utils.Conn, uid int) ltools.Tool {
    return funcTool{name: name, desc: desc, inner: fn, conn: conn, uid: uid}
}

type funcTool struct {
    name string
    desc string
    inner func(*utils.Conn, int, json.RawMessage) (interface{}, error)
    conn *utils.Conn
    uid  int
}

func (f funcTool) Name() string        { return f.name }
func (f funcTool) Description() string { return f.desc }

func (f funcTool) Call(ctx context.Context, arg string) (string, error) {
    var raw json.RawMessage = json.RawMessage(arg)
    out, err := f.inner(f.conn, f.uid, raw)
    if err != nil {
        return "", err
    }
    b, _ := json.Marshal(out)
    return string(b), nil
}
