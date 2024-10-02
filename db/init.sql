CREATE TABLE users (
    userId SERIAL PRIMARY KEY,
    username VARCHAR(100) UNIQUE NOT NULL,
    password VARCHAR(100) NOT NULL,
    settings JSON
);
CREATE INDEX idxUsers ON users (username, password);
CREATE TABLE securities (
    securityid SERIAL,
    ticker varchar(10) not null,
    figi varchar(12) not null,
    minDate timestamp,
    maxDate timestamp,
    unique (ticker, minDate),
    unique (ticker, maxDate),
    unique (securityid, minDate),
    unique (securityid, maxDate)
);
create index idxTickerDateRange on securities (ticker, minDate, maxDate);
create table setups (
    setupId serial primary key,
    userId int references users(userId) on delete cascade, 
    name varchar(50) not null,
    timeframe varchar(10) not null,
    bars int not null,
    threshold int not null,
    modelVersion int not null default 0,
    score int default 0;
    sampleSize int default 0,
    untrainedSamples int default 0,
    dolvol float not null,
    adr float not null,
    mcap float not null,
    unique (userId, name)
);
create index idxUserIdName on setups(userId, name);
create table samples (
    sampleId SERIAL PRIMARY KEY,
    setupId serial references setups(setupId) on delete cascade,
    securityId int,-- references securities(securityId), -- not unique
    timestamp timestamp not null,
    label boolean,
    unique (securityId, timestamp, setupId)
);
create index idxSetupId on samples(setupId);
CREATE TABLE studies (
    studyId serial primary key,
    userId serial references users(userId) on delete cascade,
    securityId int, --references securities(securityId), --cant because not unique
    timestamp timestamp not null,
    completed boolean not null default false,
    entry json,
    unique(userId, securityId, timestamp)
);
create index idxUserIdCompleted on studies(userId, completed);
CREATE TABLE journals (
    journalId serial primary key,
    userId serial references users(userId),
    timestamp timestamp not null,
    completed boolean not null default false,
    entry json,
    unique (timestamp, userId)
);
CREATE INDEX idxJournalIdUserId on journals(journalId,userId);
CREATE INDEX idxTimestamp on journals(timestamp);
CREATE TABLE watchlists (
    watchlistId serial primary key,
    userId serial references users(userId) on delete cascade,
    watchlistName varchar(50) not null,
    unique(watchlistName,userId)
);
CREATE INDEX idxWatchlistIdUserId on watchlists(watchlistId,userId);
CREATE TABLE watchlistItems (
    watchlistItemId serial primary key,
    watchlistId serial references watchlists(watchlistId) on delete cascade,
    securityId int, --serial references securities(securityId) on delete cascade,
    unique (watchlistId, securityId)
);
CREATE INDEX idxWatchlistId on watchlistItems(watchlistId);
COPY securities(securityid, ticker, figi, minDate, maxDate) 
FROM '/docker-entrypoint-initdb.d/securities.csv' DELIMITER ',' CSV HEADER;
INSERT INTO users (userId, username, password) VALUES (0, 'user', 'pass');

INSERT INTO setups (setupId,userId,name,timeframe,bars,threshold,dolvol,adr,mcap) VALUES 
(1,0, 'EP', '1d', 30, 30, 5000000, 2.5, 0),
(2,0, 'F', '1d', 60, 30, 5000000, 2.5, 0),
(3,0, 'MR', '1d', 30, 30, 5000000, 2.5, 0),
(4,0, 'NEP', '1d', 30, 30, 5000000, 2.5, 0),
(5,0, 'NF', '1d', 60, 30, 5000000, 2.5, 0),
(6,0, 'NP', '1d', 30, 30, 5000000, 2.5, 0),
(7,0, 'P', '1d', 30, 30, 5000000, 2.5, 0);
CREATE TEMP TABLE temp (
    setupId INTEGER NOT NULL,
    ticker VARCHAR(10) NOT NULL,
    timestamp INTEGER NOT NULL,
    label BOOLEAN
);

COPY temp(setupId,ticker,timestamp,label) 
FROM '/docker-entrypoint-initdb.d/samples.csv' 
WITH (FORMAT csv, HEADER true, DELIMITER ',');

INSERT INTO samples (setupId, securityId, timestamp, label)
SELECT
    ts.setupId,
    sec.securityId,
    TO_TIMESTAMP(ts.timestamp), 
    ts.label
FROM
    temp ts
JOIN
    securities sec
ON ts.ticker = sec.ticker
WHERE (sec.minDate <= TO_TIMESTAMP(ts.timestamp) OR sec.minDate IS NULL)
  AND (sec.maxDate > TO_TIMESTAMP(ts.timestamp) OR sec.maxDate IS NULL);

