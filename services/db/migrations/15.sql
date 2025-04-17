-- ---------------------------------------------------------------------------
-- 15.sql  (convert FK SERIAL columns → INT)
-- ---------------------------------------------------------------------------
BEGIN;

-- 1. indexes ---------------------------------------------------------------
DROP INDEX IF EXISTS idxAlertLogId;
DROP INDEX IF EXISTS idxStrategiesByStrategyId;
DROP INDEX IF EXISTS idxStrategiesByUserId;
CREATE INDEX idxStrategiesByUserId ON strategies (userid);   -- matches column

-- 2. watchlists.userid  (FK → users.userid) -------------------------------
ALTER TABLE watchlists
    ALTER COLUMN userid TYPE INT  USING userid::INT,
    ALTER COLUMN userid DROP DEFAULT;
DROP SEQUENCE IF EXISTS watchlists_userid_seq;

-- 3. watchlistitems.watchlistid  (FK → watchlists.watchlistid) ------------
ALTER TABLE watchlistitems
    ALTER COLUMN watchlistid TYPE INT USING watchlistid::INT,
    ALTER COLUMN watchlistid DROP DEFAULT;
DROP SEQUENCE IF EXISTS watchlistitems_watchlistid_seq;

-- 4. studies.userid --------------------------------------------------------
ALTER TABLE studies
    ALTER COLUMN userid TYPE INT USING userid::INT,
    ALTER COLUMN userid DROP DEFAULT;
DROP SEQUENCE IF EXISTS studies_userid_seq;

-- 5. alerts.userid ---------------------------------------------------------
ALTER TABLE alerts
    ALTER COLUMN userid TYPE INT USING userid::INT,
    ALTER COLUMN userid DROP DEFAULT;
DROP SEQUENCE IF EXISTS alerts_userid_seq;

-- 6. alerts.securityid -----------------------------------------------------
ALTER TABLE alerts
    ALTER COLUMN securityid TYPE INT USING securityid::INT,
    ALTER COLUMN securityid DROP DEFAULT;
DROP SEQUENCE IF EXISTS alerts_securityid_seq;

-- 7. horizontal_lines.userid ----------------------------------------------
ALTER TABLE horizontal_lines
    ALTER COLUMN userid TYPE INT USING userid::INT,
    ALTER COLUMN userid DROP DEFAULT;
DROP SEQUENCE IF EXISTS horizontal_lines_userid_seq;

COMMIT;

