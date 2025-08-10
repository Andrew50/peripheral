-- Add persistent ordering to watchlist items
-- 1) Add sortOrder column
ALTER TABLE watchlistItems
ADD COLUMN IF NOT EXISTS sortOrder NUMERIC(20, 10);
-- 2) Backfill existing rows with sequential order per watchlist
WITH ordered AS (
    SELECT watchlistItemId,
        ROW_NUMBER() OVER (
            PARTITION BY watchlistId
            ORDER BY watchlistItemId
        ) AS rn
    FROM watchlistItems
)
UPDATE watchlistItems w
SET sortOrder = ordered.rn * 1000
FROM ordered
WHERE w.watchlistItemId = ordered.watchlistItemId
    AND (
        w.sortOrder IS NULL
        OR w.sortOrder = 0
    );
-- 3) Index to support ordered retrieval and neighbor lookups
CREATE INDEX IF NOT EXISTS idx_watchlistitems_watchlistid_sortorder ON watchlistItems(watchlistId, sortOrder);