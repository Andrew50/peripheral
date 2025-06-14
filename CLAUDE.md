# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## System Architecture

**Atlantis** is a microservices-based trading platform with AI-powered strategy development. The system consists of:

- **Backend** (Go): Core API server with WebSocket support (`services/backend/`)
- **Frontend** (SvelteKit): Web UI with real-time charting (`services/frontend/`)
- **Database** (PostgreSQL + TimescaleDB): Time-series data storage (`services/db/`)
- **Cache** (Redis): Caching layer (`services/cache/`)
- **Python Worker** (Python): Strategy execution engine (`services/python-worker/`)

## Development Commands

### Start Development Environment
```bash
cd config/dev && ./dev.bash
```
This starts all services with Docker Compose, runs database migrations, and streams logs.

### Frontend Development
```bash
cd services/frontend
npm run dev          # Start dev server on port 5173
npm run build        # Build for production
npm run test         # Run Jest tests
npm run lint         # Run ESLint + Prettier checks
npm run lint:fix     # Auto-fix linting issues
npm run check        # Type checking with svelte-check
```

### Backend Development
```bash
cd services/backend
go run cmd/server/main.go    # Start HTTP/WebSocket server on port 5058
go run cmd/jobctl/main.go    # Job control CLI
./run_checks.sh              # Run full Go checks (vet, staticcheck, gosec, golangci-lint, tests)
```

Backend check script runs: `go mod tidy`, `go vet`, `staticcheck`, `gosec`, `golangci-lint`, `go build`, and `go test -race`.

### Database Operations
```bash
cd services/db
./scripts/migrate.sh         # Apply pending migrations
./scripts/backup.sh          # Create database backup
```

Database uses versioned migrations in `services/db/migrations/` with automatic tracking via `schema_versions` table.

## Backend Architecture

### Application Structure (`services/backend/internal/app/`)
- **agent/**: AI-powered trading assistant with Gemini integration
- **strategy/**: Strategy compilation, backtesting, and Python execution
- **account/**: Trade handling and portfolio statistics  
- **chart/**: Chart data, drawing tools, and technical indicators
- **alerts/**: Price and news alert system
- **watchlist/**: Security tracking and monitoring

### Data Layer (`services/backend/internal/data/`)
- **postgres/**: Database queries and schema operations
- **polygon/**: Market data integration (real-time and historical)
- **edgar/**: SEC filings data integration

### Service Layer (`services/backend/internal/services/`)
- **socket/**: WebSocket real-time communication
- **marketdata/**: Daily OHLCV data ingestion
- **securities/**: Stock metadata and sector management
- **email/**: Notification delivery

## Frontend Architecture

### Feature Modules (`services/frontend/src/lib/features/`)
- **chart/**: Financial charting with Lightweight Charts library
- **chat/**: AI conversation interface with persistent context
- **strategies/**: Strategy development and management UI
- **account/**: Trading calendar and portfolio management

### External Integrations
- **Polygon.io**: Real-time market data and WebSocket feeds
- **Google Gemini AI**: Strategy development and conversation assistance
- **SEC Edgar**: Corporate filings and fundamental data
- **Google OAuth2**: Authentication provider

## Key Patterns

### AI Agent System
The AI agent uses tool-based execution with persistent conversation context stored in PostgreSQL. Prompts are in `services/backend/internal/app/agent/prompts/`.

### Strategy Execution
Python strategies run in a sandboxed worker environment. Strategy specs are compiled from user input and executed with comprehensive backtesting.

### Real-time Data Flow
WebSocket connections handle live market data streaming from Polygon.io to frontend charts with session-aware price display.

### Database Schema
Uses TimescaleDB for efficient time-series storage. Migrations are numbered sequentially (0.sql, 1.sql, etc.) and auto-applied on startup.

## Development Workflow

1. Use `config/dev/dev.bash` to start the full development environment
2. Backend changes trigger auto-restart via Docker volume mounts
3. Frontend uses Vite hot reload for immediate feedback
4. Database migrations auto-apply on container startup
5. Run `services/backend/run_checks.sh` before committing Go changes
6. Run `npm run lint` in frontend before committing frontend changes

## Environment Variables

Required for development (set in `config/dev/.env` or shell):
- `GEMINI_FREE_KEYS`: Google Gemini API keys
- `POLYGON_API_KEY`: Market data API key
- `GOOGLE_CLIENT_ID/SECRET`: OAuth authentication
- `EMAIL_FROM_ADDRESS`: Notification email configuration
- `SMTP_HOST/PORT`: Email server settings

The development environment uses PostgreSQL with user `postgres` and password `devpassword` on port 5432.