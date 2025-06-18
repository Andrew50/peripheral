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

# Ensure migrations directory exists
log "Ensuring migrations directory exists..."
mkdir -p ../../services/db/migrations
chmod 777 ../../services/db/migrations

# Start Docker Compose
log "Starting Docker Compose environment..."
docker-compose -f docker-compose.yaml -p dev up -d --build --scale worker=5 

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

# Wait longer to ensure the database is fully initialized and stable
log "Waiting for database to fully initialize and stabilize..."
sleep 15

# Check if the database is still running before proceeding
if ! docker exec dev-db-1 pg_isready -U postgres; then
  error_log "Database was ready but is no longer responding. It may have crashed."
  log "Checking database logs for errors..."
  docker logs dev-db-1 | tail -n 30
  exit 1
fi

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

# Ensure all migration files are copied to the container
log "Copying migration files to the database container..."
for MIGRATION_FILE in ../../services/db/migrations/*.sql; do
  if [ -f "$MIGRATION_FILE" ]; then
    FILENAME=$(basename "$MIGRATION_FILE")
    log "Copying $FILENAME to container..."
    docker cp "$MIGRATION_FILE" dev-db-1:/tmp/migrations/
  fi
done

# Trigger migrations manually to ensure they run
log "Triggering migrations manually..."
docker exec dev-db-1 bash -c "/app/migrate.sh postgres"

# Wait for migrations to complete
log "Waiting for migrations to complete..."
sleep 5

# Show migration status after running migrations
log "Migration status after running migrations:"
docker exec -e POSTGRES_PASSWORD=devpassword dev-db-1 psql -U postgres -d postgres -c "
  SELECT version, applied_at, description FROM schema_versions ORDER BY version;
"

log "Development environment is ready!"
log "To view logs: docker-compose -f docker-compose.yaml logs -f"
log "To stop: docker-compose -f docker-compose.yaml down" 

# Enter the log stream automatically
log "Entering log stream. Press Ctrl+C to exit..."
docker-compose -f docker-compose.yaml -p dev logs -f 