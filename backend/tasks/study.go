package tasks

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type GetStudiesArgs struct {
	Completed bool `json:"completed"`
}

type GetStudiesResult struct {
	StudyId    int    `json:"studyId"`
	SecurityId int    `json:"securityId"`
	Ticker     string `json:"ticker"`
	Timestamp  int64  `json:"timestamp"`
	Completed  bool   `json:"completed"`
}

func GetStudies(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetStudiesArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("GetCik invalid args: %v", err)
	}
	rows, err := conn.DB.Query(context.Background(), `
    SELECT s.studyId, s.securityId, sec.ticker, s.timestamp, s.completed from studies as s 
    JOIN securities as sec on s.securityId = sec.securityId
    where s.userId = $1 and s.completed = $2
    `, userId, args.Completed)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var studies []GetStudiesResult
	for rows.Next() {
		var study GetStudiesResult
		var studyTime time.Time
		err := rows.Scan(&study.StudyId, &study.SecurityId, &study.Ticker, &studyTime, &study.Completed)
		if err != nil {
			return nil, err
		}
		study.Timestamp = studyTime.Unix()
		studies = append(studies, study)
	}
	return studies, nil
}

type SaveStudyArgs struct {
	Id    int             `json:"id"`
	Entry json.RawMessage `json:"entry"`
}

func SaveStudy(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args SaveStudyArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("3og9 invalid args: %v", err)
	}
	cmdTag, err := conn.DB.Exec(context.Background(), "UPDATE studies Set entry = $1 where studyId = $2", args.Entry, args.Id)
	if cmdTag.RowsAffected() == 0 {
		return nil, fmt.Errorf("0n8912")
	}
	return nil, err
}

type CompleteStudyArgs struct {
	Id        int  `json:"id"`
	Completed bool `json:"completed"`
}

func CompleteStudy(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args CompleteStudyArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("215d invalid args: %v", err)
	}
	cmdTag, err := conn.DB.Exec(context.Background(), "UPDATE studies Set completed = $1 where studyId = $2", args.Completed, args.Id)
	if cmdTag.RowsAffected() == 0 {
		return nil, fmt.Errorf("0n8912")
	}
	return nil, err
}

type DeleteStudyArgs struct {
	Id int `json:"id"`
}

func DeleteStudy(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args DeleteStudyArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("GetCik invalid args: %v", err)
	}
	cmdTag, err := conn.DB.Exec(context.Background(), "DELETE FROM studies where studyId = $1", args.Id)
	if err != nil {
		return nil, err
	}
	if cmdTag.RowsAffected() == 0 {
		return nil, fmt.Errorf("ssd7g3")
	}
	return nil, err
}

type GetStudyEntryArgs struct {
	StudyId int `json:"studyId"`
}

func GetStudyEntry(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetStudyEntryArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("GetCik invalid args: %v", err)
	}
	var entry json.RawMessage
	err = conn.DB.QueryRow(context.Background(), "SELECT entry from studies where studyId = $1", args.StudyId).Scan(&entry)
	if err != nil {
		return nil, err
	}
	return entry, nil
}

type NewStudyArgs struct {
	SecurityId int   `json:"securityId"`
	Timestamp  int64 `json:"timestamp"`
}

func NewStudy(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args NewStudyArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("GetCik invalid args: %v", err)
	}
	timestamp := time.Unix(args.Timestamp, 0)
	var studyId int
	err = conn.DB.QueryRow(context.Background(), "INSERT into studies (userId,securityId, timestamp) values ($1,$2,$3) RETURNING studyId", userId, args.SecurityId, timestamp).Scan(&studyId)
	if err != nil {
		return nil, err
	}
	return studyId, err
}
