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
# Try Go's built-in cleanup first
go clean -cache -modcache -testcache -fuzzcache 2>/dev/null || true

# Fallback to manual cleanup
if [ -d ".go/pkg/mod" ]; then
    echo "Manual cleanup of .go/pkg/mod..."
    # Make all files writable before deletion to avoid permission errors
    chmod -R u+w .go/pkg/mod 2>/dev/null || true
    rm -rf .go/pkg/mod 2>/dev/null || true
fi

# Also clean Go build cache
if [ -d ".go/cache" ]; then
    echo "Manual cleanup of .go/cache..."
    chmod -R u+w .go/cache 2>/dev/null || true
    rm -rf .go/cache 2>/dev/null || true
fi

echo "Cleanup complete."
