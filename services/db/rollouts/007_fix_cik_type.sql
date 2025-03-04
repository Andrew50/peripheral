-- Convert CIK from int to BIGINT to handle large values
-- Description: Fix CIK column type to handle large CIK values from SEC
-- First, check if the column exists and is of type integer
DO $$ BEGIN IF EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_name = 'securities'
        AND column_name = 'cik'
        AND data_type = 'integer'
) THEN -- Alter the column type to BIGINT
ALTER TABLE securities
ALTER COLUMN cik TYPE BIGINT USING cik::BIGINT;
END IF;
END $$;
-- Update schema_versions table
INSERT INTO schema_versions (version, description)
VALUES (
        '007',
        'Fix CIK column type to handle large values'
    ) ON CONFLICT (version) DO NOTHING;