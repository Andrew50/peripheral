# Notes Feature

The Notes feature allows users to create, edit, and organize personal notes within the application.

## Features

- Create and edit notes with rich text content
- Categorize notes for better organization
- Add tags to notes for easy filtering
- Pin important notes to the top
- Archive notes you no longer need but want to keep
- Filter notes by category, tags, and archived status
- **Full-text search** across note titles and content with highlighted results

## Components

- **Notes List Page** (`+page.svelte`): Displays all notes with filtering options and search functionality
- **Note Detail Page** (`[id]/+page.svelte`): Allows editing of a specific note

## API Endpoints

The notes feature uses the following API endpoints:

- `get_notes`: Retrieves all notes with optional filtering
- `get_note`: Retrieves a single note by ID
- `create_note`: Creates a new note
- `update_note`: Updates an existing note
- `delete_note`: Deletes a note
- `toggle_note_pin`: Toggles the pinned status of a note
- `toggle_note_archive`: Toggles the archived status of a note
- `search_notes`: Performs full-text search across notes with result highlighting

## Database Schema

Notes are stored in the `notes` table with the following structure:

```sql
CREATE TABLE notes (
    noteId SERIAL PRIMARY KEY,
    userId INT REFERENCES users(userId) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    content TEXT,
    category VARCHAR(100),
    tags TEXT[] DEFAULT ARRAY[]::TEXT[],
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    is_pinned BOOLEAN DEFAULT FALSE,
    is_archived BOOLEAN DEFAULT FALSE,
    search_vector TSVECTOR
);

-- Index for full-text search
CREATE INDEX notes_search_idx ON notes USING GIN(search_vector);

-- Trigger to automatically update search_vector on insert or update
CREATE TRIGGER notes_search_vector_update
BEFORE INSERT OR UPDATE ON notes
FOR EACH ROW EXECUTE FUNCTION notes_search_vector_update();
```

## Search Implementation

The notes feature implements PostgreSQL full-text search capabilities:

- Uses `tsvector` and `tsquery` for efficient text searching
- Automatically updates search vectors when notes are created or modified
- Provides relevance ranking of search results
- Returns highlighted snippets showing search term matches
- Supports searching across title, content, category, and tags

## Future Enhancements

- Rich text editor with formatting options
- Note templates
- Collaborative notes
- Note sharing
- Attachments and images
- Markdown support
- Semantic search using vector embeddings
- Note linking and knowledge graph capabilities
- Automatic tagging using NLP
- Version history for notes 