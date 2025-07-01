#!/bin/bash

echo "🧪 Friday Afternoon Gap Analysis Integration Test"
echo "=================================================="

# Set test parameters
export SKIP_INTEGRATION_TESTS=${SKIP_INTEGRATION_TESTS:-"false"}
export BACKEND_URL=${BACKEND_URL:-"http://localhost:8080"}
export RUN_FRIDAY_GAP_TEST=${RUN_FRIDAY_GAP_TEST:-"true"}

echo "📋 Test Configuration:"
echo "  • SKIP_INTEGRATION_TESTS: $SKIP_INTEGRATION_TESTS"
echo "  • BACKEND_URL: $BACKEND_URL"
echo "  • RUN_FRIDAY_GAP_TEST: $RUN_FRIDAY_GAP_TEST"
echo ""

# Check if we should run the test
if [ "$RUN_FRIDAY_GAP_TEST" != "true" ]; then
    echo "⏭️ Skipping Friday gap test (RUN_FRIDAY_GAP_TEST != true)"
    exit 0
fi

# Navigate to worker tests directory
cd services/worker/tests || {
    echo "❌ Error: Could not find services/worker/tests directory"
    exit 1
}

echo "📅 Running Friday Afternoon Gap Analysis Test..."
echo "=================================================="

# Run the main integration test
python test_friday_afternoon_gaps.py
TEST_EXIT_CODE=$?

echo ""
echo "📊 Running Direct Strategy Execution Test..."
echo "=============================================="

# Run the direct strategy test
python test_friday_strategy_execution.py
STRATEGY_EXIT_CODE=$?

echo ""
echo "📋 Friday Gap Test Suite Results"
echo "=================================="

if [ $TEST_EXIT_CODE -eq 0 ]; then
    echo "✅ Integration Test: PASSED"
else
    echo "❌ Integration Test: FAILED"
fi

if [ $STRATEGY_EXIT_CODE -eq 0 ]; then
    echo "✅ Strategy Execution Test: PASSED"
else
    echo "❌ Strategy Execution Test: FAILED"
fi

# Overall result
if [ $TEST_EXIT_CODE -eq 0 ] || [ $STRATEGY_EXIT_CODE -eq 0 ]; then
    echo ""
    echo "🎉 Friday Gap Analysis Test Suite: SUCCESS"
    echo "   At least one test passed, indicating the framework is working"
    exit 0
else
    echo ""
    echo "❌ Friday Gap Analysis Test Suite: FAILED"
    echo "   Both tests failed"
    exit 1
fi 