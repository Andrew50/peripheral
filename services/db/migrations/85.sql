-- Migration 085: Add plot storage columns to conversations table
-- Description: Add has_plot boolean flag and plot text field for storing base64 encoded plot data

BEGIN;

-- Add has_plot boolean column with default FALSE
ALTER TABLE conversations
ADD COLUMN IF NOT EXISTS has_plot BOOLEAN NOT NULL DEFAULT FALSE;

-- Add plot text column for storing base64 encoded plot data
ALTER TABLE conversations
ADD COLUMN IF NOT EXISTS plot TEXT;

-- Create index for efficient querying of conversations with plots
CREATE INDEX IF NOT EXISTS idx_conversations_has_plot ON conversations(has_plot) WHERE has_plot = TRUE;

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    85,
    'Add has_plot boolean and plot text columns to conversations table for plot storage'
) ON CONFLICT (version) DO NOTHING;

COMMIT; 
