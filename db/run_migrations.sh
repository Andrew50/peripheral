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
    
    # Apply the migration
    PGPASSWORD=$POSTGRES_PASSWORD psql -U postgres -d "$DB_NAME" -f "$MIGRATION_FILE" || {
      error_log "Failed to apply migration: $VERSION"
      exit 1
    }
    
    # Record the migration in schema_versions
    PGPASSWORD=$POSTGRES_PASSWORD psql -U postgres -d "$DB_NAME" -c "
      INSERT INTO schema_versions (version, description)
      VALUES ('$VERSION', '$DESCRIPTION')
      ON CONFLICT (version) DO NOTHING;
    " || {
      error_log "Failed to record migration in schema_versions: $VERSION"
      exit 1
    }
    
    log "Successfully applied migration: $VERSION"
  else
    log "Migration already applied: $VERSION"
  fi
done

log "All migrations completed successfully" 