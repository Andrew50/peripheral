BEGIN;
CREATE TABLE IF NOT EXISTS sectors (
    sectorId serial primary key,
    sector varchar(50) not null
);
CREATE INDEX idxSector ON sectors(sector);

CREATE TABLE IF NOT EXISTS industries (
    industryId serial primary key,
    industry varchar(50) not null
);
CREATE INDEX idxIndustry ON industries(industry);
COMMIT;
