package tasks
import (
	"api/data"
	"context"
	"encoding/json"
	"fmt"
    "time"
)

type GetStudiesArgs struct {
    Completed bool `json:"completed"`
}

type GetStudiesResult struct {
    StudyId int `json:"studyId"`
    SecurityId int `json:"securityId"`
    Ticker string `json:"ticker"`
    Datetime time.Time `json:"datetime"`
}

func GetStudies(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetStudiesArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("GetCik invalid args: %v", err)
	}
    rows, err := conn.DB.Query(context.Background(),`
    SELECT s.studyId, s.securityId, sec.ticker, s.datetime from studies as s 
    JOIN securities as sec on s.securityId = sec.securityId
    where s.userId = $1 and s.completed = $2
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

type SetStudyEntryArgs struct {
    StudyId int `json:"studyId"`
    Entry json.RawMessage `json:"entry"`
}

func SetStudyEntry(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args SetStudyEntryArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("GetCik invalid args: %v", err)
	}
    _, err = conn.DB.Exec(context.Background(), "UPDATE studies Set entry = $1 where studyId = $2",args.Entry, args.StudyId)
    return nil, err
}


type GetStudyEntryArgs struct {
    StudyId int `json:"studyId"`
}

func GetStudyEntry(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetStudyEntryArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("GetCik invalid args: %v", err)
	}
    var entry json.RawMessage
    err = conn.DB.QueryRow(context.Background(),"SELECT entry from studies where studyId = $1",args.StudyId).Scan(&entry)
    if err != nil {
        return nil, err
    }
    return entry, nil
}

type NewStudyArgs struct {
    SecurityId int `json:"securityId"`
    Datetime string `json:"datetime"`
}

func NewStudy(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args NewStudyArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("GetCik invalid args: %v", err)
	}
    var studyId int
    err = conn.DB.QueryRow(context.Background(),"INSERT into studies (userId,securityId, datetime) values ($1,$2,$3) RETURNING studyId",userId,args.SecurityId,args.Datetime).Scan(&studyId)
    if err != nil {
        return nil, err
    }
    return studyId, err
}


