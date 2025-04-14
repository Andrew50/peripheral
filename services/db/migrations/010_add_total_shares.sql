-- Description: Add missing total_shares column to securities table

DO $$
BEGIN
    -- Check if total_shares column exists in the securities table
    IF NOT EXISTS (
        SELECT 1 
        FROM information_schema.columns 
        WHERE table_name = 'securities' 
        AND column_name = 'total_shares'
    ) THEN
        -- Add total_shares column if it doesn't exist
        ALTER TABLE securities ADD COLUMN total_shares BIGINT;
        RAISE NOTICE 'Added total_shares column to securities table';
    ELSE
        RAISE NOTICE 'total_shares column already exists in securities table';
    END IF;
END
$$; 