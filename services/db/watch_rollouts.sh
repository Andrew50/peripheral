#!/bin/bash
set -e

ROLLOUTS_DIR="/tmp/rollouts"
DB_NAME="postgres"
LAST_HASH_FILE="/tmp/last_rollouts_hash"

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] WATCHER: $1"
}

error_log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] WATCHER ERROR: $1" >&2
}

# Function to calculate a hash of all files in the rollouts directory
calculate_hash() {
  if [ -d "$ROLLOUTS_DIR" ] && [ "$(ls -A $ROLLOUTS_DIR 2>/dev/null)" ]; then
    find "$ROLLOUTS_DIR" -type f -name "*.sql" -exec md5sum {} \; | sort | md5sum | awk '{print $1}'
  else
    echo "empty"
  fi
}

# Ensure rollouts directory exists
mkdir -p "$ROLLOUTS_DIR"
chmod 777 "$ROLLOUTS_DIR"

# Initialize with current hash
CURRENT_HASH=$(calculate_hash)
echo "$CURRENT_HASH" > "$LAST_HASH_FILE"
log "Initial rollouts hash: $CURRENT_HASH"

# Run migrations on startup
log "Running initial migrations..."
/app/run_migrations.sh "$DB_NAME" || {
  error_log "Failed to run initial migrations"
}

# Watch for changes
log "Watching for changes in $ROLLOUTS_DIR"
while true; do
  sleep 5
  
  # Skip if PostgreSQL is not ready
  pg_isready -U postgres -h localhost > /dev/null 2>&1 || {
    log "PostgreSQL is not ready, skipping check"
    continue
  }
  
  # Check if directory exists and has files
  if [ ! -d "$ROLLOUTS_DIR" ]; then
    log "Rollouts directory does not exist, creating it"
    mkdir -p "$ROLLOUTS_DIR"
    chmod 777 "$ROLLOUTS_DIR"
    continue
  fi
  
  # Calculate new hash
  NEW_HASH=$(calculate_hash)
  
  # If hash changed, run migrations
  if [ "$NEW_HASH" != "$CURRENT_HASH" ]; then
    log "Detected changes in rollouts directory"
    log "Previous hash: $CURRENT_HASH"
    log "New hash: $NEW_HASH"
    
    # List the SQL files for debugging
    log "Current SQL files in rollouts directory:"
    find "$ROLLOUTS_DIR" -type f -name "*.sql" -exec basename {} \; | sort || log "No SQL files found"
    
    # Run migrations
    log "Running migrations..."
    /app/run_migrations.sh "$DB_NAME" || {
      error_log "Failed to run migrations after detecting changes"
      # Continue watching even if migrations fail
    }
    
    # Update current hash
    CURRENT_HASH="$NEW_HASH"
    echo "$CURRENT_HASH" > "$LAST_HASH_FILE"
    log "Updated rollouts hash: $CURRENT_HASH"
  fi
done 