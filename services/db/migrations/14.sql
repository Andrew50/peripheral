-- drop to remake later as its just simpler
drop table if exists studies;
DROP table if exists alerts;
drop table if exists samples;
drop table if exists setups;
drop table if exists journals;
drop table if exists notes; --remove permentantly

create table strategies (
    strategyId serial primary key,
    userId int references users(userId) on delete cascade,
    name varchar(50) not null,
    criteria JSON,
    unique (userId, name)
);
CREATE INDEX idxStrategiesByUserId on strategies(strategyId);
CREATE INDEX idxStrategiesByStrategyId on strategies(strategyId);
CREATE TABLE studies (
    studyId serial primary key,
    userId serial references users(userId) on delete cascade,
    securityId int, -- optional security id references securities(securityId), --cant because not unique
    strategyId int, -- referneces strategies(strategyId) but can be null
    timestamp timestamp, 
    tradeId int,
    completed boolean not null default false,
    entry json,
    unique(userId, securityId, strategyId, timestamp, tradeId)
);
CREATE INDEX idxStudiesByUserId on studies(userId);
CREATE INDEX idxStudiesByTagUserId on studies(userId,securityId,strategyId,timestamp,tradeId);

CREATE TABLE priceAlerts (
    priceAlertId SERIAL PRIMARY KEY,
    userId SERIAL references users(userId),
    active BOOLEAN NOT NULL DEFAULT false,
    price DECIMAL(10, 4),
    direction Boolean,
    securityID serial references securities(securityId)
);
CREATE INDEX idxPriceAlertByUserId on priceAlerts(userId);
CREATE INDEX idxPriceAlertByUserIdSecurityId on priceAlerts(userId,securityId);


CREATE TABLE strategyAlerts (
    strategyAlertId SERIAL PRIMARY KEY,
    userId SERIAL references users(userId),
    active BOOLEAN NOT NULL DEFAULT false,
    strategyId serial references strategies(strategyId),
    direction Boolean,
    securityID serial references securities(securityId)
);
CREATE INDEX idxStrategyAlertByUserId on strategyAlerts(userId);
CREATE INDEX idxStrategyAlertByUserIdSecurityId on strategyAlerts(userId,securityId);

