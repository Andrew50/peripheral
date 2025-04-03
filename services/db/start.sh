#!/bin/bash
set -e

# Function to log messages with timestamps
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] START-SCRIPT: $1"
}

# Function to handle errors
error() {
    log "ERROR: $1"
    exit 1
}

# Ensure proper data directory
export PGDATA=/var/lib/postgresql/data

# Ensure rollouts directory exists with proper permissions
log "Setting up rollouts directory"
mkdir -p /tmp/rollouts || error "Failed to create rollouts directory"
chmod 777 /tmp/rollouts || error "Failed to set permissions on rollouts directory"

# Start the watcher in the background
log "Starting rollouts watcher"
/app/watch_rollouts.sh &
WATCHER_PID=$!

# Trap signals to properly shutdown the watcher
trap 'log "Received shutdown signal"; kill -TERM $WATCHER_PID; wait $WATCHER_PID' TERM INT

# Check if database already exists
if [ -f "$PGDATA/PG_VERSION" ]; then
    log "Database already initialized, skipping initialization"
    
    # Set environment variables to skip initialization
    export POSTGRES_INITDB_SKIP=true
    export POSTGRES_INITDB_SKIP_PGDATA=true
    
    # Ensure permissions are correct
    chown -R postgres:postgres "$PGDATA" || error "Failed to set ownership of data directory"
    chmod 700 "$PGDATA" || error "Failed to set permissions on data directory"
    
    # Start PostgreSQL directly
    log "Starting PostgreSQL with existing data directory"
    exec gosu postgres postgres -c config_file=/etc/postgresql/postgresql.conf
else
    log "No existing database found, initializing new database"
    # Run the normal entrypoint script to handle initialization
    exec /usr/local/bin/docker-entrypoint.sh postgres -c config_file=/etc/postgresql/postgresql.conf
fi
