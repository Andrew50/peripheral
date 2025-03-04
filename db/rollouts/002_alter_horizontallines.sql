DO $$
BEGIN
    -- Check if the columns already exist
    IF NOT EXISTS (
        SELECT 1 
        FROM information_schema.columns 
        WHERE table_name = 'horizontal_lines' AND column_name = 'color'
    ) THEN
        ALTER TABLE horizontal_lines 
        ADD COLUMN color varchar(20) DEFAULT '#FFFFFF';
    END IF;

    IF NOT EXISTS (
        SELECT 1 
        FROM information_schema.columns 
        WHERE table_name = 'horizontal_lines' AND column_name = 'line_width'
    ) THEN
        ALTER TABLE horizontal_lines 
        ADD COLUMN line_width int DEFAULT 1;
    END IF;
END $$;