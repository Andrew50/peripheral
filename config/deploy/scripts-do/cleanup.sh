#!/usr/bin/env bash
set -Eeuo pipefail

echo "Performing cleanup..."

echo "Pruning old Docker images..."
docker image prune -af

echo "Pruning Docker system volumes..."
docker system prune -af --volumes

echo "Removing Temporary config files..."
rm -rf config/deploy/tmp

echo "Cleaning Go module cache with proper permissions..."
if [ -d ".go/pkg/mod" ]; then
    # Make all files writable before deletion to avoid permission errors
    chmod -R u+w .go/pkg/mod || true
    rm -rf .go/pkg/mod || true
fi

# Also clean Go build cache
if [ -d ".go/cache" ]; then
    chmod -R u+w .go/cache || true
    rm -rf .go/cache || true
fi

echo "Cleanup complete."
