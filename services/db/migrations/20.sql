-- Migration: 020_add_archived_column_to_messages
-- Description: Add archived column to preserve messages when editing instead of deleting them

BEGIN;

-- Add archived column to conversation_messages table
ALTER TABLE conversation_messages 
ADD COLUMN IF NOT EXISTS archived BOOLEAN DEFAULT FALSE;

-- Create index for efficient querying of non-archived messages
CREATE INDEX IF NOT EXISTS idx_conversation_messages_archived ON conversation_messages(conversation_id, archived) WHERE archived = FALSE;

-- Update the GetConversationMessages query to filter out archived messages by default
-- This will be handled in the application code

-- Update the conversation stats function to exclude archived messages
CREATE OR REPLACE FUNCTION update_conversation_stats()
RETURNS TRIGGER AS $$
BEGIN
    -- Update conversation statistics (exclude archived messages)
    UPDATE conversations 
    SET 
        total_token_count = (
            SELECT COALESCE(SUM(token_count), 0) 
            FROM conversation_messages 
            WHERE conversation_id = COALESCE(NEW.conversation_id, OLD.conversation_id)
            AND archived = FALSE
        ),
        message_count = (
            SELECT COUNT(*) 
            FROM conversation_messages 
            WHERE conversation_id = COALESCE(NEW.conversation_id, OLD.conversation_id)
            AND archived = FALSE
        )
    WHERE conversation_id = COALESCE(NEW.conversation_id, OLD.conversation_id);
    
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

COMMIT; 