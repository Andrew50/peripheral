-- Migration: 025_add_prompt_strategy_columns
-- Description: Add columns needed for prompt-based strategy system
BEGIN;
-- Add the missing columns to strategies table
ALTER TABLE strategies
ADD COLUMN IF NOT EXISTS description TEXT,
    ADD COLUMN IF NOT EXISTS prompt TEXT,
    ADD COLUMN IF NOT EXISTS pythonCode TEXT,
    ADD COLUMN IF NOT EXISTS score INTEGER DEFAULT 0,
    ADD COLUMN IF NOT EXISTS version VARCHAR(20) DEFAULT '1.0',
    ADD COLUMN IF NOT EXISTS createdAt TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS isAlertActive BOOLEAN DEFAULT FALSE;
-- Create indexes for better performance
CREATE INDEX IF NOT EXISTS idx_strategies_created_at ON strategies(createdAt DESC);
CREATE INDEX IF NOT EXISTS idx_strategies_alert_active ON strategies(isAlertActive)
WHERE isAlertActive = TRUE;
CREATE INDEX IF NOT EXISTS idx_strategies_user_created ON strategies(userId, createdAt DESC);
-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
        25,
        'Add columns for prompt-based strategy system'
    ) ON CONFLICT (version) DO NOTHING;
COMMIT;