-- Add full-text search capabilities to the notes table
BEGIN;
-- Add a tsvector column to store the search document
ALTER TABLE notes
ADD COLUMN search_vector tsvector;
-- Create a function to automatically update the search vector
CREATE OR REPLACE FUNCTION notes_search_vector_update() RETURNS trigger AS $$ BEGIN NEW.search_vector := setweight(
    to_tsvector('english', COALESCE(NEW.title, '')),
    'A'
  ) || setweight(
    to_tsvector('english', COALESCE(NEW.content, '')),
    'B'
  ) || setweight(
    to_tsvector('english', COALESCE(NEW.category, '')),
    'C'
  ) || setweight(
    to_tsvector(
      'english',
      array_to_string(COALESCE(NEW.tags, ARRAY []::text []), ' ')
    ),
    'C'
  );
RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- Create a trigger to automatically update the search vector on insert or update
CREATE TRIGGER notes_search_vector_update BEFORE
INSERT
  OR
UPDATE ON notes FOR EACH ROW EXECUTE FUNCTION notes_search_vector_update();
-- Update existing notes to populate the search vector
UPDATE notes
SET search_vector = setweight(to_tsvector('english', COALESCE(title, '')), 'A') || setweight(
    to_tsvector('english', COALESCE(content, '')),
    'B'
  ) || setweight(
    to_tsvector('english', COALESCE(category, '')),
    'C'
  ) || setweight(
    to_tsvector(
      'english',
      array_to_string(COALESCE(tags, ARRAY []::text []), ' ')
    ),
    'C'
  );
-- Create a GIN index for the search vector for efficient searching
CREATE INDEX idx_notes_search_vector ON notes USING GIN(search_vector);
COMMIT;