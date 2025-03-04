-- Script to check and convert multiple columns to BIGINT
-- This script checks if columns are of type BIGINT and converts them if needed
-- Function to check and convert a column to BIGINT
CREATE OR REPLACE FUNCTION convert_to_bigint(p_table_name text, p_column_name text) RETURNS void AS $$
DECLARE v_column_type text;
BEGIN -- Check if the column exists and get its current type
SELECT data_type INTO v_column_type
FROM information_schema.columns
WHERE table_name = p_table_name
    AND column_name = p_column_name;
-- If column doesn't exist, raise notice and return
IF v_column_type IS NULL THEN RAISE NOTICE 'Column %.% does not exist',
p_table_name,
p_column_name;
RETURN;
END IF;
-- Check if the column is already BIGINT
IF v_column_type = 'bigint' THEN RAISE NOTICE 'Column %.% is already BIGINT. No changes needed.',
p_table_name,
p_column_name;
RETURN;
END IF;
-- Convert the column to BIGINT
EXECUTE format(
    'ALTER TABLE %I ALTER COLUMN %I TYPE BIGINT USING %I::BIGINT',
    p_table_name,
    p_column_name,
    p_column_name
);
RAISE NOTICE 'Column %.% converted from % to BIGINT',
p_table_name,
p_column_name,
v_column_type;
-- Log the change to schema_versions if the table exists
IF EXISTS (
    SELECT 1
    FROM information_schema.tables
    WHERE table_name = 'schema_versions'
) THEN EXECUTE format(
    'INSERT INTO schema_versions (version, description) VALUES (''custom_%s'', ''Converted %s.%s from %s to BIGINT'') ON CONFLICT DO NOTHING',
    now()::text,
    p_table_name,
    p_column_name,
    v_column_type
);
END IF;
END;
$$ LANGUAGE plpgsql;
-- Convert securities.cik to BIGINT
SELECT convert_to_bigint('securities', 'cik');
-- Convert securities.market_cap to BIGINT
SELECT convert_to_bigint('securities', 'market_cap');
-- Convert securities.share_class_shares_outstanding to BIGINT
SELECT convert_to_bigint('securities', 'share_class_shares_outstanding');
-- Convert securities.total_shares to BIGINT
SELECT convert_to_bigint('securities', 'total_shares');
-- Drop the function when done
DROP FUNCTION convert_to_bigint(text, text);