-- Migration: 061_add_invites_table
-- Description: Add invites table for single-use trial invitation codes

BEGIN;

-- Create invites table
CREATE TABLE invites (
    code CHAR(32) PRIMARY KEY,
    plan_name VARCHAR(100) NOT NULL,
    trial_days INTEGER NOT NULL DEFAULT 30,
    used BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Add index on used column for performance
CREATE INDEX idx_invites_used ON invites(used);

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    61,
    'Add invites table for single-use trial invitation codes'
) ON CONFLICT (version) DO NOTHING;

COMMIT;