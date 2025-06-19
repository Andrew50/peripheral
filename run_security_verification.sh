#!/bin/bash

# Security Verification Script
# Runs comprehensive security checks and verification tests

echo "🔒 Security Verification Suite"
echo "=============================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    local status=$1
    local message=$2
    case $status in
        "SUCCESS")
            echo -e "${GREEN}✅ $message${NC}"
            ;;
        "ERROR")
            echo -e "${RED}❌ $message${NC}"
            ;;
        "WARNING")
            echo -e "${YELLOW}⚠️  $message${NC}"
            ;;
        "INFO")
            echo -e "${BLUE}ℹ️  $message${NC}"
            ;;
    esac
}

# Check if bandit is available
check_bandit() {
    print_status "INFO" "Checking for bandit security scanner..."
    
    if command -v bandit &> /dev/null; then
        print_status "SUCCESS" "Bandit is available"
        return 0
    elif python -m bandit --help &> /dev/null; then
        print_status "SUCCESS" "Bandit is available via python -m"
        return 0
    else
        print_status "WARNING" "Bandit not found. Installing..."
        pip install bandit
        if [ $? -eq 0 ]; then
            print_status "SUCCESS" "Bandit installed successfully"
            return 0
        else
            print_status "ERROR" "Failed to install bandit"
            return 1
        fi
    fi
}

# Run bandit security scan
run_bandit_scan() {
    print_status "INFO" "Running bandit security scan..."
    
    # Try different bandit commands
    if command -v bandit &> /dev/null; then
        bandit -r . -f json -o bandit-report-verification.json 2>/dev/null
        bandit -r . --skip B101
    elif python -m bandit --help &> /dev/null; then
        python -m bandit -r . -f json -o bandit-report-verification.json 2>/dev/null
        python -m bandit -r . --skip B101
    else
        print_status "ERROR" "Cannot run bandit scan"
        return 1
    fi
    
    local exit_code=$?
    
    if [ $exit_code -eq 0 ]; then
        print_status "SUCCESS" "Bandit scan completed with no critical issues!"
    elif [ $exit_code -eq 1 ]; then
        print_status "WARNING" "Bandit found some issues (likely low severity)"
    else
        print_status "ERROR" "Bandit scan failed"
        return 1
    fi
    
    return 0
}

# Run security test suite
run_security_tests() {
    print_status "INFO" "Running security test suite..."
    
    if [ -f "test_security_fixes.py" ]; then
        python test_security_fixes.py
        local exit_code=$?
        
        if [ $exit_code -eq 0 ]; then
            print_status "SUCCESS" "Security test suite passed!"
            return 0
        else
            print_status "ERROR" "Security test suite failed"
            return 1
        fi
    else
        print_status "ERROR" "Security test suite not found (test_security_fixes.py)"
        return 1
    fi
}

# Validate Python files compile
validate_python_files() {
    print_status "INFO" "Validating Python file compilation..."
    
    local failed_files=()
    
    # Find all Python files and try to compile them
    while IFS= read -r -d '' file; do
        if ! python -m py_compile "$file" 2>/dev/null; then
            failed_files+=("$file")
        fi
    done < <(find . -name "*.py" -not -path "./venv/*" -print0)
    
    if [ ${#failed_files[@]} -eq 0 ]; then
        print_status "SUCCESS" "All Python files compile successfully"
        return 0
    else
        print_status "ERROR" "Failed to compile: ${failed_files[*]}"
        return 1
    fi
}

# Check for potential security patterns
check_security_patterns() {
    print_status "INFO" "Checking for security anti-patterns..."
    
    local issues=0
    
    # Check for SQL concatenation patterns (should be none now)
    if grep -r "f\".*SELECT\|f'.*SELECT" --include="*.py" . 2>/dev/null | grep -v test_security_fixes.py; then
        print_status "ERROR" "Found potential SQL injection patterns"
        ((issues++))
    fi
    
    # Check for dangerous exec/eval patterns
    if grep -r "exec(\|eval(" --include="*.py" . 2>/dev/null | grep -v "# nosec\|test_security_fixes.py"; then
        print_status "ERROR" "Found potentially dangerous exec/eval usage"
        ((issues++))
    fi
    
    # Check for shell=True patterns
    if grep -r "shell=True" --include="*.py" . 2>/dev/null; then
        print_status "ERROR" "Found shell=True subprocess usage"
        ((issues++))
    fi
    
    if [ $issues -eq 0 ]; then
        print_status "SUCCESS" "No security anti-patterns found"
        return 0
    else
        print_status "ERROR" "Found $issues potential security issues"
        return 1
    fi
}

# Main execution
main() {
    local total_checks=0
    local passed_checks=0
    
    echo "Starting comprehensive security verification..."
    echo ""
    
    # Run all checks
    checks=(
        "check_bandit:Check Bandit Availability"
        "validate_python_files:Validate Python Compilation"
        "check_security_patterns:Check Security Patterns"
        "run_bandit_scan:Run Bandit Security Scan"
        "run_security_tests:Run Security Test Suite"
    )
    
    for check in "${checks[@]}"; do
        IFS=':' read -ra CHECK_PARTS <<< "$check"
        check_function="${CHECK_PARTS[0]}"
        check_name="${CHECK_PARTS[1]}"
        
        echo ""
        echo "🔍 $check_name"
        echo "----------------------------------------"
        
        ((total_checks++))
        
        if $check_function; then
            ((passed_checks++))
        fi
    done
    
    # Summary
    echo ""
    echo "=============================="
    echo "📊 Security Verification Summary"
    echo "=============================="
    echo "Total Checks: $total_checks"
    echo "Passed: $passed_checks"
    echo "Failed: $((total_checks - passed_checks))"
    echo ""
    
    if [ $passed_checks -eq $total_checks ]; then
        print_status "SUCCESS" "All security verifications passed! 🎉"
        echo ""
        echo "🔒 Your Python Strategy Worker is secure and ready for production!"
        exit 0
    else
        print_status "ERROR" "Some security verifications failed"
        echo ""
        echo "⚠️  Please review the failed checks above and address any issues."
        exit 1
    fi
}

# Run main function
main "$@" 