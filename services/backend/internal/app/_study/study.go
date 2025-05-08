package tools

import (
	"backend/internal/data"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// GetStudiesArgs represents a structure for handling GetStudiesArgs data.
type GetStudiesArgs struct {
	Completed bool `json:"completed"`
}

// GetStudiesResult represents a structure for handling GetStudiesResult data.
type GetStudiesResult struct {
	StudyID    int    `json:"studyId"`
	SecurityID int    `json:"securityId"`
	Ticker     string `json:"ticker"`
	Timestamp  int64  `json:"timestamp"`
	StrategyID    *int64 `json:"strategyId"` // Pointer to handle null values
	Completed  bool   `json:"completed"`
}

// GetStudies performs operations related to GetStudies functionality.
func GetStudies(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetStudiesArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("GetStudies invalid args: %v", err)
	}

	rows, err := conn.DB.Query(context.Background(), `
		SELECT s.studyId, s.securityId, s.strategyId, sec.ticker, s.timestamp, s.completed 
		FROM studies AS s 
		JOIN securities AS sec ON s.securityId = sec.securityId 
		WHERE s.userId = $1 AND s.completed = $2
	`, userId, args.Completed)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var studies []GetStudiesResult
	for rows.Next() {
		var study GetStudiesResult
		var strategyID sql.NullInt64 // Handle nullable setupId
		var studyTime time.Time

		// Scan the row data
		err := rows.Scan(&study.StudyID, &study.SecurityID, &strategyID, &study.Ticker, &studyTime, &study.Completed)
		if err != nil {
			return nil, err
		}

		// Handle nullable strategyId
		if strategyID.Valid {
			study.StrategyID = &strategyID.Int64
		} else {
			study.StrategyID = nil
		}

		study.Timestamp = studyTime.Unix()
		studies = append(studies, study)
	}

	return studies, nil
}

// SetStudyStrategyArgs represents a structure for handling SetStudyStrategyArgs data.
type SetStudyStrategyArgs struct {
	Id      int `json:"id"`
	StrategyID int `json:"strategyId"`
}

// SetStudyStrategy performs operations related to SetStudyStrategy functionality.
func SetStudyStrategy(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args SetStudyStrategyArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("3og9 invalid args: %v", err)
	}
	cmdTag, err := conn.DB.Exec(context.Background(), "UPDATE studies SET strategyId = $1 WHERE studyId = $2 AND userId = $3", args.StrategyID, args.Id, userId)
	if cmdTag.RowsAffected() == 0 {
		return nil, fmt.Errorf("study not found or you don't have permission to update it")
	}
	return nil, err
}

// SaveStudyArgs represents a structure for handling SaveStudyArgs data.
type SaveStudyArgs struct {
	Id    int             `json:"id"`
	Entry json.RawMessage `json:"entry"`
}

// SaveStudy performs operations related to SaveStudy functionality.
func SaveStudy(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args SaveStudyArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("3og9 invalid args: %v", err)
	}
	cmdTag, err := conn.DB.Exec(context.Background(), "UPDATE studies SET entry = $1 WHERE studyId = $2 AND userId = $3", args.Entry, args.Id, userId)
	if cmdTag.RowsAffected() == 0 {
		return nil, fmt.Errorf("study not found or you don't have permission to update it")
	}
	return nil, err
}

// CompleteStudyArgs represents a structure for handling CompleteStudyArgs data.
type CompleteStudyArgs struct {
	Id        int  `json:"id"`
	Completed bool `json:"completed"`
}

// CompleteStudy performs operations related to CompleteStudy functionality.
func CompleteStudy(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args CompleteStudyArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("215d invalid args: %v", err)
	}
	cmdTag, err := conn.DB.Exec(context.Background(), "UPDATE studies SET completed = $1 WHERE studyId = $2 AND userId = $3", args.Completed, args.Id, userId)
	if cmdTag.RowsAffected() == 0 {
		return nil, fmt.Errorf("study not found or you don't have permission to update it")
	}
	return nil, err
}

// DeleteStudyArgs represents a structure for handling DeleteStudyArgs data.
type DeleteStudyArgs struct {
	Id int `json:"id"`
}

// DeleteStudy performs operations related to DeleteStudy functionality.
func DeleteStudy(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args DeleteStudyArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("GetCik invalid args: %v", err)
	}
	cmdTag, err := conn.DB.Exec(context.Background(), "DELETE FROM studies where studyId = $1 AND userId = $2", args.Id, userId)
	if err != nil {
		return nil, err
	}
	if cmdTag.RowsAffected() == 0 {
		return nil, fmt.Errorf("study not found or you don't have permission to delete it")
	}
	return nil, err
}

// GetStudyEntryArgs represents a structure for handling GetStudyEntryArgs data.
type GetStudyEntryArgs struct {
	StudyID int `json:"studyId"`
}

// GetStudyEntry performs operations related to GetStudyEntry functionality.
func GetStudyEntry(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetStudyEntryArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("GetCik invalid args: %v", err)
	}
	var entry json.RawMessage
	err = conn.DB.QueryRow(context.Background(), "SELECT entry from studies where studyId = $1 AND userId = $2", args.StudyID, userId).Scan(&entry)
	if err != nil {
		return nil, err
	}
	return entry, nil
}

// NewStudyArgs represents a structure for handling NewStudyArgs data.
type NewStudyArgs struct {
	SecurityID int   `json:"securityId"`
	Timestamp  int64 `json:"timestamp"`
}

// NewStudy performs operations related to NewStudy functionality.
func NewStudy(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args NewStudyArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("GetCik invalid args: %v", err)
	}
	timestamp := time.Unix(args.Timestamp, 0)
	var studyID int
	err = conn.DB.QueryRow(context.Background(), "INSERT into studies (userId,securityId, timestamp) values ($1,$2,$3) RETURNING studyId", userId, args.SecurityID, timestamp).Scan(&studyID)
	if err != nil {
		return nil, err
	}
	return studyID, err
}
