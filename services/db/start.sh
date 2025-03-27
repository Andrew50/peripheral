#!/bin/bash
set -e

# Function to log messages with timestamps
log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] STARTUP: $1"
}

# Function to log errors
error_log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] STARTUP ERROR: $1" >&2
}

# Ensure rollouts directory exists and has proper permissions
log "Ensuring rollouts directory exists with proper permissions"
mkdir -p /tmp/rollouts
chmod 777 /tmp/rollouts

# Start PostgreSQL using the official entrypoint
log "Starting PostgreSQL..."
docker-entrypoint.sh postgres -c config_file=/etc/postgresql/postgresql.conf &
PG_PID=$!

# Wait for PostgreSQL to start
log "Waiting for PostgreSQL to start..."
until pg_isready -U postgres -h localhost; do
  echo "PostgreSQL is unavailable - sleeping"
  sleep 1
done
log "PostgreSQL is up and running"

# Run migrations initially
log "Running initial migrations..."
/app/migrate.sh postgres || {
  error_log "Failed to run initial migrations"
}

# Start a simple watcher for migrations
log "Starting migrations watcher..."
(
  LAST_HASH="empty"
  while true; do
    sleep 5
    
    # Calculate hash of migration files
    CURRENT_HASH=$(find /tmp/rollouts -type f -name "*.sql" -exec md5sum {} \; 2>/dev/null | sort | md5sum | awk '{print $1}' || echo "empty")
    
    # Run migrations if hash changed
    if [ "$CURRENT_HASH" != "$LAST_HASH" ] && [ "$CURRENT_HASH" != "empty" ]; then
      log "Detected changes in migrations, running migrate.sh"
      /app/migrate.sh postgres
      LAST_HASH="$CURRENT_HASH"
    fi
  done
) &
WATCHER_PID=$!

log "PostgreSQL and migrations watcher are running"

# Trap SIGTERM and SIGINT to properly shutdown both processes
trap 'log "Received shutdown signal"; kill -TERM $PG_PID $WATCHER_PID; wait $PG_PID; wait $WATCHER_PID' TERM INT

# Wait for PostgreSQL to exit
wait $PG_PID
