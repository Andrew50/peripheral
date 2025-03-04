#!/bin/bash

# Function to log messages with timestamps
log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

# Function to log errors
error_log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: $1" >&2
}

# Make the script executable
chmod +x ./check_securities.sh

# Check if Docker Compose is running
log "Checking if Docker Compose environment is running..."
if ! docker ps | grep -q "dev-db-1"; then
  error_log "Docker Compose environment is not running. Please run ./dev.bash first."
  exit 1
fi

# Check if database is ready
log "Checking if database is ready..."
if ! docker exec dev-db-1 pg_isready -U postgres; then
  error_log "Database is not ready. Please wait for it to initialize."
  exit 1
fi

# Check securities that need updating
log "Checking securities that need logo/icon updates..."
docker exec dev-db-1 psql -U postgres -c "SELECT COUNT(*) FROM securities WHERE maxDate IS NULL AND (logo IS NULL OR icon IS NULL);"

# Check securities with logos
log "Checking securities with logos..."
docker exec dev-db-1 psql -U postgres -c "SELECT COUNT(*) FROM securities WHERE logo IS NOT NULL;"

# Check securities with icons
log "Checking securities with icons..."
docker exec dev-db-1 psql -U postgres -c "SELECT COUNT(*) FROM securities WHERE icon IS NOT NULL;"

# Check total active securities
log "Checking total active securities..."
docker exec dev-db-1 psql -U postgres -c "SELECT COUNT(*) FROM securities WHERE maxDate IS NULL;"

log "Done!" 