-- Add a new table for storing user notes
BEGIN;

-- Create the notes table
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
    is_archived BOOLEAN DEFAULT FALSE
);

-- Create indexes for efficient querying
CREATE INDEX idx_notes_userId ON notes(userId);
CREATE INDEX idx_notes_category ON notes(category);
CREATE INDEX idx_notes_created_at ON notes(created_at);
CREATE INDEX idx_notes_is_pinned ON notes(is_pinned);
CREATE INDEX idx_notes_is_archived ON notes(is_archived);

-- Add GIN index for tags array for efficient tag searching
CREATE INDEX idx_notes_tags ON notes USING GIN(tags);

-- Update schema version
INSERT INTO schema_versions (version, description, applied_at)
VALUES (8, 'Add notes table', NOW());

COMMIT; 