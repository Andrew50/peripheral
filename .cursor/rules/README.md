Peripheral Trading Platform – AI-powered trading research and strategy development platform.
Overview
Peripheral is composed of multiple microservices working together to provide real-time market data processing, AI-driven analysis, and strategy backtesting. The core services include a Frontend (SvelteKit UI), Backend (Go API server), Worker (Python strategy engine), a PostgreSQL database, and a Redis cache


. This design enables independent scaling and technology choices per service (e.g. Go for API performance, Python for ML analytics)

. The system emphasizes performance (sub-millisecond trading decisions), security (sandboxed code execution, strict validation), and scalability (event-driven, horizontal scaling).
pgsql
Copy
Edit
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Frontend      │    │    Backend      │    │     Worker      │
│   (SvelteKit)   │ ── │   (Go/Gin)      │ ── │    (Python)     │
│                 │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         │          HTTP/WS API  │         Redis Queue   │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌─────────────────┐    ┌─────────────────┐
                    │   Database      │    │     Cache       │
                    │ (PostgreSQL)    │    │    (Redis)      │
                    └─────────────────┘    └─────────────────┘
High-Level Architecture: The frontend provides the user interface, communicating with the backend via HTTP requests and WebSockets for real-time updates

. The backend handles business logic and uses Redis pub/sub and queues to delegate heavy computations to the Python worker service

. The PostgreSQL database (with TimescaleDB extension for time-series data) stores persistent data, while Redis is used for caching and as a message broker. This event-driven design ensures low-latency data propagation and fault-tolerant processing.
Features
Real-Time Data: Live market quotes, charts, and alerts via WebSocket streaming.
AI Strategy Engine: Users can generate trading strategies using AI (GPT-based); strategies are validated and executed in a secure Python sandbox.
Backtesting & Screening: Historical strategy backtests and market screenings are run at high speed using optimized numpy-based computations.
Multi-Service Architecture: Independent services for UI, API, and computation allow horizontal scaling and specialized optimizations in each layer.
Security Focus: End-to-end encryption (HTTPS), JWT auth, input sanitization, and restricted code execution for user-submitted strategies.
Technology Stack
Frontend: SvelteKit (TypeScript) – Reactive web app for interactive charts, strategy builder, and AI chat interface.
Backend: Go (Gin framework) – REST API and WebSocket server for core trading logic and data handling

.
Worker: Python (PyPy) – Executes AI-generated strategy code and performs data-heavy computations (uses numpy for performance).
Database: PostgreSQL (TimescaleDB) – Time-series optimized relational database

 for storing market data, user info, and strategy results.
Cache/Queue: Redis – Caching frequently used data and managing task queues + pub/sub messaging between backend and worker.
Infrastructure: Docker containers for each service, orchestrated via Kubernetes for demo/prod environments

. CI/CD pipelines on  Actions for automated testing and deployment.
Installation & Running
For development and testing, Peripheral provides a Docker Compose setup that launches all services:
bash
Copy
Edit
# 1. Clone the repository
git clone https://.com/your-org/Peripheral.git
cd Peripheral

# 2. Launch services in development mode
docker-compose -f config/dev/docker-compose.yaml up

# The frontend (SvelteKit) will be available on http://localhost:5173
# The backend API server runs on http://localhost:5058 (with WebSocket on ws://localhost:5058)
# PostgreSQL on port 5432 (user: postgres, pass: devpassword), Redis on 6379.
This will build and start the frontend, backend, worker, db, and cache services. The backend will connect to the Postgres database and Redis cache automatically using the default dev credentials from .env (see .env.example for configuration). Alternatively, you can run services individually for development:
bash
Copy
Edit
# Run backend (Go)
cd services/backend && go run cmd/server/main.go

# Run worker (Python)
cd services/worker && python worker.py

# Run frontend (SvelteKit)
cd services/frontend && npm install && npm run dev
Make sure to set up the required environment variables (see .env.example for all necessary keys like database credentials, API keys, secrets, etc.) before running services.
Repository Structure
bash
Copy
Edit
Peripheral/
├── services/              # Core microservices source code
│   ├── backend/           # Go backend service (REST API, WebSocket):contentReference[oaicite:9]{index=9}
│   ├── frontend/          # SvelteKit frontend application
│   ├── worker/            # Python worker service (strategy engine)
│   ├── db/                # Database initialization (SQL schemas, migrations)
│   └── cache/             # Cache service (Redis configuration)
├── config/                # Deployment and environment configs
│   ├── deploy/            # Kubernetes manifests for staging/production
│   ├── dev/               # Local dev environment (docker-compose, scripts)
│   └── logging/           # Logging configs (e.g., file beats, etc.)
├── docs/                  # Additional technical documentation (backup, monitoring, etc.)
├── ./workflows/     # CI/CD pipeline definitions ( Actions):contentReference[oaicite:10]{index=10}
└── backups/               # Database backup files and snapshots
Each service directory contains its own source code and configuration:
backend: cmd/ (entry points), internal/ (application code organized into subpackages for each domain, e.g. app/, data/, server/), Dockerfiles for dev and prod.
frontend: SvelteKit project (see src/ with routes, components, stores, etc.), Node configuration, etc.
worker: Python package (src/) with strategy execution engine, validator, example strategies, plus a worker.py orchestrator for queue processing.
db: SQL migration scripts, seed data, and Docker setup for Postgres/TimescaleDB.
cache: Docker configuration for Redis (with persistence and custom settings).
Getting Help
Documentation: See the other documentation files (Architecture, API, etc.) for detailed guidance on system design and usage.
Issues: For any bugs or feature requests, use the internal issue tracker (JIRA or  Issues if enabled for this repo).
Discussion: Internal team members can reach out on Slack #project-Peripheral channel for quick questions or support.