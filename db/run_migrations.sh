#!/bin/bash
set -e

DB_NAME="$1"
MIGRATIONS_DIR="/tmp/rollouts"

# Function to log messages with timestamps
log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

# Function to log errors
error_log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: $1" >&2
}

# Ensure the schema_versions table exists (should be in 000_create_schema_versions.sql)
log "Ensuring schema_versions table exists..."
PGPASSWORD=$POSTGRES_PASSWORD psql -U postgres -d "$DB_NAME" -c "
CREATE TABLE IF NOT EXISTS schema_versions (
    version VARCHAR(50) PRIMARY KEY,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    description TEXT
);" || {
  error_log "Failed to create schema_versions table"
  exit 1
}

# Set a longer statement timeout to allow for long-running migrations
log "Setting statement timeout to 5 minutes..."
PGPASSWORD=$POSTGRES_PASSWORD psql -U postgres -d "$DB_NAME" -c "SET statement_timeout = '300000';" || {
  log "Warning: Failed to set statement timeout"
}

# Get list of migration files sorted by version
MIGRATION_FILES=$(find "$MIGRATIONS_DIR" -name "*.sql" | sort)

# Process each migration file
for MIGRATION_FILE in $MIGRATION_FILES; do
  FILENAME=$(basename "$MIGRATION_FILE")
  VERSION="${FILENAME%.sql}"
  
  log "Checking migration: $VERSION"
  
  # Check if this migration has already been applied
  APPLIED=$(PGPASSWORD=$POSTGRES_PASSWORD psql -U postgres -d "$DB_NAME" -t -c "
    SELECT COUNT(*) FROM schema_versions WHERE version = '$VERSION';
  ")
  
  if [ "$(echo $APPLIED | tr -d ' ')" -eq "0" ]; then
    log "Applying migration: $VERSION"
    
    # Extract description from the migration file (second line after -- Description:)
    DESCRIPTION=$(grep -A 1 "^-- Description:" "$MIGRATION_FILE" | tail -n 1 | sed 's/^-- //')
    if [ -z "$DESCRIPTION" ]; then
      DESCRIPTION="$VERSION migration"
    fi
    
    # Create a temporary file with transaction control and lock timeout
    TEMP_SQL=$(mktemp)
    cat > "$TEMP_SQL" << EOF
-- Set lock timeout to 10 seconds for this transaction
SET lock_timeout = '10000';

-- Start transaction
BEGIN;

-- Apply the migration
$(cat "$MIGRATION_FILE")

-- Record the migration in schema_versions
INSERT INTO schema_versions (version, description)
VALUES ('$VERSION', '$DESCRIPTION')
ON CONFLICT (version) DO NOTHING;

-- Commit transaction
COMMIT;
EOF
    
    # Apply the migration with retries
    MAX_RETRIES=3
    RETRY_COUNT=0
    SUCCESS=false
    
    while [ $RETRY_COUNT -lt $MAX_RETRIES ] && [ "$SUCCESS" != "true" ]; do
      if [ $RETRY_COUNT -gt 0 ]; then
        log "Retrying migration ($RETRY_COUNT of $MAX_RETRIES)..."
        sleep 5
      fi
      
      PGPASSWORD=$POSTGRES_PASSWORD psql -U postgres -d "$DB_NAME" -f "$TEMP_SQL" 2>&1 && SUCCESS=true
      
      if [ "$SUCCESS" != "true" ]; then
        RETRY_COUNT=$((RETRY_COUNT + 1))
        log "Migration attempt failed. Error output:"
        PGPASSWORD=$POSTGRES_PASSWORD psql -U postgres -d "$DB_NAME" -f "$TEMP_SQL" 2>&1 || true
      fi
    done
    
    # Clean up temp file
    rm -f "$TEMP_SQL"
    
    if [ "$SUCCESS" = "true" ]; then
      log "Successfully applied migration: $VERSION"
    else
      error_log "Failed to apply migration after $MAX_RETRIES attempts: $VERSION"
      exit 1
    fi
  else
    log "Migration already applied: $VERSION"
  fi
done

log "All migrations completed successfully" 