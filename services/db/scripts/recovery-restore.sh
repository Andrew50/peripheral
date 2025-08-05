#!/bin/bash
set -euo pipefail

# Automated Recovery Script - Restore from Backup
# Called by the health monitor for automatic recovery

BACKUP_FILE="$1"
LOG_FILE="/backups/recovery.log"

# Logging function
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] RECOVERY: $1" | tee -a "$LOG_FILE"
}

error_log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] RECOVERY ERROR: $1" | tee -a "$LOG_FILE" >&2
}

if [ $# -ne 1 ] || [ ! -f "$BACKUP_FILE" ]; then
    error_log "Usage: $0 <backup_file.sql.gz>"
    error_log "Backup file must exist and be readable"
    exit 1
fi

log "=== AUTOMATED RECOVERY FROM BACKUP STARTED ==="
log "Backup file: $BACKUP_FILE"
log "File size: $(stat -c%s "$BACKUP_FILE") bytes"

# Database credentials
DB_USER=${POSTGRES_USER:-postgres}
DB_NAME=${POSTGRES_DB:-postgres}

# Stop accepting new connections (if possible)
log "Attempting to put database in maintenance mode..."
PGPASSWORD=$POSTGRES_PASSWORD psql -U "$DB_USER" -d "$DB_NAME" -c "
    UPDATE pg_database SET datallowconn = false WHERE datname = '$DB_NAME';
    SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '$DB_NAME' AND pid <> pg_backend_pid();
" 2>/dev/null || log "Could not put database in maintenance mode (database may be corrupted)"

# Create a temporary database for restoration
TEMP_DB="${DB_NAME}_recovery_$(date +%s)"
log "Creating temporary database: $TEMP_DB"

if PGPASSWORD=$POSTGRES_PASSWORD createdb -U "$DB_USER" "$TEMP_DB" 2>/dev/null; then
    log "Temporary database created successfully"
else
    error_log "Failed to create temporary database"
    exit 1
fi

# Extract and restore backup to temporary database
log "Extracting and restoring backup..."
if gunzip -c "$BACKUP_FILE" | PGPASSWORD=$POSTGRES_PASSWORD psql -U "$DB_USER" -d "$TEMP_DB" >/dev/null 2>>"$LOG_FILE"; then
    log "Backup restored to temporary database successfully"
else
    error_log "Failed to restore backup to temporary database"
    PGPASSWORD=$POSTGRES_PASSWORD dropdb -U "$DB_USER" "$TEMP_DB" 2>/dev/null || true
    exit 1
fi

# Verify the restored database
log "Verifying restored database..."
RESTORED_TABLES=$(PGPASSWORD=$POSTGRES_PASSWORD psql -U "$DB_USER" -d "$TEMP_DB" -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';" | xargs)
log "Restored database contains $RESTORED_TABLES tables"

if [ "$RESTORED_TABLES" -eq 0 ]; then
    error_log "Restored database appears to be empty"
    PGPASSWORD=$POSTGRES_PASSWORD dropdb -U "$DB_USER" "$TEMP_DB" 2>/dev/null || true
    exit 1
fi

# Test schema_versions table
if ! PGPASSWORD=$POSTGRES_PASSWORD psql -U "$DB_USER" -d "$TEMP_DB" -c "SELECT COUNT(*) FROM schema_versions;" >/dev/null 2>&1; then
    error_log "Schema_versions table not found in restored database"
    PGPASSWORD=$POSTGRES_PASSWORD dropdb -U "$DB_USER" "$TEMP_DB" 2>/dev/null || true
    exit 1
fi

# Replace the corrupted database with the restored one
log "Replacing corrupted database with restored database..."

# Drop the corrupted database
if PGPASSWORD=$POSTGRES_PASSWORD dropdb -U "$DB_USER" "$DB_NAME" 2>/dev/null; then
    log "Corrupted database dropped"
else
    error_log "Failed to drop corrupted database"
    PGPASSWORD=$POSTGRES_PASSWORD dropdb -U "$DB_USER" "$TEMP_DB" 2>/dev/null || true
    exit 1
fi

# Rename temporary database to original name
if PGPASSWORD=$POSTGRES_PASSWORD psql -U postgres -c "ALTER DATABASE \"$TEMP_DB\" RENAME TO \"$DB_NAME\";" 2>/dev/null; then
    log "Database renamed successfully"
else
    error_log "Failed to rename temporary database"
    exit 1
fi

# Re-enable connections
log "Re-enabling database connections..."
PGPASSWORD=$POSTGRES_PASSWORD psql -U "$DB_USER" -d "$DB_NAME" -c "UPDATE pg_database SET datallowconn = true WHERE datname = '$DB_NAME';" 2>/dev/null || true

# Verify the restored database is working
log "Verifying restored database functionality..."
if PGPASSWORD=$POSTGRES_PASSWORD pg_isready -U "$DB_USER" -d "$DB_NAME" -h localhost >/dev/null 2>&1; then
    log "Database is accepting connections"
else
    error_log "Database is not accepting connections after restore"
    exit 1
fi

# Test basic functionality
if PGPASSWORD=$POSTGRES_PASSWORD psql -U "$DB_USER" -d "$DB_NAME" -c "SELECT 1;" >/dev/null 2>&1; then
    log "Basic database query successful"
else
    error_log "Basic database query failed"
    exit 1
fi



# Get final database info
FINAL_TABLES=$(PGPASSWORD=$POSTGRES_PASSWORD psql -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';" | xargs)
FINAL_SIZE=$(PGPASSWORD=$POSTGRES_PASSWORD psql -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT pg_size_pretty(pg_database_size('$DB_NAME'));" | xargs)

log "=== AUTOMATED RECOVERY COMPLETED SUCCESSFULLY ==="
log "Final database statistics:"
log "  Tables: $FINAL_TABLES"
log "  Size: $FINAL_SIZE"
log "  Backup used: $BACKUP_FILE"

exit 0 