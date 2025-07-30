-- Migration: 086_short_conversation_ids
-- Description: Changes conversation_id from UUID to VARCHAR to allow shorter IDs for new conversations

BEGIN;

-- Change conversation_id column type from UUID to VARCHAR(36) to accommodate both UUIDs and shorter IDs
-- This preserves all existing data while allowing shorter IDs going forward

-- Drop the foreign key constraint first
ALTER TABLE conversation_messages DROP CONSTRAINT conversation_messages_conversation_id_fkey;

-- Change both column types
ALTER TABLE conversations ALTER COLUMN conversation_id TYPE VARCHAR(36);
ALTER TABLE conversation_messages ALTER COLUMN conversation_id TYPE VARCHAR(36);

-- Recreate the foreign key constraint
ALTER TABLE conversation_messages 
ADD CONSTRAINT conversation_messages_conversation_id_fkey 
FOREIGN KEY (conversation_id) REFERENCES conversations(conversation_id) ON DELETE CASCADE;

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (86, 'Change conversation_id from UUID to VARCHAR to allow shorter IDs')
ON CONFLICT (version) DO NOTHING;

COMMIT; 