
CREATE TABLE users (
    userId SERIAL PRIMARY KEY,
    username VARCHAR(100) UNIQUE NOT NULL,
    password VARCHAR(100) NOT NULL
);
CREATE INDEX idxUsers ON users (username, password);
CREATE TABLE securities {
    cik varchar(10) not null, 
    ticker varchar(8) not null
}
create index idxCik on securities (cik);
create table setups (
    setupId serial primary key,
    userId int references user(userId) on delete cascade, 
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
    sampleId serial references instances(instanceId) on delete cascade,
    instanceId integer references instances(instandId) on delete cascade,
    setupId serial references setups(setupId) on delete cascade,
    label boolean default null,
    unique (instanceId, setupId)
);
CREATE TABLE annotations (
    annotationId serial primary key,
    instanceId serial references instances(instanceId) on delete cascade,
    entry text not null default '', 
    completed boolean not null default false,
    unique (instanceId, timeframe)
);
CREATE TABLE journals (
    journalId serial primary key,
    timestamp timestamp not null,
    userId serial references users(userId),
    entry text,
    unique (timestamp, userId)
);
