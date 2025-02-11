--init.sql
CREATE EXTENSION IF NOT EXISTS pg_trgm;

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
    sector varchar(100),
    industry varchar(100),
    unique (ticker, minDate),
    unique (ticker, maxDate),
    unique (securityid, minDate),
    unique (securityid, maxDate)
);
CREATE INDEX trgm_idx_securities_ticker ON securities USING gin (ticker gin_trgm_ops);
create index idxTickerDateRange on securities (ticker, minDate, maxDate);
create table setups (
    setupId serial primary key,
    userId int references users(userId) on delete cascade,
    name varchar(50) not null,
    timeframe varchar(10) not null,
    bars int not null,
    threshold int not null,
    modelVersion int not null default 0,
    score int default 0,
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
    securityId int,
    -- references securities(securityId), -- not unique
    timestamp timestamp not null,
    label boolean,
    unique (securityId, timestamp, setupId)
);
create index idxSetupId on samples(setupId);
CREATE TABLE studies (
    studyId serial primary key,
    userId serial references users(userId) on delete cascade,
    securityId int,
    --references securities(securityId), --cant because not unique
    setupId serial references setups(setupId),
    --no action
    timestamp timestamp not null,
    completed boolean not null default false,
    entry json,
    unique(userId, securityId, timestamp, setupId)
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
CREATE INDEX idxJournalIdUserId on journals(journalId, userId);
CREATE INDEX idxTimestamp on journals(timestamp);
CREATE TABLE watchlists (
    watchlistId serial primary key,
    userId serial references users(userId) on delete cascade,
    watchlistName varchar(50) not null,
    unique(watchlistName, userId)
);
CREATE INDEX idxWatchlistIdUserId on watchlists(watchlistId, userId);
CREATE TABLE watchlistItems (
    watchlistItemId serial primary key,
    watchlistId serial references watchlists(watchlistId) on delete cascade,
    securityId int,
    --serial references securities(securityId) on delete cascade,
    unique (watchlistId, securityId)
);
CREATE INDEX idxWatchlistId on watchlistItems(watchlistId);
/*CREATE TABLE algos (
 algoId serial primary key,
 algoName VARCHAR(50) not null
 );*/
CREATE TABLE alerts (
    alertId SERIAL PRIMARY KEY,
    userId SERIAL REFERENCES users(userId) ON DELETE CASCADE,
    active BOOLEAN NOT NULL DEFAULT false,
    alertType VARCHAR(10) NOT NULL CHECK (alertType IN ('price', 'setup', 'algo')),
    -- Restrict the allowed alert types
    setupId INT REFERENCES setups(setupId) ON DELETE CASCADE,
    --algoId INT REFERENCES algos(algoId) ON DELETE CASCADE,
    algoId INT,
    price DECIMAL(10, 4),
    direction Boolean,
    securityID INT,
    CONSTRAINT chk_alert_price_or_setup CHECK (
        (
            alertType = 'price'
            AND price IS NOT NULL
            AND securityID IS NOT NULL
            AND direction IS NOT NULL
            AND algoId IS NULL
            AND setupId IS NULL
        )
        OR (
            alertType = 'setup'
            AND setupId IS NOT NULL
            AND algoId IS NULL
            AND price IS NULL
            AND securityID IS NULL
        )
        OR (
            alertType = 'algo'
            AND algoId IS NOT NULL
            AND setupId IS NULL
            AND price IS NULL
        )
    )
);
CREATE INDEX idxAlertByUserId on alerts(userId);
CREATE TABLE alertLogs (
    alertLogId serial primary key,
    alertId serial references alerts(alertId) on delete cascade,
    timestamp timestamp not null,
    securityId INT,
    --references sercurities
    unique(alertId, timestamp, securityId)
);
CREATE INDEX idxAlertLogId on alertLogs(alertLogId);
CREATE TABLE horizontal_lines (
    id serial primary key,
    userId serial references users(userId) on delete cascade,
    securityId int,
    --references securities(securityId),
    price float not null,
    unique (userId, securityId, price)
);
CREATE TABLE trades (
    tradeId SERIAL PRIMARY KEY,
    userId INT REFERENCES users(userId) ON DELETE CASCADE,
    securityId INT, 
    ticker VARCHAR(20) NOT NULL,
    tradeDirection VARCHAR(10) NOT NULL,
    date DATE NOT NULL,
    status VARCHAR(10) NOT NULL CHECK (status IN ('Open', 'Closed')),
    openQuantity INT,
    closedPnL DECIMAL(10, 2),
    -- Store up to 20 entries
    entry_times TIMESTAMP [] DEFAULT ARRAY []::TIMESTAMP [],
    entry_prices DECIMAL(10, 4) [] DEFAULT ARRAY []::DECIMAL(10, 4) [],
    entry_shares INT [] DEFAULT ARRAY []::INT [],
    -- Store up to 50 exits
    exit_times TIMESTAMP [] DEFAULT ARRAY []::TIMESTAMP [],
    exit_prices DECIMAL(10, 4) [] DEFAULT ARRAY []::DECIMAL(10, 4) [],
    exit_shares INT [] DEFAULT ARRAY []::INT []
);
CREATE TABLE trade_executions (
    executionId SERIAL PRIMARY KEY,
    userId INT REFERENCES users(userId) ON DELETE CASCADE,
    securityId INT,
    ticker VARCHAR(20) NOT NULL,
    date DATE NOT NULL,
    price DECIMAL(10, 4) NOT NULL,
    size INT NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    direction VARCHAR(10) NOT NULL,
    tradeId INT REFERENCES trades(tradeId)
);
CREATE INDEX idxUserIdSecurityIdPrice on horizontal_lines(userId, securityId, price);
COPY securities(securityid, ticker, figi, minDate, maxDate)
FROM '/docker-entrypoint-initdb.d/securities.csv' DELIMITER ',' CSV HEADER;
INSERT INTO users (userId, username, password)
VALUES (0, 'user', 'pass');
Insert into setups (
        setupid,
        userid,
        name,
        timeframe,
        bars,
        threshold,
        dolvol,
        adr,
        mcap
    )
values (1, 0, 'ep', '1d', 30, 30, 5000000, 2.5, 0),
    (2, 0, 'f', '1d', 60, 30, 5000000, 2.5, 0),
    (3, 0, 'mr', '1d', 30, 30, 5000000, 2.5, 0),
    (4, 0, 'nep', '1d', 30, 30, 5000000, 2.5, 0),
    (5, 0, 'nf', '1d', 60, 30, 5000000, 2.5, 0),
    (6, 0, 'np', '1d', 30, 30, 5000000, 2.5, 0),
    (7, 0, 'p', '1d', 30, 30, 5000000, 2.5, 0);
alter sequence setups_setupid_seq restart with 8;
CREATE TEMP TABLE temp (
    setupId INTEGER NOT NULL,
    ticker VARCHAR(10) NOT NULL,
    timestamp INTEGER NOT NULL,
    label BOOLEAN
);
COPY temp(setupId, ticker, timestamp, label)
FROM '/docker-entrypoint-initdb.d/samples.csv' WITH (FORMAT csv, HEADER true, DELIMITER ',');
INSERT INTO samples (setupId, securityId, timestamp, label)
SELECT ts.setupId,
    sec.securityId,
    TO_TIMESTAMP(ts.timestamp),
    ts.label
FROM temp ts
    JOIN securities sec ON ts.ticker = sec.ticker
WHERE (
        sec.minDate <= TO_TIMESTAMP(ts.timestamp)
        OR sec.minDate IS NULL
    )
    AND (
        sec.maxDate > TO_TIMESTAMP(ts.timestamp)
        OR sec.maxDate IS NULL
    );