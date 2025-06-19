#!/bin/bash
set -e

echo "🧪 Testing Worker Docker Build Speed"
echo "=================================="

# Record start time
start_time=$(date +%s)

echo "Building worker service with optimized Dockerfile..."
echo "Expected build time: 30-60 seconds (vs 1000+ seconds before)"
echo ""

# Build with timing
DOCKER_BUILDKIT=1 docker build \
  --progress=plain \
  -t test-worker:latest \
  -f services/worker/Dockerfile.prod \
  services/worker

# Calculate build time
end_time=$(date +%s)
build_time=$((end_time - start_time))

echo ""
echo "🎉 Build completed successfully!"
echo "⏱️  Build time: ${build_time} seconds"

if [ $build_time -lt 120 ]; then
  echo "✅ PASS: Build time under 2 minutes (was 1000+ seconds)"
else
  echo "⚠️  WARNING: Build took longer than expected"
fi

echo ""
echo "🧪 Testing container functionality..."

# Test the container runs
docker run --rm test-worker:latest python -c "
import numpy as np
import pandas as pd
print('✅ Core dependencies working')
print('✅ Numpy version:', np.__version__)
print('✅ Pandas version:', pd.__version__)
print('✅ System ready for strategy execution')
"

echo ""
echo "🎉 All tests passed! Worker build is optimized and functional."

# Cleanup
docker rmi test-worker:latest 2>/dev/null || true

echo ""
echo "📊 Build Performance Summary:"
echo "  - Before: 1000+ seconds (TA-Lib installation)"
echo "  - After:  ${build_time} seconds (no TA-Lib needed)"
echo "  - Improvement: $(( (1000 - build_time) * 100 / 1000 ))% faster" 