BEGIN;
-- This command identifies duplicate rows based on the combination of `ticker` and `active`.
-- For each group of duplicates, it preserves the single row that has the earliest `minDate`
-- and deletes all other rows in that group.

WITH duplicates_to_delete AS (
    SELECT
        ctid
    FROM (
        SELECT
            ctid,
            -- Assign a unique number to each row within a group of the same (ticker, active).
            -- The rows are ordered by `minDate`, so the record with the earliest `minDate` gets number 1.
            -- `securityid` is used as a tie-breaker if `minDate`s are identical.
            ROW_NUMBER() OVER(PARTITION BY ticker, active ORDER BY minDate ASC, securityid ASC) as rn
        FROM
            securities
    ) ranked_rows
    -- We select all rows that are not the first one in their group for deletion.
    WHERE ranked_rows.rn > 1
)
DELETE FROM securities
WHERE ctid IN (SELECT ctid FROM duplicates_to_delete);

-- Add unique constraint on ticker and active because maxdate null doesnt trigger unique constraint
ALTER TABLE securities
ADD CONSTRAINT unique_ticker_active UNIQUE (ticker, active);
-- Insert schema version
INSERT INTO schema_versions (version, description)
VALUES (
    77,
    'Add unique constraint on ticker and active'
) ON CONFLICT (version) DO NOTHING;




COMMIT;