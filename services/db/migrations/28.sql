-- Migration 028: Add archive_reason field to conversation_messages table
-- This field will track why a message was archived (edited, retried, etc.)

ALTER TABLE conversation_messages 
ADD COLUMN archive_reason VARCHAR(50);

-- Add index for efficient querying of archived messages by reason
CREATE INDEX IF NOT EXISTS idx_conversation_messages_archive_reason 
ON conversation_messages(archive_reason) 
WHERE archive_reason IS NOT NULL;

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (28, 'Added archive_reason field to conversation_messages table')
ON CONFLICT (version) DO NOTHING; 