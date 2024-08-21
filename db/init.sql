
CREATE TABLE users (
    user_id SERIAL PRIMARY KEY,
    username VARCHAR(100) UNIQUE NOT NULL,
    password VARCHAR(100) NOT NULL,
    settings JSONB DEFAULT '{}'
);
CREATE INDEX idx_users ON users (username, password);

CREATE TABLE instances (
    instance_id serial PRIMARY KEY,
    user_id serial references users(user_id),
    security_id varchar(100),
    timestamp timestamp
);

create table training_instances (
    instance_id serial references instances(instances_id),
    setup_id serial references setups(setup_id),
    label boolean
)


CREATE TABLE annotations (
    instance_id serial references instances(instance_id),
    timeframe , 
    annotation text
);


CREATE TABLE journals (
    journal_id serial primary key,
    user_id serial references users(user_id),
    entry text
);


CREATE TABLE journal_instances (
    journal_id SERIAL REFERENCES journals(journal_id) ON DELETE CASCADE,
    instance_id SERIAL REFERENCES instances(instance_id) ON DELETE CASCADE,
    character pos
    PRIMARY KEY (journal_id, instance_id)
);

