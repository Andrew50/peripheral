#!/bin/bash
set -e

DB_NAME="${1:-postgres}"
MIGRATIONS_DIR="/migrations"

# Function to log messages with timestamps
log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] MIGRATION: $1"
}

# Function to log errors
error_log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] MIGRATION ERROR: $1" >&2
}

# Function to extract numeric version from filename
extract_version() {
  local filename="$1"
  # Remove the .sql extension
  local version_str=$(echo "$filename" | sed 's/\.sql$//')
  # If it's a purely numeric filename, return as is
  if [[ "$version_str" =~ ^[0-9]+$ ]]; then
    echo "$version_str"
  else
    # Otherwise, use the full filename without extension as the version
    echo "$version_str"
  fi
}

# Ensure the schema_versions table exists
log "Ensuring schema_versions table exists..."
PGPASSWORD=$POSTGRES_PASSWORD psql -U postgres -d "$DB_NAME" -c "
CREATE TABLE IF NOT EXISTS schema_versions (
    version NUMERIC PRIMARY KEY,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    description TEXT
);" || {
  error_log "Failed to create schema_versions table"
  exit 1
}

# Get list of migration files sorted by version
log "Looking for migration files in $MIGRATIONS_DIR"
MIGRATION_FILES=$(find "$MIGRATIONS_DIR" -name "*.sql" | sort -V)

if [ -z "$MIGRATION_FILES" ]; then
  log "No migration files found"
  exit 0
fi

log "Found migration files: $(echo "$MIGRATION_FILES" | wc -l)"

# If this is first run after init.sql, we need to mark all migrations as applied
# Check if schema_versions is empty
SCHEMA_VERSION_COUNT=$(PGPASSWORD=$POSTGRES_PASSWORD psql -U postgres -d "$DB_NAME" -t -c "
  SELECT COUNT(*) FROM schema_versions;
")
SCHEMA_VERSION_COUNT=$(echo $SCHEMA_VERSION_COUNT | tr -d ' ')

if [ "$SCHEMA_VERSION_COUNT" -eq "0" ]; then
  log "No schema versions found. This appears to be first run after init.sql."
  log "Marking all migrations as applied..."
  
  # Find highest migration number from file names
  HIGHEST_MIGRATION_FILE=$(basename "$(echo "$MIGRATION_FILES" | tail -n 1)")
  log "Highest migration file: $HIGHEST_MIGRATION_FILE"
  
  # Mark all migrations as applied
  for MIGRATION_FILE in $MIGRATION_FILES; do
    FILENAME=$(basename "$MIGRATION_FILE")
    VERSION=$(extract_version "$FILENAME")
    
    # Extract description from the migration file
    DESCRIPTION=$(grep -A 1 "^-- Description:" "$MIGRATION_FILE" | tail -n 1 | sed 's/^-- //')
    if [ -z "$DESCRIPTION" ]; then
      DESCRIPTION="Migration $VERSION"
    fi
    
    log "Marking migration as applied: $VERSION (from $FILENAME)"
    PGPASSWORD=$POSTGRES_PASSWORD psql -U postgres -d "$DB_NAME" -c "
      INSERT INTO schema_versions (version, description)
      VALUES ($VERSION, '$DESCRIPTION')
      ON CONFLICT (version) DO NOTHING;
    " || {
      error_log "Failed to mark migration as applied: $VERSION"
    }
  done
  
  log "All migrations marked as applied"
  exit 0
fi

# Process each migration file (only those that haven't been applied)
for MIGRATION_FILE in $MIGRATION_FILES; do
  FILENAME=$(basename "$MIGRATION_FILE")
  VERSION=$(extract_version "$FILENAME")
  
  log "Checking migration: $VERSION (from $FILENAME)"
  
  # Check if this migration has already been applied
  APPLIED=$(PGPASSWORD=$POSTGRES_PASSWORD psql -U postgres -d "$DB_NAME" -t -c "
    SELECT COUNT(*) FROM schema_versions WHERE version = $VERSION;
  ")
  
  if [ "$(echo $APPLIED | tr -d ' ')" -eq "0" ]; then
    log "Applying migration: $VERSION (from $FILENAME)"
    
    # Extract description from the migration file
    DESCRIPTION=$(grep -A 1 "^-- Description:" "$MIGRATION_FILE" | tail -n 1 | sed 's/^-- //')
    if [ -z "$DESCRIPTION" ]; then
      DESCRIPTION="Migration $VERSION"
    fi
    
    log "Migration description: $DESCRIPTION"
    
    # Apply the migration
    if PGPASSWORD=$POSTGRES_PASSWORD psql -U postgres -d "$DB_NAME" -f "$MIGRATION_FILE"; then
      log "Successfully applied migration: $VERSION"
      
      # Record the migration in schema_versions
      PGPASSWORD=$POSTGRES_PASSWORD psql -U postgres -d "$DB_NAME" -c "
        INSERT INTO schema_versions (version, description)
        VALUES ($VERSION, '$DESCRIPTION')
        ON CONFLICT (version) DO NOTHING;
      " || {
        error_log "Failed to record migration in schema_versions table"
      }
    else
      error_log "Failed to apply migration: $VERSION"
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