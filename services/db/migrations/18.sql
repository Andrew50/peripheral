-- 1. Main table -------------------------------------------------------------
DROP TABLE IF EXISTS sic_codes CASCADE;

CREATE TABLE sic_codes (
    sic_code             CHAR(4)  PRIMARY KEY,   -- 4-digit SIC
    division             CHAR(2)  NOT NULL,      -- A…J
    major_group_code     INT       NOT NULL,     -- 2-digit
    industry_group_code  INT       NOT NULL,     -- 3-digit
    industry_description TEXT      NOT NULL,     -- 4-digit description

    -- Enriched text columns (populated later)
    sector_name          TEXT,
    industry_name        TEXT,                   -- major-group name
    minor_industry_name  TEXT                    -- industry-group name
);

-- 2. Load the raw 4-digit file ---------------------------------------------
-- csv header must match the first five columns above
COPY sic_codes (division,
                major_group_code,
                industry_group_code,
                sic_code,
                industry_description)
FROM '/docker-entrypoint-initdb.d/sic_codes.csv'
WITH (FORMAT csv, HEADER true);

-- 3. Hard-code Division → Sector mapping -----------------------------------
UPDATE sic_codes
SET sector_name = CASE division
    WHEN 'A' THEN 'Agriculture, Forestry & Fishing'
    WHEN 'B' THEN 'Mining'
    WHEN 'C' THEN 'Construction'
    WHEN 'D' THEN 'Manufacturing'
    WHEN 'E' THEN 'Transportation & Public Utilities'
    WHEN 'F' THEN 'Wholesale Trade'
    WHEN 'G' THEN 'Retail Trade'
    WHEN 'H' THEN 'Finance, Insurance & Real Estate'
    WHEN 'I' THEN 'Services'
    WHEN 'J' THEN 'Public Administration'
END;

-- 4. Stage & load Major-group names (2-digit) -------------------------------
DROP TABLE IF EXISTS tmp_major_groups;
CREATE TEMP TABLE tmp_major_groups (
    division             CHAR(2),
    major_group_code     INT PRIMARY KEY,
    industry_name        TEXT
);

COPY tmp_major_groups (division, major_group_code, industry_name)
FROM '/docker-entrypoint-initdb.d/major_groups.csv'
WITH (FORMAT csv, HEADER true);

-- 5. Stage & load Industry-group names (3-digit) ----------------------------
DROP TABLE IF EXISTS tmp_industry_groups;
CREATE TEMP TABLE tmp_industry_groups (
    division             CHAR(2),
    major_group_code     INT,
    industry_group_code  INT PRIMARY KEY,
    minor_industry_name  TEXT
);

COPY tmp_industry_groups (division, major_group_code, industry_group_code, minor_industry_name)
FROM '/docker-entrypoint-initdb.d/industry_groups.csv'
WITH (FORMAT csv, HEADER true);

-- 6. Enrich the main table from the staging tables -------------------------
UPDATE sic_codes sc
SET industry_name = mg.industry_name
FROM tmp_major_groups mg
WHERE sc.major_group_code = mg.major_group_code;

UPDATE sic_codes sc
SET minor_industry_name = ig.minor_industry_name
FROM tmp_industry_groups ig
WHERE sc.industry_group_code = ig.industry_group_code;

-- Optional: tidy up ---------------------------------------------------------
-- VACUUM ANALYZE sic_codes;
-- DROP TABLE tmp_major_groups;
-- DROP TABLE tmp_industry_groups;

