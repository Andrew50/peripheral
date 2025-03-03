#!/bin/bash

# Timestamp for the backup file
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")

# Backup file location (use the /backups directory mounted from the host)
BACKUP_DIR="/backups"
BACKUP_FILE="$BACKUP_DIR/backup_$TIMESTAMP.sql"

# Ensure backup directory exists
mkdir -p "$BACKUP_DIR"

# Database credentials from environment variables
DB_USER=${POSTGRES_USER:-postgres}
DB_NAME=${POSTGRES_DB:-postgres}

# Log the backup start
echo "[$(date '+%Y-%m-%d %H:%M:%S')] Starting database backup..."

# Run pg_dump to create the backup
PGPASSWORD=$POSTGRES_PASSWORD pg_dump -U "$DB_USER" -d "$DB_NAME" > "$BACKUP_FILE"
if [ $? -eq 0 ]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Database backup created successfully: $BACKUP_FILE"
else
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: Database backup failed"
    exit 1
fi

# Compress the backup file to save space
gzip "$BACKUP_FILE"
if [ $? -eq 0 ]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Backup compressed successfully: $BACKUP_FILE.gz"
else
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] WARNING: Backup compression failed"
fi

# Optional: Remove backups older than 30 days
find "$BACKUP_DIR" -name "backup_*.sql.gz" -type f -mtime +30 -delete
echo "[$(date '+%Y-%m-%d %H:%M:%S')] Removed backups older than 30 days"

echo "[$(date '+%Y-%m-%d %H:%M:%S')] Backup process completed"
