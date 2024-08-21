package tasks

import (
    "api/data"
    "encoding/json"
    "fmt"
)



type NewInstanceArgs struct {

}




func NewInstance (conn *data.Conn, user_id int, rawArgs json.RawMessage) (interface{}, error) {
    var args NewInstanceArgs
    if err := json.Unmarshal(rawArgs, &args); err != nil {
        return nil, fmt.Errorf("NewInstance invalid args: %v", err)
    }

    return nil, nil 
}
