-- Create schema_versions table to track migrations
CREATE TABLE IF NOT EXISTS schema_versions (
    version NUMERIC PRIMARY KEY,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    description TEXT
);

-------------
-- SET CURRENT SCHEMA VERSION
-------------
INSERT INTO schema_versions (version, description)
VALUES (
        16, 
        'Initial schema version'
    ) ON CONFLICT (version) DO NOTHING;
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
    userId int references users(userId) on delete cascade,
    watchlistName varchar(50) not null,
    unique(watchlistName, userId)
);
CREATE INDEX idxWatchlistIdUserId on watchlists(watchlistId, userId);
CREATE TABLE watchlistItems (
    watchlistItemId serial primary key,
    watchlistId int references watchlists(watchlistId) on delete cascade,
    securityId int, --serial references securities(securityId) on delete cascade,
    unique (watchlistId, securityId)
);
CREATE INDEX idxWatchlistId on watchlistItems(watchlistId);
-- The old alerts table is dropped and replaced by priceAlerts and strategyAlerts in migration 14

create table strategies (
    strategyId serial primary key,
    userId int references users(userId) on delete cascade,
    name varchar(50) not null,
    spec JSON,
    alertActive bool not null default false,
    unique (userId, name)
);
CREATE INDEX idxStrategiesByUserId ON strategies(strategyId);
CREATE TABLE studies (
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
CREATE INDEX idxStudiesByUserId on studies(userId);
CREATE INDEX idxStudiesByTagUserId on studies(userId,securityId,strategyId,timestamp,tradeId);

CREATE TABLE alerts (
    alertId SERIAL PRIMARY KEY,
    userId int references users(userId),
    active BOOLEAN NOT NULL DEFAULT false,
    triggeredTimestamp timestamp default null,
    price DECIMAL(10, 4),
    direction Boolean,
    securityId int-- references securities(securityId)
);
CREATE INDEX idxalertByUserId on alerts(userId);
CREATE INDEX idxalertByUserIdSecurityId on alerts(userId,securityId);

CREATE TABLE horizontal_lines (
    id serial primary key,
    userId int references users(userId) on delete cascade,
    securityId int, --references securities(securityId),
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


-- Create the ohlcv_1d table
CREATE TABLE IF NOT EXISTS ohlcv_1d (
    timestamp TIMESTAMP NOT NULL,
    securityid INTEGER NOT NULL,
    open DECIMAL(25, 6) NOT NULL,
    high DECIMAL(25, 6) NOT NULL,
    low DECIMAL(25, 6) NOT NULL,
    close DECIMAL(25, 6) NOT NULL,
    volume BIGINT NOT NULL,
    PRIMARY KEY (securityid, timestamp)
);

-- Convert to TimescaleDB hypertable with partitioning by securityid
SELECT create_hypertable('ohlcv_1d', 'timestamp', 
                         'securityid', number_partitions => 16,
                         chunk_time_interval => INTERVAL '1 month',
                         if_not_exists => TRUE);

-- Create indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_ohlcv_1d_securityid ON ohlcv_1d(securityid);
CREATE INDEX IF NOT EXISTS idx_ohlcv_1d_timestamp ON ohlcv_1d(timestamp DESC);

-- changes with time
CREATE TABLE fundamentals (
    security_id VARCHAR(20),-- REFERENCES securities(security_id),
    timestamp TIMESTAMP,
    market_cap DECIMAL(22,2),
    shares_outstanding BIGINT,
    eps DECIMAL(12,4),
    revenue DECIMAL(22,2),
    dividend DECIMAL(12,4),
    social_sentiment DECIMAL(10,4),
    fear_greed DECIMAL(10,4),
    short_interest DECIMAL(10,4),
    borrow_fee DECIMAL(10,4),
    PRIMARY KEY (security_id, timestamp)
);






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

-- Create query_logs table
CREATE TABLE IF NOT EXISTS query_logs (
    log_id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(userId),
    query_text TEXT NOT NULL,
    context JSONB, -- Store the provided context items
    response_type VARCHAR(50), -- e.g., 'text', 'mixed_content', 'function_calls', 'error'
    response_summary TEXT, -- Store a summary or error message
    llm_thinking_response TEXT, -- Raw response from the thinking model
    llm_final_response TEXT, -- Raw response from the final response model
    requested_functions JSONB, -- Store JSON array of function calls requested by LLM
    executed_functions JSONB, -- Store JSON array of function names called, if any
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_query_logs_user_id ON query_logs(user_id);
CREATE INDEX idx_query_logs_timestamp ON query_logs(timestamp);
