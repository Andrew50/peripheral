-- Migration: 094_move_runtime_table_settings_to_migration
-- Purpose: Move ALTER TABLE autovacuum settings for static_refs tables out of runtime code
--          to avoid AccessExclusive locks during refreshes and reduce lock timeout failures.

BEGIN;

-- Apply aggressive autovacuum/analyze settings for high-write static refs tables once

-- static_refs_1m (frequent updates during market hours)
ALTER TABLE IF EXISTS static_refs_1m
    SET (autovacuum_vacuum_threshold = 100,
         autovacuum_vacuum_scale_factor = 0.02,
         autovacuum_analyze_threshold = 50,
         autovacuum_analyze_scale_factor = 0.01,
         autovacuum_vacuum_cost_delay = 2,
         autovacuum_vacuum_cost_limit = 2000);

-- static_refs_daily (updated every few minutes during regular hours)
ALTER TABLE IF EXISTS static_refs_daily
    SET (autovacuum_vacuum_threshold = 200,
         autovacuum_vacuum_scale_factor = 0.03,
         autovacuum_analyze_threshold = 100,
         autovacuum_analyze_scale_factor = 0.02,
         autovacuum_vacuum_cost_delay = 2,
         autovacuum_vacuum_cost_limit = 2000);

-- Record schema version
INSERT INTO schema_versions (version, description)
VALUES (94, 'Move static_refs autovacuum settings to migration to avoid runtime DDL')
ON CONFLICT (version) DO NOTHING;

COMMIT;


