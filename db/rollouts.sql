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
    entry_times TIMESTAMP[] DEFAULT ARRAY[]::TIMESTAMP[],
    entry_prices DECIMAL(10,4)[] DEFAULT ARRAY[]::DECIMAL(10,4)[],
    entry_shares INT[] DEFAULT ARRAY[]::INT[],
    -- Store up to 50 exits
    exit_times TIMESTAMP[] DEFAULT ARRAY[]::TIMESTAMP[],
    exit_prices DECIMAL(10,4)[] DEFAULT ARRAY[]::DECIMAL(10,4)[],
    exit_shares INT[] DEFAULT ARRAY[]::INT[],
    UNIQUE (userId, ticker, date)
);
CREATE TABLE trade_executions (
    executionId SERIAL PRIMARY KEY,
    userId INT REFERENCES users(userId) ON DELETE CASCADE,
    securityId INT,
    date DATE NOT NULL,
    price DECIMAL(10, 4) NOT NULL,
    size INT NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    direction VARCHAR(10) NOT NULL,
    tradeId INT REFERENCES trades(tradeId),
    UNIQUE (userId, securityId, timestamp)
);
