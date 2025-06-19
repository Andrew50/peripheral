#!/bin/bash
set -e

echo "🔍 Running Python Worker Lint & Security Checks"
echo "================================================"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to run a command and report status
run_check() {
    local name="$1"
    local cmd="$2"
    local allow_fail="${3:-false}"
    
    echo -e "\n${YELLOW}Running $name...${NC}"
    if eval "$cmd"; then
        echo -e "${GREEN}✅ $name passed${NC}"
        return 0
    else
        if [ "$allow_fail" = "true" ]; then
            echo -e "${YELLOW}⚠️  $name had issues (allowed to fail)${NC}"
            return 0
        else
            echo -e "${RED}❌ $name failed${NC}"
            return 1
        fi
    fi
}

# Ensure we're in the right directory
cd "$(dirname "$0")"

# Install dependencies if they don't exist
if [ ! -d "venv" ] || [ ! -f "venv/bin/activate" ]; then
    echo "🔧 Setting up virtual environment..."
    python3 -m venv venv
    source venv/bin/activate
    pip install --upgrade pip
    pip install -r requirements.txt
else
    source venv/bin/activate
fi

echo "📦 Using Python: $(which python)"
echo "📦 Python version: $(python --version)"

# Run all checks
run_check "Black formatting" "black --check --diff ."
run_check "Import sorting (isort)" "isort --check-only --diff ."
run_check "Code style (flake8)" "flake8 ."
run_check "Static type checking (mypy)" "mypy ." true
run_check "Comprehensive linting (pylint)" "pylint src/ worker.py hot_reload.py" true
run_check "Security linting (bandit)" "bandit -r . -f txt"
run_check "Dependency vulnerabilities (safety)" "safety check"
run_check "Tests with coverage" "pytest --cov=src --cov-report=term-missing"

echo -e "\n${GREEN}🎉 All checks completed!${NC}" 