#!/bin/bash

# Get the directory of the script itself
SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

# Change to the script's directory to ensure commands run in the correct context
cd "$SCRIPT_DIR" || exit 1

echo "Changed working directory to: $(pwd)"
echo ""

echo "Running go mod tidy & verify..."
go mod tidy
if [ $? -ne 0 ]; then echo "Error: go mod tidy failed"; exit 1; fi
go mod verify
if [ $? -ne 0 ]; then echo "Error: go mod verify failed"; exit 1; fi

echo ""
echo "Running go vet..."
go vet ./...
if [ $? -ne 0 ]; then echo "Error: go vet failed"; exit 1; fi

echo ""
echo "Running staticcheck..."
staticcheck -tags=all ./...
if [ $? -ne 0 ]; then echo "Error: staticcheck failed"; exit 1; fi

echo ""
echo "Running gosec..."
# Assuming gosec is installed in ~/go/bin/ or in PATH. Using original script's path.
~/go/bin/gosec -quiet -tags=all ./...
if [ $? -ne 0 ]; then echo "Error: gosec failed"; exit 1; fi

echo ""
echo "Running golangci-lint..."
# Using --config because .golangci.yml exists in this directory
golangci-lint run --config=.golangci.yml ./...
if [ $? -ne 0 ]; then echo "Error: golangci-lint failed"; exit 1; fi

echo ""
echo "Running Go Build..."
go build -v -tags=all ./...
if [ $? -ne 0 ]; then echo "Error: go build failed"; exit 1; fi

echo ""
echo "Running Go Tests (with race detector)..."
go test -race -count=1 -tags=all ./...
if [ $? -ne 0 ]; then echo "Error: go test failed"; exit 1; fi

echo ""
echo "All checks complete." 