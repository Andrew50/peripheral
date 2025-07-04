#!/bin/bash
set -euo pipefail

# Automated Recovery Script - Fresh Database Reset
# Called by the health monitor when no valid backups are available

LOG_FILE="/backups/recovery.log"

# Logging function
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] RESET: $1" | tee -a "$LOG_FILE"
}

error_log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] RESET ERROR: $1" | tee -a "$LOG_FILE" >&2
}

log "=== AUTOMATED FRESH DATABASE RESET STARTED ==="
log "WARNING: This will create a completely fresh database with no data"

# Database credentials
DB_USER=${POSTGRES_USER:-postgres}
DB_NAME=${POSTGRES_DB:-postgres}

# Stop PostgreSQL if running
log "Stopping PostgreSQL server..."
if pg_ctl -D "$PGDATA" status >/dev/null 2>&1; then
    pg_ctl -D "$PGDATA" -m fast stop || log "PostgreSQL was not running or failed to stop gracefully"
fi

# Remove corrupted data directory
log "Removing corrupted data directory..."
rm -rf "$PGDATA"/* || true
rm -rf "$PGDATA"/.* 2>/dev/null || true

# Initialize fresh PostgreSQL cluster
log "Initializing fresh PostgreSQL cluster..."
if initdb -D "$PGDATA" --auth-local=trust --auth-host=md5 --encoding=UTF8 --locale=C; then
    log "PostgreSQL cluster initialized successfully"
else
    error_log "Failed to initialize PostgreSQL cluster"
    exit 1
fi

# Start PostgreSQL
log "Starting PostgreSQL server..."
if pg_ctl -D "$PGDATA" -l /tmp/postgres.log start; then
    log "PostgreSQL started successfully"
else
    error_log "Failed to start PostgreSQL"
    exit 1
fi

# Wait for PostgreSQL to become ready
log "Waiting for PostgreSQL to become ready..."
for i in {1..30}; do
    if pg_isready -h localhost -p 5432 >/dev/null 2>&1; then
        log "PostgreSQL is ready"
        break
    fi
    if [ $i -eq 30 ]; then
        error_log "PostgreSQL failed to become ready after 30 attempts"
        exit 1
    fi
    sleep 1
done

# Create the main database if it doesn't exist
log "Creating database: $DB_NAME"
createdb "$DB_NAME" || log "Database $DB_NAME already exists or creation failed"

# Set password for postgres user
log "Setting up postgres user password..."
psql -d "$DB_NAME" -c "ALTER USER postgres PASSWORD '$POSTGRES_PASSWORD';" || true

# Run initialization scripts if they exist
if [ -d "/docker-entrypoint-initdb.d" ]; then
    log "Running initialization scripts..."
    for f in /docker-entrypoint-initdb.d/*; do
        if [ -f "$f" ]; then
            case "$f" in
                *.sql)
                    log "Running SQL script: $f"
                    psql -d "$DB_NAME" -f "$f" || error_log "Failed to run script: $f"
                    ;;
                *.sh)
                    log "Running shell script: $f"
                    bash "$f" || error_log "Failed to run script: $f"
                    ;;
                *)
                    log "Skipping file: $f"
                    ;;
            esac
        fi
    done
fi

# Run migrations if available
if [ -f "/app/migrate.sh" ]; then
    log "Running database migrations..."
    if /app/migrate.sh "$DB_NAME"; then
        log "Migrations completed successfully"
    else
        error_log "Migrations failed, but continuing with fresh database"
    fi
fi

# Verify the fresh database
log "Verifying fresh database..."
if pg_isready -h localhost -p 5432 >/dev/null 2>&1; then
    log "Database is accepting connections"
else
    error_log "Database is not accepting connections"
    exit 1
fi

# Test basic functionality
if psql -d "$DB_NAME" -c "SELECT 1;" >/dev/null 2>&1; then
    log "Basic database query successful"
else
    error_log "Basic database query failed"
    exit 1
fi

# Get database info
TABLES=$(psql -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';" | xargs)
SIZE=$(psql -d "$DB_NAME" -t -c "SELECT pg_size_pretty(pg_database_size('$DB_NAME'));" | xargs)

log "=== AUTOMATED FRESH RESET COMPLETED SUCCESSFULLY ==="
log "Fresh database statistics:"
log "  Tables: $TABLES"
log "  Size: $SIZE"
log "  Note: This is a fresh database with no user data"

exit 0 