-- Migration: 023_add_conversation_sharing
-- Description: Add conversation sharing columns to enable public conversation sharing

BEGIN;

-- Add sharing-related columns to conversations table
ALTER TABLE conversations 
ADD COLUMN IF NOT EXISTS is_public BOOLEAN NOT NULL DEFAULT FALSE,
ADD COLUMN IF NOT EXISTS view_count INTEGER DEFAULT 0,
ADD COLUMN IF NOT EXISTS last_viewed_at TIMESTAMP WITH TIME ZONE;

-- Create index for efficient querying of public conversations
CREATE INDEX IF NOT EXISTS idx_conversations_public ON conversations(is_public) WHERE is_public = TRUE;

-- Create index for view count ordering (for potential "popular conversations" feature)
CREATE INDEX IF NOT EXISTS idx_conversations_view_count ON conversations(view_count DESC) WHERE is_public = TRUE;

-- Create index for recently viewed conversations
CREATE INDEX IF NOT EXISTS idx_conversations_last_viewed ON conversations(last_viewed_at DESC) WHERE is_public = TRUE;

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (23, 'Add conversation sharing columns and indexes')
ON CONFLICT (version) DO NOTHING;

COMMIT; 