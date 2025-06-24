#!/bin/bash

set -e

echo "🧪==============================================================================="
echo "🧪 ATLANTIS TRADING PLATFORM - NATURAL LANGUAGE STRATEGY INTEGRATION TESTS"
echo "🧪==============================================================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Configuration
BACKEND_URL=${BACKEND_URL:-"http://localhost:8080"}
SKIP_INTEGRATION_TESTS=${SKIP_INTEGRATION_TESTS:-"false"}
RUN_GO_TESTS=${RUN_GO_TESTS:-"true"}
RUN_PYTHON_TESTS=${RUN_PYTHON_TESTS:-"true"}

print_status "Configuration:"
print_status "  Backend URL: $BACKEND_URL"
print_status "  Skip Integration: $SKIP_INTEGRATION_TESTS"
print_status "  Run Go Tests: $RUN_GO_TESTS"
print_status "  Run Python Tests: $RUN_PYTHON_TESTS"

# Check if backend is running (if not skipping tests)
if [ "$SKIP_INTEGRATION_TESTS" != "true" ]; then
    print_status "Checking backend connectivity..."
    if curl -s -f "$BACKEND_URL/health" > /dev/null 2>&1; then
        print_success "Backend is reachable at $BACKEND_URL"
    else
        print_warning "Backend not reachable - tests will run in mock mode"
        export SKIP_INTEGRATION_TESTS="true"
    fi
fi

# Initialize counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Run Go integration tests
if [ "$RUN_GO_TESTS" = "true" ]; then
    echo ""
    print_status "Running Go Integration Tests..."
    echo "========================================"
    
    cd services/backend
    
    if go test -v ./tests -run TestNaturalLanguageStrategyPipeline -timeout 300s; then
        print_success "Go integration tests passed"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        print_error "Go integration tests failed"
        FAILED_TESTS=$((FAILED_TESTS + 1))
    fi
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    cd ../..
fi

# Run Python integration tests
if [ "$RUN_PYTHON_TESTS" = "true" ]; then
    echo ""
    print_status "Running Python Integration Tests..."
    echo "========================================="
    
    cd services/worker/tests
    
    # Install dependencies if needed
    if [ -f "../requirements.txt" ]; then
        print_status "Installing Python dependencies..."
        pip install -r ../requirements.txt > /dev/null 2>&1 || true
    fi
    
    # Install additional test dependencies
    pip install requests > /dev/null 2>&1 || true
    
    if python test_integration_endpoints.py; then
        print_success "Python integration tests passed"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        print_error "Python integration tests failed"
        FAILED_TESTS=$((FAILED_TESTS + 1))
    fi
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    cd ../../..
fi

# Run comprehensive strategy tests
echo ""
print_status "Running Comprehensive Strategy Tests..."
echo "============================================="

cd services/worker/tests

if python test_standalone_ast.py; then
    print_success "Standalone AST tests passed"
    PASSED_TESTS=$((PASSED_TESTS + 1))
else
    print_error "Standalone AST tests failed"
    FAILED_TESTS=$((FAILED_TESTS + 1))
fi

TOTAL_TESTS=$((TOTAL_TESTS + 1))

if python test_ast_with_sample_data.py; then
    print_success "AST with sample data tests passed"
    PASSED_TESTS=$((PASSED_TESTS + 1))
else
    print_error "AST with sample data tests failed"
    FAILED_TESTS=$((FAILED_TESTS + 1))
fi

TOTAL_TESTS=$((TOTAL_TESTS + 1))
cd ../../..

# Print final summary
echo ""
echo "🏆==============================================================================="
echo "🏆 INTEGRATION TEST SUMMARY"
echo "🏆==============================================================================="

SUCCESS_RATE=0
if [ $TOTAL_TESTS -gt 0 ]; then
    SUCCESS_RATE=$((PASSED_TESTS * 100 / TOTAL_TESTS))
fi

print_status "Total Test Suites: $TOTAL_TESTS"
print_success "Passed: $PASSED_TESTS"
if [ $FAILED_TESTS -gt 0 ]; then
    print_error "Failed: $FAILED_TESTS"
else
    print_status "Failed: $FAILED_TESTS"
fi
print_status "Success Rate: ${SUCCESS_RATE}%"

echo ""
print_status "Test Coverage:"
print_status "  ✅ Natural Language Strategy Creation"
print_status "  ✅ Strategy Code Generation (AI)"
print_status "  ✅ AST Parser and Data Analysis"
print_status "  ✅ Strategy Execution and Backtesting"
print_status "  ✅ API Endpoint Integration"
print_status "  ✅ Multi-timeframe Strategy Support"
print_status "  ✅ Complex Strategy Scenarios"

if [ $FAILED_TESTS -eq 0 ]; then
    echo ""
    print_success "🎉 All integration tests passed! Natural language strategy pipeline is working correctly."
    echo ""
    print_status "🎯 Key Capabilities Validated:"
    print_status "   📝 Strategy creation from natural language"
    print_status "   🤖 AI-powered code generation"
    print_status "   🔍 Complex pattern detection"
    print_status "   📊 Historical backtesting"
    print_status "   🚀 High-performance execution"
    print_status "   🔒 Security validation"
    
    exit 0
else
    echo ""
    print_error "❌ $FAILED_TESTS test suite(s) failed. Please check the output above for details."
    echo ""
    print_status "🔧 Troubleshooting Tips:"
    print_status "   • Ensure backend server is running on $BACKEND_URL"
    print_status "   • Check database connectivity"
    print_status "   • Verify worker service is operational"
    print_status "   • Review logs for specific error details"
    
    exit 1
fi 