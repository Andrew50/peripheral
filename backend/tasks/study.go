package tasks
import (
	"api/data"
	"context"
	"encoding/json"
	"fmt"
)

type GetStudiesArgs struct {
    Completed bool `json:"completed"`
}

type GetStudiesResult struct {
    StudyId int `json:"studyId"`
    SecurityId int `json:"securityId"`
    Ticker string `json:"ticker"`
    Datetime string `json:"datetime"`
}

func GetStudies(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetStudiesArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("GetCik invalid args: %v", err)
	}
    rows, err := conn.DB.Query(context.Background(),`
    SELECT s.studyId, s.securityId, sec.ticker, s.datetime, s.entry from studies as s 
    JOIN securities as sec on s.securityId = sec.securityId
    where s.userId = $1, s.completed = $2
    `,userId, args.Completed)
    var studies []GetStudiesResult
    for rows.Next(){
        var study GetStudiesResult
        err := rows.Scan(&study.StudyId,&study.SecurityId, &study.Ticker, &study.Datetime)
        if err != nil {
            return nil, err
        }
        studies = append( studies, study)
    }
    return studies, nil
}
