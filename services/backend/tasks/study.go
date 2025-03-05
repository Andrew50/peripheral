package tasks

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"time"
    "database/sql"
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
	SetupID    *int64 `json:"setupId"` // Pointer to handle null values
	Completed  bool   `json:"completed"`
}
// GetStudies performs operations related to GetStudies functionality.
func GetStudies(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetStudiesArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("GetStudies invalid args: %v", err)
	}
	
	rows, err := conn.DB.Query(context.Background(), `
		SELECT s.studyId, s.securityId, s.setupId, sec.ticker, s.timestamp, s.completed 
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
		var setupID sql.NullInt64 // Handle nullable setupId
		var studyTime time.Time

		// Scan the row data
		err := rows.Scan(&study.StudyID, &study.SecurityID, &setupId, &study.Ticker, &studyTime, &study.Completed)
		if err != nil {
			return nil, err
		}

		// Handle nullable setupId
		if setupId.Valid {
			study.SetupID = &setupId.Int64
		} else {
			study.SetupID = nil
		}

		study.Timestamp = studyTime.Unix()
		studies = append(studies, study)
	}

	return studies, nil
}
// SetStudySetupArgs represents a structure for handling SetStudySetupArgs data.
type SetStudySetupArgs struct {
    Id int  `json:"id"`
    SetupID int `json:"setupId"`
// SetStudySetup performs operations related to SetStudySetup functionality.
func SetStudySetup(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args SetStudySetupArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("3og9 invalid args: %v", err)
	}
	cmdTag, err := conn.DB.Exec(context.Background(), "UPDATE studies Set setupId = $1 where studyId = $2", args.SetupID, args.Id)
	if cmdTag.RowsAffected() == 0 {
		return nil, fmt.Errorf("0n8912")
	}
	return nil, err
}
// SaveStudyArgs represents a structure for handling SaveStudyArgs data.
type SaveStudyArgs struct {
	Id    int             `json:"id"`
	Entry json.RawMessage `json:"entry"`
}
// SaveStudy performs operations related to SaveStudy functionality.
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
// CompleteStudyArgs represents a structure for handling CompleteStudyArgs data.
type CompleteStudyArgs struct {
	Id        int  `json:"id"`
	Completed bool `json:"completed"`
}
// CompleteStudy performs operations related to CompleteStudy functionality.
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
// DeleteStudyArgs represents a structure for handling DeleteStudyArgs data.
type DeleteStudyArgs struct {
	Id int `json:"id"`
}
// DeleteStudy performs operations related to DeleteStudy functionality.
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
// GetStudyEntryArgs represents a structure for handling GetStudyEntryArgs data.
type GetStudyEntryArgs struct {
	StudyID int `json:"studyId"`
}
// GetStudyEntry performs operations related to GetStudyEntry functionality.
func GetStudyEntry(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetStudyEntryArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("GetCik invalid args: %v", err)
	}
	var entry json.RawMessage
	err = conn.DB.QueryRow(context.Background(), "SELECT entry from studies where studyId = $1", args.StudyID).Scan(&entry)
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
func NewStudy(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args NewStudyArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("GetCik invalid args: %v", err)
	}
	timestamp := time.Unix(args.Timestamp, 0)
	var studyID int
	err = conn.DB.QueryRow(context.Background(), "INSERT into studies (userId,securityId, timestamp) values ($1,$2,$3) RETURNING studyId", userId, args.SecurityID, timestamp).Scan(&studyId)
	if err != nil {
		return nil, err
	}
	return studyId, err
}
