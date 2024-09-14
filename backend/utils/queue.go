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
    return json.RawMessage([]byte(task)), nil
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

    fmt.Println(funcName)
    fmt.Println(arguments)
    if err := conn.Cache.LPush(context.Background(), "queue", serializedTask).Err(); err != nil {
        return "", err
    }
    err = conn.Cache.Set(context.Background(), id, "queued",0).Err()
    if err != nil {
        return "", err
    }
    return id, nil
}
    



