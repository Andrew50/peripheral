-- Create schema_versions table to track migrations
CREATE TABLE IF NOT EXISTS schema_versions (
    version NUMERIC PRIMARY KEY,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    description TEXT
);
-- Insert hardcoded entry for migrations up to version 10, when adding migrations 
-- to the init.sql file, update the version number here
INSERT INTO schema_versions (version, description)
VALUES (
        14,
        'Initial schema version - all migrations up to 14 included in init.sql'
    ) ON CONFLICT (version) DO NOTHING;
-- Schema versions will be populated by the migration script--init.sql
CREATE EXTENSION IF NOT EXISTS timescaledb;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE TABLE users (
    userId SERIAL PRIMARY KEY,
    username VARCHAR(100) UNIQUE NOT NULL,
    password VARCHAR(100) NOT NULL,
    settings JSON,
    email VARCHAR(255),
    google_id VARCHAR(255),
    profile_picture TEXT,
    auth_type VARCHAR(20) DEFAULT 'password' -- 'password' for password-only auth, 'google' for Google-only auth, 'both' for users who can use either method
);
CREATE INDEX idxUsers ON users (username, password);
CREATE INDEX idxUserAuthType ON users(auth_type);

-- Added in migration 14 (replaces setups)
create table strategies (
    strategyId serial primary key,
    userId int references users(userId) on delete cascade,
    name varchar(50) not null,
    criteria JSON,
    unique (userId, name)
);
CREATE INDEX idxStrategiesByUserId on strategies(strategyId);
CREATE INDEX idxStrategiesByStrategyId on strategies(strategyId);

-- Added in migration 14 (replaces old studies)
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

-- Added in migration 14 (replaces part of alerts)
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

-- Added in migration 14 (replaces part of alerts)
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

CREATE TABLE securities (
    securityid SERIAL,
    ticker varchar(10) not null,
    figi varchar(12) not null,
    name varchar(200),
    market varchar(50),
    locale varchar(50),
    primary_exchange varchar(50),
    active boolean DEFAULT true,
    market_cap numeric,
    description text,
    logo text,
    -- base64 encoded image
    icon text,
    -- base64 encoded image
    share_class_shares_outstanding bigint,
    sector varchar(100),
    industry varchar(100),
    minDate timestamp,
    maxDate timestamp,
    cik bigint,
    total_shares bigint,
    unique (ticker, minDate),
    unique (ticker, maxDate),
    unique (securityid, minDate),
    unique (securityid, maxDate)
);
CREATE INDEX trgm_idx_securities_ticker ON securities USING gin (ticker gin_trgm_ops);
create index idxTickerDateRange on securities (ticker, minDate, maxDate);
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
-- The old alerts table is dropped and replaced by priceAlerts and strategyAlerts in migration 14
CREATE TABLE alertLogs (
    alertLogId serial primary key,
    -- alertId now refers conceptually to either priceAlertId or strategyAlertId,
    -- but we can't use a direct FK constraint easily. This might need application logic adjustment.
    -- Consider adding separate FKs or a type column if strict FKs are needed.
    alertId int, -- Was: serial references alerts(alertId) on delete cascade,
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
    color varchar(20) DEFAULT '#FFFFFF',
    -- Default to white
    line_width int DEFAULT 1,
    -- Default to 1px
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
-- Create the daily OHLCV table for storing time-series market data
CREATE TABLE IF NOT EXISTS daily_ohlcv (
    timestamp TIMESTAMP NOT NULL,
    securityid INTEGER NOT NULL,
    ticker VARCHAR(10) NOT NULL,
    open DECIMAL(25, 6) NOT NULL,
    high DECIMAL(25, 6) NOT NULL,
    low DECIMAL(25, 6) NOT NULL,
    close DECIMAL(25, 6) NOT NULL,
    volume BIGINT NOT NULL,
    vwap DECIMAL(25, 6),
    transactions INTEGER,
    market_cap DECIMAL(25, 6),
    share_class_shares_outstanding BIGINT,
    CONSTRAINT unique_security_date UNIQUE (securityid, timestamp)
);
-- Convert to TimescaleDB hypertable
SELECT create_hypertable('daily_ohlcv', 'timestamp');
-- Create indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_daily_ohlcv_security_id ON daily_ohlcv(securityid);
CREATE INDEX IF NOT EXISTS idx_daily_ohlcv_ticker ON daily_ohlcv(ticker);
CREATE INDEX IF NOT EXISTS idx_daily_ohlcv_timestamp_desc ON daily_ohlcv(timestamp DESC);
COPY securities(securityid, ticker, figi, minDate, maxDate)
FROM '/docker-entrypoint-initdb.d/securities.csv' DELIMITER ',' CSV HEADER;
-- Create the guest account with user ID 0
INSERT INTO users (userId, username, password, email, auth_type)
VALUES (
        0,
        'Guest',
        'guest-password',
        'guest@atlantis.local',
        'guest'
    );
CREATE UNIQUE INDEX idx_users_email ON users(email)
WHERE email IS NOT NULL;
