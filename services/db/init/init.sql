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
CREATE INDEX trgm_idx_securities_name ON securities USING gin (name gin_trgm_ops);
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

create table strategies (
    strategyId serial primary key,
    userId int references users(userId) on delete cascade,
    name varchar(100) not null,
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

-- Multiple Conversations Support (Migration 19)
-- Main conversations table
CREATE TABLE IF NOT EXISTS conversations (
    conversation_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id INTEGER NOT NULL REFERENCES users(userid) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    metadata JSONB DEFAULT '{}',
    
    -- Track total conversation stats
    total_token_count INTEGER DEFAULT 0,
    message_count INTEGER DEFAULT 0
);

-- Conversation messages table
CREATE TABLE IF NOT EXISTS conversation_messages (
    message_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(conversation_id) ON DELETE CASCADE,
    
    -- Message content and metadata (matches current ChatMessage structure)
    query TEXT NOT NULL,
    response_text TEXT DEFAULT '',
    content_chunks JSONB DEFAULT '[]',
    function_calls JSONB DEFAULT '[]',
    tool_results JSONB DEFAULT '[]',
    context_items JSONB DEFAULT '[]',
    suggested_queries JSONB DEFAULT '[]',
    citations JSONB DEFAULT '[]',
    
    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP WITH TIME ZONE,
    
    -- Status and metadata
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'completed', 'error')),
    token_count INTEGER DEFAULT 0,
    archived BOOLEAN NOT NULL DEFAULT FALSE,
    
    -- Ordering within conversation
    message_order INTEGER NOT NULL,
    
    UNIQUE(conversation_id, message_order)
);

-- Indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_conversations_user_id ON conversations(user_id);
CREATE INDEX IF NOT EXISTS idx_conversations_created_at ON conversations(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_conversations_updated_at ON conversations(updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_conversation_messages_conversation_id ON conversation_messages(conversation_id);
CREATE INDEX IF NOT EXISTS idx_conversation_messages_created_at ON conversation_messages(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_conversation_messages_status ON conversation_messages(status);
CREATE INDEX IF NOT EXISTS idx_conversation_messages_order ON conversation_messages(conversation_id, message_order);

-- GIN indexes for JSONB columns that might be searched
CREATE INDEX IF NOT EXISTS idx_conversation_messages_context_items ON conversation_messages USING GIN(context_items);
CREATE INDEX IF NOT EXISTS idx_conversation_messages_suggested_queries ON conversation_messages USING GIN(suggested_queries);

-- Function to update conversation updated_at timestamp when messages are modified
CREATE OR REPLACE FUNCTION update_conversation_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    -- Update the parent conversation's updated_at timestamp
    UPDATE conversations 
    SET updated_at = CURRENT_TIMESTAMP 
    WHERE conversation_id = COALESCE(NEW.conversation_id, OLD.conversation_id);
    
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

-- Trigger to automatically update conversation timestamps
CREATE TRIGGER trigger_update_conversation_updated_at
    AFTER INSERT OR UPDATE OR DELETE ON conversation_messages
    FOR EACH ROW
    EXECUTE FUNCTION update_conversation_updated_at();

-- Function to auto-increment message_order
CREATE OR REPLACE FUNCTION set_message_order()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.message_order IS NULL THEN
        SELECT COALESCE(MAX(message_order), 0) + 1
        INTO NEW.message_order
        FROM conversation_messages
        WHERE conversation_id = NEW.conversation_id;
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to automatically set message order
CREATE TRIGGER trigger_set_message_order
    BEFORE INSERT ON conversation_messages
    FOR EACH ROW
    EXECUTE FUNCTION set_message_order();

-- Function to update conversation stats (token count, message count)
CREATE OR REPLACE FUNCTION update_conversation_stats()
RETURNS TRIGGER AS $$
BEGIN
    -- Update conversation statistics 
    UPDATE conversations 
    SET 
        total_token_count = (
            SELECT COALESCE(SUM(token_count), 0) 
            FROM conversation_messages 
            WHERE conversation_id = COALESCE(NEW.conversation_id, OLD.conversation_id)
            AND archived = FALSE
        ),
        message_count = (
            SELECT COUNT(*) 
            FROM conversation_messages 
            WHERE conversation_id = COALESCE(NEW.conversation_id, OLD.conversation_id)
            AND archived = FALSE
        )
    WHERE conversation_id = COALESCE(NEW.conversation_id, OLD.conversation_id);
    
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

-- Trigger to automatically update conversation stats
CREATE TRIGGER trigger_update_conversation_stats
    AFTER INSERT OR UPDATE OR DELETE ON conversation_messages
    FOR EACH ROW
    EXECUTE FUNCTION update_conversation_stats();

-- End Multiple Conversations Support

-- Why Is It Moving table for tracking stock movement explanations
CREATE TABLE why_is_it_moving (
    id SERIAL PRIMARY KEY,
    securityid int, 
    ticker VARCHAR(10) NOT NULL,
    content TEXT NOT NULL,
    source VARCHAR(100), -- Optional: track the source of the information
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
    
);

-- Create indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_why_is_it_moving_ticker ON why_is_it_moving(ticker);
CREATE INDEX IF NOT EXISTS idx_why_is_it_moving_created_at ON why_is_it_moving(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_why_is_it_moving_ticker_date ON why_is_it_moving(ticker, created_at DESC);

COPY securities(securityid, ticker, figi, minDate, maxDate)
FROM '/docker-entrypoint-initdb.d/securities.csv' DELIMITER ',' CSV HEADER;

