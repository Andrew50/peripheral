-- Add CIK column to securities table
DO $$ BEGIN -- Check if the column doesn't exist
IF NOT EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_name = 'securities'
        AND column_name = 'cik'
) THEN -- Add the CIK column
ALTER TABLE securities
ADD COLUMN cik VARCHAR(10);
-- Create an index on the CIK column for better query performance
CREATE INDEX idx_securities_cik ON securities(cik);
END IF;
END $$;