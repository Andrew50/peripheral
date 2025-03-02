/*CREATE TABLE IF NOT EXISTS horizontal_lines (
 id SERIAL PRIMARY KEY,
 userId INT REFERENCES users(userId) ON DELETE CASCADE,
 securityId INT,
 price FLOAT NOT NULL,
 UNIQUE (userId, securityId, price)
 );
 CREATE INDEX IF NOT EXISTS idxUserIdSecurityIdPrice ON horizontal_lines(userId, securityId, price);
 */
/*
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
 
 DROP TABLE trade_executions;
 DROP TABLE trades; 
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
 
 
 
 */
ALTER TABLE securities
ADD COLUMN name varchar(200),
    ADD COLUMN market varchar(50),
    ADD COLUMN locale varchar(50),
    ADD COLUMN primary_exchange varchar(50),
    ADD COLUMN active boolean DEFAULT true,
    ADD COLUMN market_cap decimal(20, 2),
    ADD COLUMN description text,
    ADD COLUMN logo text,
    ADD COLUMN icon text,
    ADD COLUMN share_class_shares_outstanding bigint;
ALTER TABLE users
ADD COLUMN profile_picture TEXT;
--ADD COLUMN google_id VARCHAR(255),
---ADD COLUMN email VARCHAR(255),
-------------------------
ALTER TABLE securities
ADD COLUMN total_shares BIGINT;


-----------------------

ALTER TABLE securities 
ADD COLUMN cik int; 

alter table securities
rename column cik to cik_varchar; 
alter table securities 
add column cik int; 


ALTER TABLE users ADD COLUMN IF NOT EXISTS auth_type VARCHAR(20);

-- Set default values:
-- If google_id is not null or empty, set auth_type to 'google'
-- Otherwise, set auth_type to 'password'
UPDATE users 
SET auth_type = CASE 
    WHEN google_id IS NOT NULL AND google_id != '' THEN 'google' 
    ELSE 'password' 
END
WHERE auth_type IS NULL;

-- Make the auth_type column non-nullable after populating it
ALTER TABLE users ALTER COLUMN auth_type SET NOT NULL; 
