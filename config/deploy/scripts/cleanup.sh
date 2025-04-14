#!/usr/bin/env bash
set -Eeuo pipefail

echo "Performing cleanup..."

echo "Pruning old Docker images..."
docker image prune -af

echo "Pruning Docker system volumes..."
docker system prune -af --volumes

echo "Removing Temporary config files..."
rm -rf config/deploy/tmp

echo "Cleanup complete."
