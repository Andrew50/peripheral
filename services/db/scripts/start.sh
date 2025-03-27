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

# Run migrations once on startup
log "Running migrations..."
# Use the correct path for the migrate.sh script based on the environment
if [ -f "/app/migrate.sh" ]; then
  # Development environment path
  /app/migrate.sh postgres || {
    error_log "Failed to run migrations"
  }
elif [ -f "/usr/local/bin/migrate.sh" ]; then
  # Production environment path
  /usr/local/bin/migrate.sh postgres || {
    error_log "Failed to run migrations"
  }
else
  error_log "migrate.sh not found in expected locations"
  exit 1
fi

log "PostgreSQL is running with migrations applied"

# Trap SIGTERM and SIGINT to properly shutdown
trap 'log "Received shutdown signal"; kill -TERM $PG_PID; wait $PG_PID' TERM INT

# Wait for PostgreSQL to exit
wait $PG_PID
