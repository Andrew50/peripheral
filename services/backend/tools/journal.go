package tools

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// GetJournalsResult represents a structure for handling GetJournalsResult data.
type GetJournalsResult struct {
	JournalID int   `json:"journalId"`
	Timestamp int64 `json:"timestamp"`
	Completed bool  `json:"completed"`
}

// GetJournals performs operations related to GetJournals functionality.
func GetJournals(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	rows, err := conn.DB.Query(context.Background(), "SELECT journalId, timestamp, completed from journals where userId = $1 order by timestamp desc", userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var journals []GetJournalsResult
	for rows.Next() {
		var journal GetJournalsResult
		var journalTime time.Time
		err := rows.Scan(&journal.JournalID, &journalTime, &journal.Completed)
		if err != nil {
			return nil, fmt.Errorf("19nv %v", err)
		}
		journal.Timestamp = journalTime.Unix() * 1000
		journals = append(journals, journal)
	}
	return journals, nil
}

// SaveJournalArgs represents a structure for handling SaveJournalArgs data.
type SaveJournalArgs struct {
	Id    int             `json:"id"`
	Entry json.RawMessage `json:"entry"`
}

// SaveJournal performs operations related to SaveJournal functionality.
func SaveJournal(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args SaveJournalArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("3og9 invalid args: %v", err)
	}
	cmdTag, err := conn.DB.Exec(context.Background(), "UPDATE journals Set entry = $1 where journalId = $2", args.Entry, args.Id)
	if cmdTag.RowsAffected() == 0 {
		return nil, fmt.Errorf("0n8912")
	}
	return nil, err
}

// CompleteJournalArgs represents a structure for handling CompleteJournalArgs data.
type CompleteJournalArgs struct {
	Id        int  `json:"id"`
	Completed bool `json:"completed"`
}

// CompleteJournal performs operations related to CompleteJournal functionality.
func CompleteJournal(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args CompleteJournalArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("215d invalid args: %v", err)
	}
	cmdTag, err := conn.DB.Exec(context.Background(), "UPDATE journals Set completed = $1 where journalId = $2", args.Completed, args.Id)
	if cmdTag.RowsAffected() == 0 {
		return nil, fmt.Errorf("0n8912")
	}
	return nil, err
}

// DeleteJournalArgs represents a structure for handling DeleteJournalArgs data.
type DeleteJournalArgs struct {
	Id int `json:"id"`
}

// DeleteJournal performs operations related to DeleteJournal functionality.
func DeleteJournal(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args DeleteJournalArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("GetCik invalid args: %v", err)
	}
	cmdTag, err := conn.DB.Exec(context.Background(), "DELETE FROM journals where journalId = $1", args.Id)
	if err != nil {
		return nil, err
	}
	if cmdTag.RowsAffected() == 0 {
		return nil, fmt.Errorf("ssd7g3")
	}
	return nil, err
}

// GetJournalEntryArgs represents a structure for handling GetJournalEntryArgs data.
type GetJournalEntryArgs struct {
	JournalID int `json:"journalId"`
}

// GetJournalEntry performs operations related to GetJournalEntry functionality.
func GetJournalEntry(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetJournalEntryArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("GetCik invalid args: %v", err)
	}
	var entry json.RawMessage
	err = conn.DB.QueryRow(context.Background(), "SELECT entry from journals where journalId = $1", args.JournalID).Scan(&entry)
	if err != nil {
		return nil, err
	}
	return entry, nil
}
