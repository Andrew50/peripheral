
CREATE TABLE users (
    userId SERIAL PRIMARY KEY,
    username VARCHAR(100) UNIQUE NOT NULL,
    password VARCHAR(100) NOT NULL
);
CREATE INDEX idxUsers ON users (username, password);
CREATE TABLE securities (
    securityId INT,
    ticker varchar(10) not null,
    figi varchar(12) not null,
    --cik varchar(10) not null,
    minDate timestamp,
    maxDate timestamp,
    unique (ticker, minDate),
    unique (ticker, maxDate),
    unique (securityId, minDate),
    unique (securityId, maxdate)
);
create index idxTickerDateRange on securities (ticker, minDate, maxDate);
create table setups (
    setupId serial primary key,
    userId int references users(userId) on delete cascade, 
    name varchar(50) not null,
    timeframe varchar(10) not null
);
CREATE TABLE instances (
    instanceId serial PRIMARY KEY,
    userId serial references users(userId) on delete cascade,
    cik varchar(10) not null,
    timestamp timestamp not null,
    unique (userId, cik, timestamp)
);
create index idxInstances on instances (cik, timestamp);
create table samples (
    sampleId SERIAL PRIMARY KEY,
    instanceId integer references instances(instanceId) on delete cascade,
    setupId serial references setups(setupId) on delete cascade,
    label boolean default null,
    unique (instanceId, setupId)
);
CREATE TABLE annotations (
    annotationId serial primary key,
    instanceId serial references instances(instanceId) on delete cascade,
    entry text not null default '', 
    completed boolean not null default false
);
CREATE TABLE journals (
    journalId serial primary key,
    timestamp timestamp not null,
    userId serial references users(userId),
    entry text,
    unique (timestamp, userId)
);
