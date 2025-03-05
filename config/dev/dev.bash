#!/bin/bash
set -e

# Function to log messages with timestamps
log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

# Function to log errors
error_log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: $1" >&2
}

# Make the script executable
chmod +x ./dev.bash

# Ensure rollouts directory exists
log "Ensuring rollouts directory exists..."
mkdir -p ../../services/db/rollouts
chmod 777 ../../services/db/rollouts

# Start Docker Compose
log "Starting Docker Compose environment..."
docker-compose -f docker-compose.dev.yaml -p dev up -d --build --scale worker=5 

# Wait for the database to be ready
log "Waiting for database to be ready..."
MAX_RETRIES=30
RETRY_COUNT=0
DB_READY=false

while [ $RETRY_COUNT -lt $MAX_RETRIES ] && [ "$DB_READY" != "true" ]; do
  if [ $RETRY_COUNT -gt 0 ]; then
    log "Retrying database connection ($RETRY_COUNT of $MAX_RETRIES)..."
    sleep 2
  fi
  
  if docker exec dev-db-1 pg_isready -U postgres; then
    DB_READY=true
    log "Database is ready!"
  else
    RETRY_COUNT=$((RETRY_COUNT + 1))
  fi
done

if [ "$DB_READY" != "true" ]; then
  error_log "Database failed to become ready after $MAX_RETRIES attempts"
  exit 1
fi

# Wait a bit more to ensure the database is fully initialized
log "Waiting a few more seconds for database initialization..."
sleep 5

# Check if the schema_versions table exists
log "Checking if schema_versions table exists..."
TABLE_EXISTS=$(docker exec -e POSTGRES_PASSWORD=devpassword dev-db-1 psql -U postgres -d postgres -t -c "
  SELECT EXISTS (
    SELECT FROM information_schema.tables 
    WHERE table_schema = 'public' 
    AND table_name = 'schema_versions'
  );
")

if [[ $(echo $TABLE_EXISTS | tr -d ' ') == "f" ]]; then
  log "schema_versions table does not exist. It will be created during migration."
else
  log "schema_versions table already exists."
  
  # Show current migration status
  log "Current migration status:"
  docker exec -e POSTGRES_PASSWORD=devpassword dev-db-1 psql -U postgres -d postgres -c "
    SELECT version, applied_at, description FROM schema_versions ORDER BY version;
  "
fi

# Ensure all rollout files are copied to the container
log "Copying rollout files to the database container..."
for ROLLOUT_FILE in ../../services/db/rollouts/*.sql; do
  if [ -f "$ROLLOUT_FILE" ]; then
    FILENAME=$(basename "$ROLLOUT_FILE")
    log "Copying $FILENAME to container..."
    docker cp "$ROLLOUT_FILE" dev-db-1:/tmp/rollouts/
  fi
done

# Trigger migrations manually to ensure they run
log "Triggering migrations manually..."
docker exec dev-db-1 bash -c "/app/run_migrations.sh postgres"

# Wait for migrations to complete
log "Waiting for migrations to complete..."
sleep 5

# Show migration status after running migrations
log "Migration status after running migrations:"
docker exec -e POSTGRES_PASSWORD=devpassword dev-db-1 psql -U postgres -d postgres -c "
  SELECT version, applied_at, description FROM schema_versions ORDER BY version;
"

log "Development environment is ready!"
log "To view logs: docker-compose -f docker-compose.dev.yaml logs -f"
log "To stop: docker-compose -f docker-compose.dev.yaml down" 

# Enter the log stream automatically
log "Entering log stream. Press Ctrl+C to exit..."
docker-compose -f docker-compose.dev.yaml -p dev logs -f 