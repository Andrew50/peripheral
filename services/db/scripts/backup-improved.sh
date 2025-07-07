#!/bin/bash
set -euo pipefail

# Improved Database Backup Script
# Includes verification, better error handling, and comprehensive logging

# Configuration
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
BACKUP_DIR="/backups"
BACKUP_FILE="$BACKUP_DIR/backup_$TIMESTAMP.sql"
LOG_FILE="$BACKUP_DIR/backup.log"
RETENTION_DAYS=30
MAX_RETRIES=3

# Database credentials from environment variables
DB_USER=${POSTGRES_USER:-postgres}
DB_NAME=${POSTGRES_DB:-postgres}
DB_HOST=${DB_HOST:-localhost}

# Logging function
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

error_log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: $1" | tee -a "$LOG_FILE" >&2
}

# Send Telegram notification for backup events
send_backup_alert() {
    local message="$1"
    local alert_type="${2:-INFO}"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S UTC')
    
    # Skip Telegram if credentials not configured
    if [ -z "$TELEGRAM_BOT_TOKEN" ] || [ -z "$TELEGRAM_CHAT_ID" ]; then
        return 0
    fi
    
    # Determine emoji based on alert type
    local emoji="ℹ️"
    case "$alert_type" in
        "ERROR") emoji="❌" ;;
        "WARNING") emoji="⚠️" ;;
        "SUCCESS") emoji="✅" ;;
        "INFO") emoji="ℹ️" ;;
    esac
    
    # Get environment info consistently to avoid literal placeholder output
    local env_info="${ENVIRONMENT:-Development}"
    
    # Format message for Telegram
    local telegram_message="$emoji *Database Backup - $alert_type*

*System:* PostgreSQL Backup System
*Time:* $timestamp
*Environment:* $env_info

*Message:*
$message

*Backup Status:*
• Backup Directory: /backups
• Retention Policy: $RETENTION_DAYS days
• Database: $DB_NAME@$DB_HOST"

    # Send to Telegram
    curl -s -X POST "https://api.telegram.org/bot$TELEGRAM_BOT_TOKEN/sendMessage" \
        -H "Content-Type: application/json" \
        -d "{
            \"chat_id\": \"$TELEGRAM_CHAT_ID\",
            \"text\": \"$telegram_message\",
            \"parse_mode\": \"Markdown\",
            \"disable_web_page_preview\": true
        }" >/dev/null 2>&1
}

# Ensure backup directory exists
mkdir -p "$BACKUP_DIR"

log "=== Database Backup Started ==="
log "Backup file: $BACKUP_FILE"

# Function to check database connectivity
check_db_connection() {
    local retry=0
    while [ $retry -lt $MAX_RETRIES ]; do
        if PGPASSWORD=$POSTGRES_PASSWORD pg_isready -U "$DB_USER" -d "$DB_NAME" -h "$DB_HOST"; then
            log "Database connection verified"
            return 0
        else
            retry=$((retry + 1))
            log "Database connection attempt $retry failed, retrying in 5 seconds..."
            sleep 5
        fi
    done
    error_log "Database connection failed after $MAX_RETRIES attempts"
    return 1
}

# Function to get database size
get_db_size() {
    PGPASSWORD=$POSTGRES_PASSWORD psql -U "$DB_USER" -d "$DB_NAME" -h "$DB_HOST" -t -c "SELECT pg_size_pretty(pg_database_size('$DB_NAME'));" | xargs
}

# Function to get table count
get_table_count() {
    PGPASSWORD=$POSTGRES_PASSWORD psql -U "$DB_USER" -d "$DB_NAME" -h "$DB_HOST" -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';" | xargs
}

# Function to verify backup integrity
verify_backup() {
    local backup_file="$1"
    
    if [ ! -f "$backup_file" ]; then
        error_log "Backup file not found: $backup_file"
        return 1
    fi
    
    # Check file size
    local file_size=$(stat -c%s "$backup_file")
    if [ "$file_size" -lt 1000 ]; then
        error_log "Backup file too small ($file_size bytes), likely failed"
        return 1
    fi
    
    # Check if file contains expected SQL content
    if ! grep -q "PostgreSQL database dump" "$backup_file"; then
        error_log "Backup file doesn't contain expected PostgreSQL dump header"
        return 1
    fi
    
    # Count tables in backup
    local backup_tables=$(grep -c "CREATE TABLE" "$backup_file" || echo "0")
    local db_tables=$(get_table_count)
    
    log "Database has $db_tables tables, backup contains $backup_tables table definitions"
    
    if [ "$backup_tables" -eq 0 ] && [ "$db_tables" -gt 0 ]; then
        error_log "Backup contains no tables but database has $db_tables tables"
        return 1
    fi
    
    log "Backup verification passed"
    return 0
}

