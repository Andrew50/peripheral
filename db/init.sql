CREATE TABLE users (
    userId SERIAL PRIMARY KEY,
    username VARCHAR(100) UNIQUE NOT NULL,
    password VARCHAR(100) NOT NULL,
    settings JSONB DEFAULT '{}'
);
CREATE INDEX idxUsers ON users (username, password);
CREATE TABLE securities {
    cik varchar(10) not null, 
    ticker varchar(8) not null,
}
create table setups (
    setupId serial primary key,
    name varchar(50) not null
);
CREATE TABLE instances (
    instanceId serial PRIMARY KEY,
    userId serial references users(userId) on delete cascade,
    cik varchar(10) not null,
    timestamp timestamp not null,
    unique (userId, cik, timestamp)
);
create index idxInstances on instances (cik, timestamp);
create table trainingInstances (
    instanceId serial references instances(instanceId) on delete cascade,
    setupId serial references setups(setupId) on delete cascade,
    label boolean,
    unique (instanceId, setupId)
);
CREATE TABLE annotations (
    annotationId serial primary key,
    instanceId serial references instances(instanceId) on delete cascade,
    timeframe varchar(10) not null,
    entry text not null default '', 
    unique (instanceId, timeframe)
);
CREATE TABLE journals (
    journalId serial primary key,
    timestamp timestamp not null,
    userId serial references users(userId),
    entry text,
    unique (timestamp, userId)
);
CREATE TABLE journalInstances (
    journalId SERIAL REFERENCES journals(journalId) ON DELETE CASCADE,
    instanceId SERIAL REFERENCES instances(instanceId) ON DELETE CASCADE,
    position int,
    PRIMARY KEY (journalId, instanceId),
    unique (journalId, position)
);

