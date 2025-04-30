BEGIN;

-- drop to remake later as its just simpler
drop table if exists alertLogs;
drop table if exists studies;
DROP table if exists alerts;
drop table if exists samples;
drop table if exists setups;
drop table if exists journals;
drop table if exists notes; --remove permentantly

create table IF NOT EXISTS strategies (
    strategyId serial primary key,
    userId int references users(userId) on delete cascade,
    name varchar(50) not null,
    criteria JSON,
    alertActive bool not null default false,
    unique (userId, name)
);
CREATE INDEX IF NOT EXISTS idxStrategiesByUserId ON strategies(strategyId);
CREATE TABLE IF NOT EXISTS studies (
    studyId serial primary key,
    userId int references users(userId) on delete cascade,
    securityId int, --security id isnt unique,
    strategyId int null references strategies(strategyId),
    timestamp timestamp, 
    tradeId int,
    completed boolean not null default false,
    entry json,
    unique(userId, securityId, strategyId, timestamp, tradeId)
);
CREATE INDEX IF NOT EXISTS idxStudiesByUserId on studies(userId);
CREATE INDEX IF NOT EXISTS idxStudiesByTagUserId on studies(userId,securityId,strategyId,timestamp,tradeId);

CREATE TABLE IF NOT EXISTS alerts (
    alertId SERIAL PRIMARY KEY,
    userId int references users(userId),
    active BOOLEAN NOT NULL DEFAULT false,
    triggeredTimestamp timestamp default null,
    price DECIMAL(10, 4),
    direction Boolean,
    securityId int-- references securities(securityId)
);
CREATE INDEX IF NOT EXISTS idxalertByUserId on alerts(userId);
CREATE INDEX IF NOT EXISTS idxalertByUserIdSecurityId on alerts(userId,securityId);


COMMIT;
