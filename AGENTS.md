# Agent Development and Testing Guide

This document provides instructions on how to set up, run, and test the project.

## Prerequisites

Before you begin, ensure you have the following installed:
- [Go](https://golang.org/doc/install) (see `go.mod` in `services/backend` for version, e.g., 1.24)
- [Node.js and npm](https://nodejs.org/) (see `services/frontend/Dockerfile.dev` for version, e.g., Node 18)
- [Docker and Docker Compose](https://www.docker.com/get-started)
- Go development tools (see "Running Backend Checks and Verifications" section for details)

## Local Development

You can run the project's services locally either directly on your host machine or using Docker Compose for a more integrated environment.

### 1. Running Services Directly on Host

This method is useful for focused development on individual services.

#### Backend (Go)

1.  **Navigate to the backend directory:**
    ```bash
    cd services/backend
    ```
2.  **Install dependencies:**
    If this is your first time or dependencies have changed:
    ```bash
    go mod tidy
    ```
3.  **Run the backend application:**
    The command to run the backend depends on its entry point. Assuming your main application is in a `cmd/server` or similar directory:
    ```bash
    # Example: if your main package is in services/backend/cmd/server/
    go run ./cmd/server/main.go
    ```
    *Note: The `services/backend/Dockerfile.dev` uses `air` for live reloading within Docker. For direct local execution, `go run` targeting your main application file is standard.*
    The backend typically requires a database and other services. Ensure these are running and accessible. Connection details (e.g., for the database) can often be found in `config/dev/docker-compose.yaml` or your application's configuration files.

#### Frontend (Node.js/npm)

1.  **Navigate to the frontend directory:**
    ```bash
    cd services/frontend
    ```
2.  **Install dependencies:**
    ```bash
    npm install
    ```
3.  **Run the development server:**
    As specified in `services/frontend/Dockerfile.dev`:
    ```bash
    npm run dev -- --host 0.0.0.0
    ```
    This usually starts a development server (e.g., Vite) on `http://localhost:5173` (or the configured port). The `--host 0.0.0.0` makes it accessible from outside the container if you were running this command inside one, or from other devices on your network if run directly on host.

### 2. Running Services with Docker Compose

Docker Compose allows you to run all project services (backend, frontend, database, cache, etc.) together in isolated containers. This is defined by `Dockerfile.dev` files for each service and orchestrated by `config/dev/docker-compose.yaml`. This is often the recommended way for a full development setup.

1.  **Navigate to the Docker Compose configuration directory:**
    ```bash
    cd config/dev
    ```
2.  **Build and start all services:**
    ```bash
    docker-compose up --build
    ```
    To run in detached mode (in the background):
    ```bash
    docker-compose up --build -d
    ```
3.  **View logs for services:**
    ```bash
    docker-compose logs -f <service_name>  # e.g., docker-compose logs -f backend db frontend
    ```
    Service names (`backend`, `db`, `frontend`, `cache`) are defined in `config/dev/docker-compose.yaml`.
4.  **Stop all services:**
    ```bash
    docker-compose down
    ```
    To remove volumes as well (useful for a clean restart, e.g., database data):
    ```bash
    docker-compose down -v
    ```

*   **Dockerfiles**:
    *   `services/backend/Dockerfile.dev`: Defines the Go backend development environment, using `air` for live reloading.
    *   `services/frontend/Dockerfile.dev`: Defines the Node.js frontend development environment.
    *   `services/db/Dockerfile.dev`: Sets up the TimescaleDB/PostgreSQL database.
    *   `services/cache/Dockerfile`: Sets up the Redis cache.
*   **`config/dev/docker-compose.yaml`**: This file is key to understanding the overall application architecture in development, including service definitions, ports, volumes, networks, and environment variables.

## Running Backend Checks and Verifications

The `services/backend/run_checks.sh` script provides a comprehensive way to format, lint, analyze, build, and test the Go backend.

1.  **Navigate to the backend directory:**
    ```bash
    cd services/backend
    ```
2.  **Ensure the script is executable:**
    ```bash
    chmod +x run_checks.sh
    ```
3.  **Execute the script:**
    ```bash
    ./run_checks.sh
    ```

The script performs the following actions (as detailed in `run_checks.sh`):

*   **`go mod tidy` & `go mod verify`**: Manages and verifies dependencies.
*   **`go vet ./...`**: Reports suspicious constructs in Go source code.
*   **`staticcheck -tags=all ./...`**: Runs extensive static analysis.
    *   *Installation*: `go install honnef.co/go/tools/cmd/staticcheck@latest`
*   **`~/go/bin/gosec -quiet -tags=all ./...`**: Inspects source code for security vulnerabilities.
    *   *Installation*: `go install github.com/securego/gosec/v2/cmd/gosec@latest` (Ensure `~/go/bin` is in your `PATH` or adjust the script if installed elsewhere).
*   **`golangci-lint run --config=.golangci.yml ./...`**: Runs a Go linters aggregator using the `.golangci.yml` configuration.
    *   *Installation*: `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`
*   **`go build -v -tags=all ./...`**: Compiles the backend application and its packages.
*   **`go test -race -count=1 -tags=all ./...`**: Runs all tests with the race detector enabled.

Ensure you have the necessary Go tools (`staticcheck`, `gosec`, `golangci-lint`) installed and available in your system's `PATH` or Go binary path (e.g., `$(go env GOPATH)/bin`, which is often `~/go/bin`).