# Main backup process
main() {
    # Check database connection
    if ! check_db_connection; then
        error_log "Cannot connect to database, aborting backup"
        send_backup_alert "Database backup failed: Cannot connect to database $DB_NAME@$DB_HOST" "ERROR"
        exit 1
    fi
    
    # Get database info
    local db_size=$(get_db_size)
    local table_count=$(get_table_count)
    log "Database size: $db_size, Tables: $table_count"
    
    # Create backup with verbose output
    log "Starting pg_dump..."
    
    # Use comprehensive pg_dump options
    if PGPASSWORD=$POSTGRES_PASSWORD pg_dump \
        -U "$DB_USER" \
        -d "$DB_NAME" \
        -h "$DB_HOST" \
        --verbose \
        --no-owner \
        --no-privileges \
        --create \
        --clean \
        --if-exists \
        > "$BACKUP_FILE" 2>>"$LOG_FILE"; then
        
        log "pg_dump completed successfully"
    else
        error_log "pg_dump failed"
        rm -f "$BACKUP_FILE"
        exit 1
    fi
    
    # Verify backup integrity
    if ! verify_backup "$BACKUP_FILE"; then
        error_log "Backup verification failed"
        send_backup_alert "Database backup failed: Backup verification failed for $BACKUP_FILE" "ERROR"
        rm -f "$BACKUP_FILE"
        exit 1
    fi
    
    # Get backup file size
    local backup_size=$(stat -c%s "$BACKUP_FILE")
    log "Backup file size: $(numfmt --to=iec-i --suffix=B --format="%.1f" "$backup_size")"
    
    # Compress the backup file
    log "Compressing backup..."
    if gzip "$BACKUP_FILE"; then
        local compressed_size=$(stat -c%s "$BACKUP_FILE.gz")
        local compression_ratio=$(( (backup_size - compressed_size) * 100 / backup_size ))
        log "Backup compressed successfully: $BACKUP_FILE.gz"
        log "Compression ratio: ${compression_ratio}% ($(numfmt --to=iec-i --suffix=B --format="%.1f" "$compressed_size"))"
    else
        error_log "Backup compression failed"
        exit 1
    fi
    
    # Create backup manifest
    cat > "$BACKUP_DIR/backup_${TIMESTAMP}.manifest" <<EOF
{
    "timestamp": "$TIMESTAMP",
    "database": "$DB_NAME",
    "original_size": $backup_size,
    "compressed_size": $(stat -c%s "$BACKUP_FILE.gz"),
    "table_count": $table_count,
    "database_size": "$db_size",
    "backup_file": "backup_${TIMESTAMP}.sql.gz"
}
EOF
    
    # Cleanup old backups
    log "Cleaning up backups older than $RETENTION_DAYS days..."
    local deleted_count=$(find "$BACKUP_DIR" -name "backup_*.sql.gz" -type f -mtime +$RETENTION_DAYS -delete -print | wc -l)
    find "$BACKUP_DIR" -name "backup_*.manifest" -type f -mtime +$RETENTION_DAYS -delete
    log "Removed $deleted_count old backup files"
    
    # List recent backups
    log "Recent backups:"
    ls -lah "$BACKUP_DIR"/backup_*.sql.gz | tail -5 | while read line; do
        log "  $line"
    done
    
    log "=== Database Backup Completed Successfully ==="
    
    # Send success notification
    local success_message="Database backup completed successfully!

📊 *Backup Details:*
• File: backup_${TIMESTAMP}.sql.gz
• Size: $(numfmt --to=iec-i --suffix=B --format="%.1f" $(stat -c%s "$BACKUP_FILE.gz"))
• Tables: $table_count
• Database Size: $db_size
• Compression: ${compression_ratio}%

📅 *Schedule:* Next backup at 6 AM/6 PM UTC"
    
    send_backup_alert "$success_message" "SUCCESS"
}

# Run main function
main "$@" 