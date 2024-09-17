#!/bin/bash

# Timestamp for the backup file
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")

# Backup file location (use the /backups directory mounted from the host)
BACKUP_DIR="/backups"
BACKUP_FILE="$BACKUP_DIR/backup_$TIMESTAMP.sql"

# Run pg_dump to create the backup
pg_dump -U postgres_user -d database_name > "$BACKUP_FILE"

# Optional: compress the backup file to save space
gzip "$BACKUP_FILE"
