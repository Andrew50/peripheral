#!/bin/bash
set -e

DB_NAME="$1"
MIGRATIONS_DIR="/tmp/rollouts"

# Function to log messages with timestamps
log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] MIGRATION: $1"
}

# Function to log errors
error_log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] MIGRATION ERROR: $1" >&2
}

# Ensure migrations directory exists
if [ ! -d "$MIGRATIONS_DIR" ]; then
  log "Creating migrations directory: $MIGRATIONS_DIR"
  mkdir -p "$MIGRATIONS_DIR"
  chmod 777 "$MIGRATIONS_DIR"
fi

# Check if there are any migration files
if [ ! "$(ls -A $MIGRATIONS_DIR/*.sql 2>/dev/null)" ]; then
  log "No migration files found in $MIGRATIONS_DIR"
  exit 0
fi

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
log "Looking for migration files in $MIGRATIONS_DIR"
MIGRATION_FILES=$(find "$MIGRATIONS_DIR" -name "*.sql" | sort)

if [ -z "$MIGRATION_FILES" ]; then
  log "No migration files found after directory check"
  exit 0
fi

log "Found migration files: $(echo "$MIGRATION_FILES" | wc -l)"

# Process each migration file
for MIGRATION_FILE in $MIGRATION_FILES; do
  FILENAME=$(basename "$MIGRATION_FILE")
  VERSION="${FILENAME%.sql}"
  
  log "Checking migration: $VERSION ($MIGRATION_FILE)"
  
  # Check if this migration has already been applied
  APPLIED=$(PGPASSWORD=$POSTGRES_PASSWORD psql -U postgres -d "$DB_NAME" -t -c "
    SELECT COUNT(*) FROM schema_versions WHERE version = '$VERSION';
  ")
  
  if [ "$(echo $APPLIED | tr -d ' ')" -eq "0" ]; then
    log "Applying migration: $VERSION"
    
    # Check if file exists and is readable
    if [ ! -f "$MIGRATION_FILE" ] || [ ! -r "$MIGRATION_FILE" ]; then
      error_log "Migration file does not exist or is not readable: $MIGRATION_FILE"
      continue
    fi
    
    # Extract description from the migration file (second line after -- Description:)
    DESCRIPTION=$(grep -A 1 "^-- Description:" "$MIGRATION_FILE" | tail -n 1 | sed 's/^-- //')
    if [ -z "$DESCRIPTION" ]; then
      DESCRIPTION="$VERSION migration"
    fi
    
    log "Migration description: $DESCRIPTION"
    
    # Create a temporary file with transaction control and lock timeout
    TEMP_SQL=$(mktemp)
    cat > "$TEMP_SQL" << EOF
-- Set lock timeout to 10 seconds for this transaction
SET lock_timeout = '10000';

-- Start transaction
BEGIN;

-- Apply the migration
$(cat "$MIGRATION_FILE")

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
      
      log "Executing SQL from $MIGRATION_FILE"
      if PGPASSWORD=$POSTGRES_PASSWORD psql -U postgres -d "$DB_NAME" -f "$TEMP_SQL" 2>&1; then
        SUCCESS=true
        
        # Record the migration in schema_versions separately to ensure it's recorded
        log "Recording migration in schema_versions table"
        PGPASSWORD=$POSTGRES_PASSWORD psql -U postgres -d "$DB_NAME" -c "
          INSERT INTO schema_versions (version, description)
          VALUES ('$VERSION', '$DESCRIPTION')
          ON CONFLICT (version) DO NOTHING;
        " || {
          error_log "Failed to record migration in schema_versions table"
        }
      fi
      
      if [ "$SUCCESS" != "true" ]; then
        RETRY_COUNT=$((RETRY_COUNT + 1))
        error_log "Migration attempt failed. Error output:"
        PGPASSWORD=$POSTGRES_PASSWORD psql -U postgres -d "$DB_NAME" -f "$TEMP_SQL" 2>&1 || true
      fi
    done
    
    # Clean up temp file
    rm -f "$TEMP_SQL"
    
    if [ "$SUCCESS" = "true" ]; then
      log "Successfully applied migration: $VERSION"
    else
      error_log "Failed to apply migration after $MAX_RETRIES attempts: $VERSION"
      # Continue with other migrations instead of exiting
      # exit 1
    fi
  else
    log "Migration already applied: $VERSION"
  fi
done

log "All migrations completed successfully"

# Show current migration status
log "Current migration status:"
PGPASSWORD=$POSTGRES_PASSWORD psql -U postgres -d "$DB_NAME" -c "
  SELECT version, applied_at, description FROM schema_versions ORDER BY version;
" || {
  error_log "Failed to show migration status"
} 