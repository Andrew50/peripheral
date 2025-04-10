#!/usr/bin/env bash
set -Eeuo pipefail

echo "Performing cleanup..."

# Remove dangling/unused images older than 24h (adjust filters as you see fit)
echo "Pruning old Docker images..."
docker image prune -af --filter "until=24h"

# Prune system
echo "Pruning Docker system volumes..."
docker system prune -af --volumes

echo "Cleanup complete."
