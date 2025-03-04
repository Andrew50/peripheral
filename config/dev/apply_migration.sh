#!/bin/bash
set -e

# Check if a migration file was provided
if [ $# -lt 1 ]; then
  echo "Usage: $0 <migration_file>"
  echo "Example: $0 db/rollouts/007_fix_cik_type.sql"
  exit 1
fi

MIGRATION_FILE="$1"

if [ ! -f "$MIGRATION_FILE" ]; then
  echo "Error: Migration file '$MIGRATION_FILE' does not exist"
  exit 1
fi

FILENAME=$(basename "$MIGRATION_FILE")
echo "Applying migration: $FILENAME"

# Ensure the rollouts directory exists in the container
echo "Ensuring rollouts directory exists in the container..."
docker-compose -f docker-compose.dev.yaml exec db mkdir -p /tmp/rollouts

# Copy the migration file to the database container
echo "Copying migration file to the database container..."
docker cp "$MIGRATION_FILE" $(docker-compose -f docker-compose.dev.yaml ps -q db):/tmp/rollouts/

# Run the migration
echo "Running migration..."
docker-compose -f docker-compose.dev.yaml exec db bash -c "POSTGRES_PASSWORD=devpassword bash /app/run_migrations.sh postgres"

# Show migration status
echo "Migration status:"
docker-compose -f docker-compose.dev.yaml exec db bash -c "PGPASSWORD=devpassword psql -U postgres -d postgres -c \"SELECT version, applied_at, description FROM schema_versions ORDER BY version;\""

echo "Migration complete!" 