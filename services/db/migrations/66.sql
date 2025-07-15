-- Migration: 066_optimize_securities_fuzzy_search
-- Description: Add optimized trigram indexes for securities fuzzy search performance

-- Ensure pg_trgm extension is available
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- 1. Add trigram index on ticker_norm for % operator and similarity() function
-- This replaces the basic B-tree index with a GIN trigram index
-- Using partial index to only cover active securities (maxDate IS NULL)
CREATE INDEX IF NOT EXISTS gin_securities_ticker_norm_trgm
    ON securities
    USING gin (ticker_norm gin_trgm_ops)
    WHERE maxDate IS NULL;

-- 2. Add expression index for UPPER(name) trigram searches
-- This allows the query's UPPER(name) predicates to use a trigram index
CREATE INDEX IF NOT EXISTS gin_securities_upper_name_trgm
    ON securities
    USING gin ((upper(name)) gin_trgm_ops)
    WHERE maxDate IS NULL;

-- 3. Add pattern ops index for prefix LIKE searches on ticker_norm
-- This optimizes the ticker_norm LIKE $1 || '%' predicate
CREATE INDEX IF NOT EXISTS btree_securities_ticker_norm_pattern
    ON securities (ticker_norm text_pattern_ops)
    WHERE maxDate IS NULL;

-- 4. Add pattern ops index for prefix LIKE searches on UPPER(name)
-- This optimizes the UPPER(name) LIKE $1 || '%' predicate
CREATE INDEX IF NOT EXISTS btree_securities_upper_name_pattern
    ON securities ((upper(name)) text_pattern_ops)
    WHERE maxDate IS NULL;

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    66,
    'Add optimized trigram and pattern indexes for securities fuzzy search performance'
) ON CONFLICT (version) DO NOTHING; 