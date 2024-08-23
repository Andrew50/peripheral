package tasks

import (
    "fmt"
    "encoding/json"
    "context"
    "api/data"

)

type SetAnnotationArgs struct {
	AnnotationId int    `json:"a1"`
	Entry        string `json:"a2"`
}

func SetAnnotation(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
    var args SetAnnotationArgs
    if err := json.Unmarshal(rawArgs, &args); err != nil {
        return nil, fmt.Errorf("getAnnotations invalid args: %v", err)
    }

    cmdTag, err := conn.DB.Exec(context.Background(), "UPDATE annotations SET entry = $1, completed = true WHERE annotation_id = $2 AND user_id = $3", args.Entry, args.AnnotationId, userId)
    if err != nil {
        return nil, fmt.Errorf("SetAnnotation execution failed: %v", err)
    }

	// Check if any rows were affected, this ensures the annotation_id existed
	if cmdTag.RowsAffected() == 0 {
		return nil, fmt.Errorf("SetAnnotation no annotation found with the provided annotation_id and user_id")
	}

    return "success", nil

}



