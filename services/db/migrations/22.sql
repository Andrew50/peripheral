-- Migration: 022_add_name_index
-- Description: Add trigram index on securities name column for better search performance

BEGIN;

-- Create trigram index on name column for fuzzy search optimization
CREATE INDEX IF NOT EXISTS trgm_idx_securities_name ON securities USING gin (name gin_trgm_ops);

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (22, 'Add trigram index on securities name column for search optimization');

COMMIT; 