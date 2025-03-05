#!/bin/bash

# Script to run golangci-lint with the same config as CI

# Check if golangci-lint is installed
if ! [ -x "$(command -v golangci-lint)" ]; then
  echo 'Error: golangci-lint is not installed.' >&2
  echo 'Install it using: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.64.2' >&2
  exit 1
fi

# Run linting with the same config as CI
echo "Running golangci-lint..."
golangci-lint run --config=.golangci.yml

# Check if fix flag is passed
if [ "$1" == "--fix" ]; then
  echo "Attempting to automatically fix issues..."
  golangci-lint run --config=.golangci.yml --fix
fi

# Print helpful message
echo ""
echo "To fix issues automatically where possible, run: ./lint.sh --fix"
echo "Review backend/LINTING.md for guidance on fixing issues manually." 