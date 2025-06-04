-- Migration: 019_add_conversations_tables
-- Description: Creates tables for storing multiple conversations per user (without message expiration)

BEGIN;

-- Main conversations table
CREATE TABLE IF NOT EXISTS conversations (
    conversation_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id INTEGER NOT NULL REFERENCES users(userid) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    metadata JSONB DEFAULT '{}',
    
    -- Track total conversation stats
    total_token_count INTEGER DEFAULT 0,
    message_count INTEGER DEFAULT 0
);

-- Conversation messages table
CREATE TABLE IF NOT EXISTS conversation_messages (
    message_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(conversation_id) ON DELETE CASCADE,
    
    -- Message content and metadata (matches current ChatMessage structure)
    query TEXT NOT NULL,
    response_text TEXT DEFAULT '',
    content_chunks JSONB DEFAULT '[]',
    function_calls JSONB DEFAULT '[]',
    tool_results JSONB DEFAULT '[]',
    context_items JSONB DEFAULT '[]',
    suggested_queries JSONB DEFAULT '[]',
    citations JSONB DEFAULT '[]',
    
    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP WITH TIME ZONE,
    
    -- Status and metadata
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'completed', 'error')),
    token_count INTEGER DEFAULT 0,
    
    -- Ordering within conversation
    message_order INTEGER NOT NULL,
    
    UNIQUE(conversation_id, message_order)
);

-- Indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_conversations_user_id ON conversations(user_id);
CREATE INDEX IF NOT EXISTS idx_conversations_created_at ON conversations(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_conversations_updated_at ON conversations(updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_conversation_messages_conversation_id ON conversation_messages(conversation_id);
CREATE INDEX IF NOT EXISTS idx_conversation_messages_created_at ON conversation_messages(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_conversation_messages_status ON conversation_messages(status);
CREATE INDEX IF NOT EXISTS idx_conversation_messages_order ON conversation_messages(conversation_id, message_order);

-- GIN indexes for JSONB columns that might be searched
CREATE INDEX IF NOT EXISTS idx_conversation_messages_context_items ON conversation_messages USING GIN(context_items);
CREATE INDEX IF NOT EXISTS idx_conversation_messages_suggested_queries ON conversation_messages USING GIN(suggested_queries);

-- Function to update conversation updated_at timestamp when messages are modified
CREATE OR REPLACE FUNCTION update_conversation_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    -- Update the parent conversation's updated_at timestamp
    UPDATE conversations 
    SET updated_at = CURRENT_TIMESTAMP 
    WHERE conversation_id = COALESCE(NEW.conversation_id, OLD.conversation_id);
    
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

-- Trigger to automatically update conversation timestamps
CREATE TRIGGER trigger_update_conversation_updated_at
    AFTER INSERT OR UPDATE OR DELETE ON conversation_messages
    FOR EACH ROW
    EXECUTE FUNCTION update_conversation_updated_at();

-- Function to auto-increment message_order
CREATE OR REPLACE FUNCTION set_message_order()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.message_order IS NULL THEN
        SELECT COALESCE(MAX(message_order), 0) + 1
        INTO NEW.message_order
        FROM conversation_messages
        WHERE conversation_id = NEW.conversation_id;
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to automatically set message order
CREATE TRIGGER trigger_set_message_order
    BEFORE INSERT ON conversation_messages
    FOR EACH ROW
    EXECUTE FUNCTION set_message_order();

-- Function to update conversation stats (token count, message count)
CREATE OR REPLACE FUNCTION update_conversation_stats()
RETURNS TRIGGER AS $$
BEGIN
    -- Update conversation statistics (no longer filtering by expiration)
    UPDATE conversations 
    SET 
        total_token_count = (
            SELECT COALESCE(SUM(token_count), 0) 
            FROM conversation_messages 
            WHERE conversation_id = COALESCE(NEW.conversation_id, OLD.conversation_id)
        ),
        message_count = (
            SELECT COUNT(*) 
            FROM conversation_messages 
            WHERE conversation_id = COALESCE(NEW.conversation_id, OLD.conversation_id)
        )
    WHERE conversation_id = COALESCE(NEW.conversation_id, OLD.conversation_id);
    
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

-- Trigger to automatically update conversation stats
CREATE TRIGGER trigger_update_conversation_stats
    AFTER INSERT OR UPDATE OR DELETE ON conversation_messages
    FOR EACH ROW
    EXECUTE FUNCTION update_conversation_stats();

COMMIT; 