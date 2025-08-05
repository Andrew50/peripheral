-- Migration: 035_add_selected_polygon_fields
-- Description: Add selected Polygon API fields to securities table
BEGIN;
-- Add selected missing fields from Polygon ticker details
ALTER TABLE securities
ADD COLUMN IF NOT EXISTS share_class_figi VARCHAR(20),
    ADD COLUMN IF NOT EXISTS sic_code VARCHAR(10),
    ADD COLUMN IF NOT EXISTS sic_description TEXT,
    ADD COLUMN IF NOT EXISTS total_employees BIGINT,
    ADD COLUMN IF NOT EXISTS weighted_shares_outstanding BIGINT;
-- Create indexes for commonly searched fields
CREATE INDEX IF NOT EXISTS idx_securities_share_class_figi ON securities (share_class_figi);
CREATE INDEX IF NOT EXISTS idx_securities_sic_code ON securities (sic_code);
-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
        35,
        'Add selected Polygon API fields: share_class_figi, sic_code, sic_description, total_employees, weighted_shares_outstanding'
    ) ON CONFLICT (version) DO NOTHING;
COMMIT;