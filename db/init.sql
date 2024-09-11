CREATE TABLE users (
    userId SERIAL PRIMARY KEY,
    username VARCHAR(100) UNIQUE NOT NULL,
    password VARCHAR(100) NOT NULL
);
CREATE INDEX idxUsers ON users (username, password);
DROP TABLE IF EXISTS securities;
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
    timeframe varchar(10) not null
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
    timestamp timestamp not null,
    userId serial references users(userId),
    entry json,
    unique (timestamp, userId)
);
