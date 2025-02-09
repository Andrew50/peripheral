/*CREATE TABLE IF NOT EXISTS horizontal_lines (
 id SERIAL PRIMARY KEY,
 userId INT REFERENCES users(userId) ON DELETE CASCADE,
 securityId INT,
 price FLOAT NOT NULL,
 UNIQUE (userId, securityId, price)
 );
 CREATE INDEX IF NOT EXISTS idxUserIdSecurityIdPrice ON horizontal_lines(userId, securityId, price);
 */
CREATE TABLE trades (
    tradeId SERIAL PRIMARY KEY,
    userId INT REFERENCES users(userId) ON DELETE CASCADE,
    ticker VARCHAR(10) NOT NULL,
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
    exit_shares INT [] DEFAULT ARRAY []::INT [],
    UNIQUE (userId, ticker, date)
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
    tradeId INT REFERENCES trades(tradeId),
    UNIQUE (userId, securityId, timestamp)
);
ALTER TABLE securities
ADD COLUMN sector VARCHAR(100),
    ADD COLUMN industry VARCHAR(100);
CREATE TABLE alerts (
    alertId SERIAL PRIMARY KEY,
    userId SERIAL REFERENCES users(userId) ON DELETE CASCADE,
    active BOOLEAN NOT NULL DEFAULT false,
    alertType VARCHAR(10) NOT NULL CHECK (alertType IN ('price', 'setup', 'algo')),
    -- Restrict the allowed alert types
    setupId INT REFERENCES setups(setupId) ON DELETE CASCADE,
    --algoId INT REFERENCES algos(algoId) ON DELETE CASCADE,
    algoId INT,
    price DECIMAL(10, 4),
    direction Boolean,
    securityID INT,
    CONSTRAINT chk_alert_price_or_setup CHECK (
        (
            alertType = 'price'
            AND price IS NOT NULL
            AND securityID IS NOT NULL
            AND direction IS NOT NULL
            AND algoId IS NULL
            AND setupId IS NULL
        )
        OR (
            alertType = 'setup'
            AND setupId IS NOT NULL
            AND algoId IS NULL
            AND price IS NULL
            AND securityID IS NULL
        )
        OR (
            alertType = 'algo'
            AND algoId IS NOT NULL
            AND setupId IS NULL
            AND price IS NULL
        )
    )
);
CREATE INDEX idxAlertByUserId on alerts(userId);
CREATE TABLE alertLogs (
    alertLogId serial primary key,
    alertId serial references alerts(alertId) on delete cascade,
    timestamp timestamp not null,
    securityId INT,
    --references sercurities
    unique(alertId, timestamp, securityId)
);
CREATE INDEX idxAlertLogId on alertLogs(alertLogId);




--new
CREATE EXTENSION IF NOT EXISTS pg_trgm;