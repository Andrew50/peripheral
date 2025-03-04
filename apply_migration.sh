#!/bin/bash
set -e

echo "Copying migration file to the database container..."
docker cp db/rollouts/004_fix_market_cap_type.sql $(docker-compose -f docker-compose.dev.yaml ps -q db):/tmp/rollouts/

echo "Running migration..."
docker-compose -f docker-compose.dev.yaml exec db bash -c "POSTGRES_PASSWORD=devpassword bash /app/run_migrations.sh postgres"

echo "Migration complete!" 