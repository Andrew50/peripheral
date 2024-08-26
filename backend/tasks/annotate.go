package tasks

import (
    "fmt"
    "encoding/json"
    "context"
    "api/data"

)

type GetAnnotationArgs struct {
    InstanceId int `json:"instanceId"`
}

type GetAnnotationResults struct {
    AnnotationId int `json:"annotationId"`
    Timeframe string `json:"timeframe"`
    Entry string `json:"entry"`
}

func GetAnnotations(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
    var args GetAnnotationArgs
    if err := json.Unmarshal(rawArgs, &args); err != nil {
        return nil, fmt.Errorf("getAnnotations invalid args: %v", err)
    }
    rows, err := conn.DB.Query(context.Background(), "SELECT annotationId, timeframe, entry FROM annotations WHERE instanceId = $1", args.InstanceId)
    if err != nil {
        return nil, err
    }
    var annotations []GetAnnotationResults
    for rows.Next() {
        var annotation GetAnnotationResults
        if err := rows.Scan(&annotation.AnnotationId, &annotation.Timeframe, &annotation.Entry); err != nil {
            return nil, err
        }
        annotations = append(annotations, annotation)
    }
    return annotations, nil
}

type NewAnnotationArgs struct {
    InstanceId int `json:"instanceId"`
    Timeframe string `json:"timeframe"`
}

func NewAnnotation(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
    var args NewAnnotationArgs
    if err := json.Unmarshal(rawArgs, &args); err != nil {
        return nil, fmt.Errorf("getAnnotations invalid args: %v", err)
    }
	var annotationId int
    err := conn.DB.QueryRow(context.Background(), "INSERT INTO annotations(instanceId, timeframe) VALUES ($1, $2) RETURNING annotationId", args.InstanceId, args.Timeframe).Scan(&annotationId)
    if err != nil {
        return nil, fmt.Errorf("SetAnnotation execution failed: %v", err)
    }
	return annotationId, nil
}


type SetAnnotationArgs struct {
	AnnotationId int    `json:"annotationId"`
	Entry        string `json:"entry"`
}

func SetAnnotation(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
    var args SetAnnotationArgs
    if err := json.Unmarshal(rawArgs, &args); err != nil {
        return nil, fmt.Errorf("getAnnotations invalid args: %v", err)
    }

    cmdTag, err := conn.DB.Exec(context.Background(), "UPDATE annotations SET entry = $1 WHERE annotationId = $2", args.Entry, args.AnnotationId)
    if err != nil {
        return nil, fmt.Errorf("SetAnnotation execution failed: %v", err)
    }

	// Check if any rows were affected, this ensures the annotation_id existed, 
	if cmdTag.RowsAffected() == 0 {
		return nil, fmt.Errorf("SetAnnotation no annotation found with the provided annotationId and userId")
	}

    return "success", nil

}



