CREATE TABLE users (
    user_id SERIAL PRIMARY KEY,
    username VARCHAR(100) UNIQUE NOT NULL,
    password VARCHAR(100) NOT NULL,
    settings JSONB DEFAULT '{}'
);
CREATE INDEX idx_users ON users (username, password);

create table setups (
    setup_id serial primary key,
    name varchar(50) not null
);
CREATE TABLE instances (
    instance_id serial PRIMARY KEY,
    user_id serial references users(user_id) on delete cascade,
    security_id varchar(100) not null,
    timestamp timestamp not null,
    unique (user_id, security_id, timestamp)
);
create index idx_instances on instances (security_id, timestamp);
create table training_instances (
    instance_id serial references instances(instance_id) on delete cascade,
    setup_id serial references setups(setup_id) on delete cascade,
    label boolean,
    unique (instance_id, setup_id)
);
CREATE TABLE annotations (
    instance_id serial references instances(instance_id) on delete cascade,
    timeframe varchar(10) not null,
    annotation text, 
    unique (instance_id, timeframe)
);
CREATE TABLE journals (
    journal_id serial primary key,
    timestamp timestamp not null,
    user_id serial references users(user_id),
    entry text,
    unique (timestamp, user_id)
);
CREATE TABLE journal_instances (
    journal_id SERIAL REFERENCES journals(journal_id) ON DELETE CASCADE,
    instance_id SERIAL REFERENCES instances(instance_id) ON DELETE CASCADE,
    position int,
    PRIMARY KEY (journal_id, instance_id),
    unique (journal_id, position)
);

