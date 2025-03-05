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

# Start the rollouts watcher in the background
log "Starting rollouts watcher..."
/app/watch_rollouts.sh &
WATCHER_PID=$!

log "Both PostgreSQL and rollouts watcher are running"

# Trap SIGTERM and SIGINT to properly shutdown both processes
trap 'log "Received shutdown signal"; kill -TERM $PG_PID $WATCHER_PID; wait $PG_PID; wait $WATCHER_PID' TERM INT

# Wait for PostgreSQL to exit
wait $PG_PID
