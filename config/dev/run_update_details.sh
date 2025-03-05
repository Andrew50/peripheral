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
chmod +x ./run_update_details.sh

# Check if Docker Compose is running
log "Checking if Docker Compose environment is running..."
if ! docker ps | grep -q "dev-backend-1"; then
  error_log "Docker Compose environment is not running. Please run ./dev.bash first."
  exit 1
fi

# Check if database is ready
log "Checking if database is ready..."
if ! docker ps | grep -q "dev-db-1" || ! docker exec dev-db-1 pg_isready -U postgres; then
  error_log "Database is not ready. Please wait for it to initialize."
  exit 1
fi

# Run the UpdateSecurityDetails job
log "Running UpdateSecurityDetails job..."
docker exec dev-backend-1 go run /app/cmd/jobctl/main.go run UpdateSecurityDetails

# Check the job status
log "Checking job status..."
docker exec dev-backend-1 go run /app/cmd/jobctl/main.go status

log "Done!" 