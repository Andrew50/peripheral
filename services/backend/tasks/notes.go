package tasks

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Note represents a user note
type Note struct {
	NoteID     int       `json:"noteId"`
	UserID     int       `json:"userId"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	Category   string    `json:"category"`
	Tags       []string  `json:"tags"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	IsPinned   bool      `json:"isPinned"`
	IsArchived bool      `json:"isArchived"`
}

// GetNotesArgs represents arguments for retrieving notes
type GetNotesArgs struct {
	Category    string   `json:"category,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	IsPinned    *bool    `json:"isPinned,omitempty"`
	IsArchived  *bool    `json:"isArchived,omitempty"`
	SearchQuery string   `json:"searchQuery,omitempty"`
}

// GetNotes retrieves notes for the current user with optional filtering
func GetNotes(conn *utils.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetNotesArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing args: %v", err)
	}

	// Start building the query
	query := `
		SELECT noteId, userId, title, content, category, tags, created_at, updated_at, is_pinned, is_archived
		FROM notes
		WHERE userId = $1
	`
	params := []interface{}{userID}
	paramCount := 2

	// Add filters if provided
	if args.Category != "" {
		query += fmt.Sprintf(" AND category = $%d", paramCount)
		params = append(params, args.Category)
		paramCount++
	}

	if len(args.Tags) > 0 {
		query += fmt.Sprintf(" AND tags && $%d", paramCount)
		params = append(params, args.Tags)
		paramCount++
	}

	if args.IsPinned != nil {
		query += fmt.Sprintf(" AND is_pinned = $%d", paramCount)
		params = append(params, *args.IsPinned)
		paramCount++
	}

	if args.IsArchived != nil {
		query += fmt.Sprintf(" AND is_archived = $%d", paramCount)
		params = append(params, *args.IsArchived)
		paramCount++
	}

	// Add full-text search if a search query is provided
	if args.SearchQuery != "" {
		query += fmt.Sprintf(" AND search_vector @@ plainto_tsquery('english', $%d)", paramCount)
		params = append(params, args.SearchQuery)
		paramCount++

		// Add ranking for search results
		query += fmt.Sprintf(" ORDER BY ts_rank(search_vector, plainto_tsquery('english', $%d)) DESC", paramCount-1)
	} else {
		// Default ordering if no search query
		query += " ORDER BY is_pinned DESC, updated_at DESC"
	}

	// Execute the query
	rows, err := conn.DB.Query(context.Background(), query, params...)
	if err != nil {
		return nil, fmt.Errorf("error querying notes: %v", err)
	}
	defer rows.Close()

	// Process the results
	var notes []Note
	for rows.Next() {
		var note Note
		if err := rows.Scan(
			&note.NoteID,
			&note.UserID,
			&note.Title,
			&note.Content,
			&note.Category,
			&note.Tags,
			&note.CreatedAt,
			&note.UpdatedAt,
			&note.IsPinned,
			&note.IsArchived,
		); err != nil {
			return nil, fmt.Errorf("error scanning note row: %v", err)
		}
		notes = append(notes, note)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating note rows: %v", err)
	}

	return notes, nil
}

// SearchNotesArgs represents arguments for searching notes
type SearchNotesArgs struct {
	Query      string `json:"query"`
	IsArchived *bool  `json:"isArchived,omitempty"`
}

// SearchNotes performs a full-text search on notes
func SearchNotes(conn *utils.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args SearchNotesArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing args: %v", err)
	}

	if args.Query == "" {
		return nil, fmt.Errorf("search query is required")
	}

	// Build the query with ranking
	query := `
		SELECT 
			noteId, userId, title, content, category, tags, created_at, updated_at, is_pinned, is_archived,
			ts_rank(search_vector, plainto_tsquery('english', $2)) AS rank,
			ts_headline('english', title, plainto_tsquery('english', $2), 'StartSel=<mark>, StopSel=</mark>') AS title_highlight,
			ts_headline('english', content, plainto_tsquery('english', $2), 'StartSel=<mark>, StopSel=</mark>, MaxFragments=3, MaxWords=50, MinWords=15') AS content_highlight
		FROM notes
		WHERE userId = $1 AND search_vector @@ plainto_tsquery('english', $2)
	`
	params := []interface{}{userID, args.Query}
	paramCount := 3

	// Add archived filter if provided
	if args.IsArchived != nil {
		query += fmt.Sprintf(" AND is_archived = $%d", paramCount)
		params = append(params, *args.IsArchived)
		paramCount++
	}

	// Order by rank
	query += " ORDER BY rank DESC, is_pinned DESC, updated_at DESC"

	// Execute the query
	rows, err := conn.DB.Query(context.Background(), query, params...)
	if err != nil {
		return nil, fmt.Errorf("error searching notes: %v", err)
	}
	defer rows.Close()

	// Define a struct for search results with highlights
	type SearchResult struct {
		Note             Note    `json:"note"`
		Rank             float64 `json:"rank"`
		TitleHighlight   string  `json:"titleHighlight"`
		ContentHighlight string  `json:"contentHighlight"`
	}

	// Process the results
	var results []SearchResult
	for rows.Next() {
		var note Note
		var rank float64
		var titleHighlight, contentHighlight string

		if err := rows.Scan(
			&note.NoteID,
			&note.UserID,
			&note.Title,
			&note.Content,
			&note.Category,
			&note.Tags,
			&note.CreatedAt,
			&note.UpdatedAt,
			&note.IsPinned,
			&note.IsArchived,
			&rank,
			&titleHighlight,
			&contentHighlight,
		); err != nil {
			return nil, fmt.Errorf("error scanning search result row: %v", err)
		}

		results = append(results, SearchResult{
			Note:             note,
			Rank:             rank,
			TitleHighlight:   titleHighlight,
			ContentHighlight: contentHighlight,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating search result rows: %v", err)
	}

	return results, nil
}

// GetNoteArgs represents arguments for retrieving a single note
type GetNoteArgs struct {
	NoteID int `json:"noteId"`
}

// GetNote retrieves a single note by ID
func GetNote(conn *utils.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetNoteArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing args: %v", err)
	}

	var note Note
	err := conn.DB.QueryRow(context.Background(), `
		SELECT noteId, userId, title, content, category, tags, created_at, updated_at, is_pinned, is_archived
		FROM notes
		WHERE noteId = $1 AND userId = $2
	`, args.NoteID, userID).Scan(
		&note.NoteID,
		&note.UserID,
		&note.Title,
		&note.Content,
		&note.Category,
		&note.Tags,
		&note.CreatedAt,
		&note.UpdatedAt,
		&note.IsPinned,
		&note.IsArchived,
	)

	if err != nil {
		return nil, fmt.Errorf("error retrieving note: %v", err)
	}

	return note, nil
}

// CreateNoteArgs represents arguments for creating a new note
type CreateNoteArgs struct {
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	Category   string   `json:"category"`
	Tags       []string `json:"tags"`
	IsPinned   bool     `json:"isPinned"`
	IsArchived bool     `json:"isArchived"`
}

// CreateNote creates a new note for the user
func CreateNote(conn *utils.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args CreateNoteArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing args: %v", err)
	}

	// Validate required fields
	if args.Title == "" {
		return nil, fmt.Errorf("title is required")
	}

	var noteID int
	err := conn.DB.QueryRow(context.Background(), `
		INSERT INTO notes (userId, title, content, category, tags, is_pinned, is_archived)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING noteId
	`, userID, args.Title, args.Content, args.Category, args.Tags, args.IsPinned, args.IsArchived).Scan(&noteID)

	if err != nil {
		return nil, fmt.Errorf("error creating note: %v", err)
	}

	// Return the newly created note
	return GetNote(conn, userID, json.RawMessage(fmt.Sprintf(`{"noteId":%d}`, noteID)))
}

// UpdateNoteArgs represents arguments for updating a note
type UpdateNoteArgs struct {
	NoteID     int      `json:"noteId"`
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	Category   string   `json:"category"`
	Tags       []string `json:"tags"`
	IsPinned   bool     `json:"isPinned"`
	IsArchived bool     `json:"isArchived"`
}

// UpdateNote updates an existing note
func UpdateNote(conn *utils.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args UpdateNoteArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing args: %v", err)
	}

	// Validate required fields
	if args.NoteID <= 0 {
		return nil, fmt.Errorf("noteId is required")
	}
	if args.Title == "" {
		return nil, fmt.Errorf("title is required")
	}

	// First check if the note exists and belongs to the user
	var exists bool
	err := conn.DB.QueryRow(context.Background(), `
		SELECT EXISTS(SELECT 1 FROM notes WHERE noteId = $1 AND userId = $2)
	`, args.NoteID, userID).Scan(&exists)

	if err != nil {
		return nil, fmt.Errorf("error checking note existence: %v", err)
	}

	if !exists {
		return nil, fmt.Errorf("note not found or you don't have permission to update it")
	}

	// Update the note
	_, err = conn.DB.Exec(context.Background(), `
		UPDATE notes
		SET title = $1, content = $2, category = $3, tags = $4, updated_at = NOW(), is_pinned = $5, is_archived = $6
		WHERE noteId = $7 AND userId = $8
	`, args.Title, args.Content, args.Category, args.Tags, args.IsPinned, args.IsArchived, args.NoteID, userID)

	if err != nil {
		return nil, fmt.Errorf("error updating note: %v", err)
	}

	// Return the updated note
	return GetNote(conn, userID, json.RawMessage(fmt.Sprintf(`{"noteId":%d}`, args.NoteID)))
}

// DeleteNoteArgs represents arguments for deleting a note
type DeleteNoteArgs struct {
	NoteID int `json:"noteId"`
}

// DeleteNote deletes a note
func DeleteNote(conn *utils.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args DeleteNoteArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing args: %v", err)
	}

	// Validate required fields
	if args.NoteID <= 0 {
		return nil, fmt.Errorf("noteId is required")
	}

	// Delete the note
	result, err := conn.DB.Exec(context.Background(), `
		DELETE FROM notes
		WHERE noteId = $1 AND userId = $2
	`, args.NoteID, userID)

	if err != nil {
		return nil, fmt.Errorf("error deleting note: %v", err)
	}

	// Check if any rows were affected
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, fmt.Errorf("note not found or you don't have permission to delete it")
	}

	return map[string]interface{}{
		"success": true,
		"message": "Note deleted successfully",
	}, nil
}

// ToggleNotePinArgs represents arguments for toggling a note's pinned status
type ToggleNotePinArgs struct {
	NoteID   int  `json:"noteId"`
	IsPinned bool `json:"isPinned"`
}

// ToggleNotePin toggles the pinned status of a note
func ToggleNotePin(conn *utils.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args ToggleNotePinArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing args: %v", err)
	}

	// Update the note's pinned status
	result, err := conn.DB.Exec(context.Background(), `
		UPDATE notes
		SET is_pinned = $1, updated_at = NOW()
		WHERE noteId = $2 AND userId = $3
	`, args.IsPinned, args.NoteID, userID)

	if err != nil {
		return nil, fmt.Errorf("error updating note pin status: %v", err)
	}

	// Check if any rows were affected
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, fmt.Errorf("note not found or you don't have permission to update it")
	}

	// Return the updated note
	return GetNote(conn, userID, json.RawMessage(fmt.Sprintf(`{"noteId":%d}`, args.NoteID)))
}

// ToggleNoteArchiveArgs represents arguments for toggling a note's archived status
type ToggleNoteArchiveArgs struct {
	NoteID     int  `json:"noteId"`
	IsArchived bool `json:"isArchived"`
}

// ToggleNoteArchive toggles the archived status of a note
func ToggleNoteArchive(conn *utils.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args ToggleNoteArchiveArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing args: %v", err)
	}

	// Update the note's archived status
	result, err := conn.DB.Exec(context.Background(), `
		UPDATE notes
		SET is_archived = $1, updated_at = NOW()
		WHERE noteId = $2 AND userId = $3
	`, args.IsArchived, args.NoteID, userID)

	if err != nil {
		return nil, fmt.Errorf("error updating note archive status: %v", err)
	}

	// Check if any rows were affected
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, fmt.Errorf("note not found or you don't have permission to update it")
	}

	// Return the updated note
	return GetNote(conn, userID, json.RawMessage(fmt.Sprintf(`{"noteId":%d}`, args.NoteID)))
}
