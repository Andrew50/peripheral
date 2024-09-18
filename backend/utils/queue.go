package utils

import (
    "github.com/google/uuid"
    "encoding/json"
    "context"
    "fmt"
)

func Poll(conn *Conn, taskId string)(json.RawMessage, error){
    task := conn.Cache.Get(context.Background(), taskId).Val()
    if task == "" {
        return nil, fmt.Errorf("weh3")
    }
    result := json.RawMessage([]byte(task)) //its already json and you dont care about its contents utnil frontend so just push the json
    return result,nil
}
type QueueArgs struct {
    ID string `json:"id"`
    Func string `json:"func"`
    Args interface{} `json:"args"`
}

type queueResponse struct {
    TaskId string `json:"taskId"`
}

func Queue(conn *Conn, funcName string, arguments interface{}) (string, error){
    id := uuid.New().String()
    taskArgs := QueueArgs{
        ID: id,
        Func: funcName,
        Args: arguments,
    }
    serializedTask, err := json.Marshal(taskArgs)
    if err != nil {
        return "", err
    }

    if err := conn.Cache.LPush(context.Background(), "queue", serializedTask).Err(); err != nil {
        return "", err
    }
    serializedStatus, err := json.Marshal("queued")
    if err != nil {
        return "", fmt.Errorf("error marshaling task status: %w", err)
    }
    if err := conn.Cache.Set(context.Background(), id, serializedStatus, 0).Err(); err != nil {
        return "", fmt.Errorf("error setting task status: %w", err)
    }
    return id, nil
}
    



