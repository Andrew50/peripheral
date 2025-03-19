# Backend Service

This project has been restructured following Go best practices and the standard project layout.

## Directory Structure

```
backend/
  ├── cmd/                  # Main applications
  │   └── backend/          # Main server application
  │       └── main.go       # Entry point
  │
  ├── internal/             # Private application code
  │   ├── server/           # Server package
  │   │   ├── http/         # HTTP server implementation
  │   │   └── handlers/     # API endpoint handlers
  │   │       ├── auth/     # Authentication handlers
  │   │       ├── health/   # Health check handlers
  │   │       ├── users/    # User-related handlers
  │   │       └── queue/    # Queue-related handlers
  │   │
  │   ├── auth/             # Authentication logic
  │   ├── domain/           # Business domains
  │   ├── jobs/             # Background jobs
  │   ├── socket/           # WebSocket functionality
  │   └── alerts/           # Alert system
  │
  ├── pkg/                  # Public/reusable packages
  │   ├── utils/            # Utility functions
  │   └── tools/            # Tooling
  │
  ├── tasks/                # Task definitions
  │
  ├── scripts/              # Shell scripts
  │   ├── lint.sh
  │   └── fix_id_naming.sh
  │
  ├── configs/              # Configuration files
  │   └── .air.toml
  │
  ├── build/                # Build-related files
  │   ├── Dockerfile.dev
  │   └── Dockerfile.prod
  │
  └── go.mod                # Go module file
```

## Key Concepts

1. **Separation of Concerns**: Code is organized by functional domain and layer
2. **Clean Architecture**: Controllers/handlers -> Services -> Repositories -> Models
3. **Modularity**: Each package has a specific responsibility
4. **Testability**: Easier to write unit tests for focused components

## Migration Notes

This is a work in progress. The following steps are needed to complete the migration:

1. Update import paths throughout the codebase
2. Move handler logic from large files to their respective domain packages
3. Create proper interfaces between layers
4. Ensure tests are updated to reflect the new structure

## Running the Application

```bash
# Development with hot reload
air -c configs/.air.toml

# Production build
go build -o bin/backend cmd/backend/main.go
```

## Contributing

Please follow the directory structure and package organization when adding new features. 