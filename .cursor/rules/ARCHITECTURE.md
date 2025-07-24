System Design & Architecture
Peripheral is built with a microservice architecture to separate concerns and optimize each part of the stack. The key components and their responsibilities are:
Frontend (SvelteKit): Provides the web user interface for the platform. It handles user interactions (e.g. creating strategies, viewing charts, managing account settings) and communicates with the backend via REST API calls and WebSockets for live updates.
Backend (Go + Gin): Acts as the main API server and orchestrator. It exposes RESTful endpoints and a WebSocket endpoint for real-time data

. The backend handles authentication, user management, strategy CRUD operations, and delegates heavy computations to the worker service. It uses the PostgreSQL database for persistent storage and Redis for caching data and queuing tasks.
Worker (Python): A high-performance strategy execution engine. The worker runs user-generated Python code for strategy screening and backtesting in a sandboxed environment (for security)

. It receives tasks from the backend via a Redis queue and returns results either through Redis pub/sub or direct callbacks. The worker is optimized with numpy and PyPy to handle data-intensive tasks faster than traditional pandas-based approaches

.
Database (PostgreSQL/TimescaleDB): Central data store for user data, strategies, and historical market data. TimescaleDB (a PostgreSQL extension) is used to efficiently store and query time-series data (e.g. price history)

. The schema is optimized with partitioning and indexing for fast time-series queries and aggregations.
Cache/Queue (Redis): Serves a dual role as a cache for frequently accessed data (e.g. reference data, session tokens) and as a message broker. Redis pub/sub channels and lists enable event-driven communication between services – for example, the backend pushes a "strategy run" task to a Redis queue which the worker subscribes to, and the worker publishes results to a channel that the backend listens on

. This decoupling allows horizontal scaling of workers and non-blocking request handling.
Data Flow
User Request → Frontend → Backend → Worker: When a user initiates an action (e.g. running a strategy screening on a set of stocks), the request flows from the frontend to a backend API endpoint. The backend may directly handle simple requests (like retrieving data from the database), but for compute-heavy tasks, it enqueues a job to Redis. The worker picks up the job, processes it (executing the AI-generated strategy code on provided data), and returns results back via Redis. The backend then sends the results to the frontend (either in the API response if it waited synchronously, or via a WebSocket message for asynchronous updates). This event-driven pipeline ensures the UI stays responsive – long computations run asynchronously and the system can handle many requests in parallel. Real-Time Updates: The platform uses WebSockets for streaming live data (quotes, price updates, alerts). The backend pushes real-time events (e.g. price tick updates, new alert signals) to connected frontend clients. Under the hood, market data feeds (from external APIs like Polygon) are fed into the backend (or a separate data ingestor service), and then broadcast via Redis pub/sub and websockets. This design achieves low-latency propagation from data source to UI, critical for trading scenarios

. Persistence and Caching: The PostgreSQL database stores canonical data such as user profiles, saved strategies (including the strategy code, description, performance metrics), and historical market data. Redis caching is used for transient data and to reduce DB load for frequently accessed data (for instance, caching latest quotes or precomputed results for quick retrieval). The system’s backup processes (see docs/backup-recovery-system.md) ensure data durability with twice-daily backups and automatic recovery procedures in case of DB failures.
Service Responsibilities and Composition
Backend Service (Go)
Framework: Gin Web Framework for HTTP API and WebSocket endpoints.
Responsibilities: Authentication (OAuth 2.0 with Google and email/password with JWT issuance), user account management, strategy creation & editing, initiating strategy runs/backtests, aggregating results, and serving market data to the frontend.
Structure:
cmd/server/main.go launches the HTTP and WS servers.
internal/server/ contains route handlers (e.g., auth routes, schedule tasks, WebSocket management).
internal/app/ contains business logic subdivided by domain:
agent/ – AI assistant integration (manages chat conversations, uses GPT via the "Gemini" planner for suggestions)

.
strategy/ – Strategy management and execution logic (CRUD operations, triggering backtests, processing screening results)

.
account/, watchlist/, chart/, filings/ – various domains like user portfolio, watchlist management, chart data aggregation, SEC filings retrieval, etc.
internal/data/ – Data access layer (database queries, Redis connections, external API clients for market data like Polygon and Benzinga).
internal/jobs/ – Background jobs (scheduled tasks such as daily data refresh, alert checks) that run in a separate worker process (Go cmd/worker).
Communication: The backend communicates with the Python worker primarily through Redis. For example, when a strategy screening is run, the backend places a job in a Redis list (queue) and then waits on a Redis pub/sub channel for the result


. The HTTP request may block (with a timeout) until the worker responds, then the backend sends the result back to the client.
Security: Implements input validation and sanitization at the API layer, uses JWT tokens for auth, and ensures only authorized users can access their data (e.g., verifying strategy ownership before running it)

. Also coordinates with the worker’s code validator to enforce safe code execution.
Frontend Service (SvelteKit)
Framework: SvelteKit (with TypeScript) for building a reactive single-page application (SPA) that supports server-side rendering and client-side hydration.
UI Features:
Charting – Interactive candlestick charts with technical indicators (integrates a library or custom canvas for TradingView-like experience).
Strategy Builder – A UI for users to input strategy parameters or natural language descriptions, which are sent to the backend to generate or run strategies.
AI Chatbot – An in-app chat interface allowing users to ask an AI assistant (powered by the backend’s agent system) for market insights or strategy suggestions

.
Account & Portfolio – Screens for user portfolio, watchlists, and account settings.
State Management: Uses Svelte stores and context to manage state. Real-time updates are handled via a WebSocket client (src/lib/utils/stream/socket.ts) that listens for data events (e.g., price updates, alerts).
Routing: Organized by SvelteKit’s filesystem-based routing. For example, pages like /login, /signup, /app/strategies/[id] correspond to components in src/routes/. Some server routes (src/routes/api/*) may exist for auxiliary backend logic (though primary API calls go to the Go backend).
Design Decisions: Chose SvelteKit over frameworks like React for its performance (faster, smaller bundles) and developer experience

. TypeScript is used throughout to ensure type safety in API calls and data handling. The UI is component-based, promoting reusability (common components for forms, modals, charts) and clarity in data flow (unidirectional data flow with explicit props and events).
Worker Service (Python)
Role: Executes trading strategy code and complex data analysis tasks offloaded from the backend. This service enables AI-generated code to run in isolation.
Components:
Execution Engine: Runs user-defined strategy functions on market data. Uses an async event loop and sandboxing: untrusted code is executed with restricted globals (only pandas, numpy, and data provided) and prohibited operations (no filesystem or network calls, etc.)


. If code tries disallowed operations, the Security Validator catches it.
Security Validator: Checks the generated Python code for unsafe patterns (uses AST parsing or regex to disallow dangerous imports like os, sys, or functions like eval()

). Ensures the strategy adheres to required function signatures and fields (e.g., function must accept a DataFrame df and return a list of results with certain fields).
Data Provider: Interfaces with the database and external APIs to fetch raw data needed for strategies. For example, retrieving OHLCV data for a list of symbols or fundamental metrics. The data is likely provided to strategies as pandas DataFrames or numpy arrays.
Task Queue Manager: The worker continuously listens to a Redis list (e.g., strategy_queue) for new tasks. When a task appears (JSON specifying strategy ID and parameters), it retrieves the required data, runs the strategy, and then publishes results to a Redis channel (e.g., worker_task_updates) that the backend subscribes to


. The worker may run multiple tasks in parallel (depending on how it’s designed, possibly multi-process or just cooperative multitasking with async).
Performance Optimizations: The worker uses NumPy extensively for vectorized operations, avoiding Python loops where possible. In some critical paths, it may use PyPy (just-in-time compiled Python) to gain extra speed for pure Python code

. No precomputed technical indicators are fed to strategies — they must compute their own indicators from raw data, which encourages efficient algorithm implementation and creativity. Caching of data in memory or Redis is used to avoid repeated heavy data fetches (e.g., if multiple strategies request the same market data).
Example Workflow: A user requests to "find top 5 tech stocks with increasing volume and momentum". The backend (via the agent component) might generate a Python strategy code for this query, save it as a new Strategy, then instruct the worker to run it on the universe of tech stocks. The worker fetches required price and volume data, runs the code in sandbox, and returns a ranked list of stocks with scores. The backend receives this result and forwards it to the frontend, which displays it to the user.
Database Service (PostgreSQL/TimescaleDB)
Purpose: Central relational database for storing persistent data. This includes:
User data: accounts, profiles, OAuth tokens, subscription status (trial, paid, etc., integrated with Stripe as needed).
Strategies: saved strategies with their metadata (name, description, code, performance metrics, created date, etc.).
Market Data: time-series data for equities (OHLCV bars, fundamental data, earnings, etc.), which can be large in volume. TimescaleDB is utilized for efficient time-series storage, partitioning data by time for performance

.
Alerts/Signals: records of strategy alerts, logs of AI interactions, etc., if they need to be stored.
Migrations & Schema: The repository includes migration scripts (services/db/migrations/) for evolving the schema and an init/ directory for initial schema setup and seeding minimal data

. Key tables likely include users, strategies, strategy_results, prices (for historical prices), etc.
Backup & Recovery: The system implements an automated backup system that takes SQL dumps twice daily and verifies them




Cache & Messaging (Redis)
Cache: A Redis instance provides in-memory caching for frequently needed data. For example, it might cache the latest price of popular stocks, or the output of expensive computations (to quickly serve if requested again).
Session store: Redis might also be used to store user session data or JWT blacklist for token revocation, etc., due to its fast in-memory access.
Task Queue: The backend and worker communicate through a Redis queue. The backend pushes tasks (like "run strategy X on universe Y") onto a list, and the worker service pops tasks from it. This decoupling means the backend thread doesn’t handle the heavy computation directly and can continue serving other requests.
Pub/Sub: Redis channels facilitate publish/subscribe messaging. The worker publishes task results or progress updates on a channel (e.g., worker_task_updates), and the backend subscribes to get the result to complete the request. Similarly, other real-time events could be distributed via Redis pub/sub (though often the backend might push directly via WebSocket to clients, Redis could be used if multiple backend instances need to broadcast among themselves).
Scaling Consideration: Both the queue and pubsub systems allow multiple worker instances to run in parallel (they will compete for tasks in the queue) and multiple backend instances to handle incoming requests, without tight coupling. This enhances scalability and fault tolerance: if a worker goes down, another can pick up its tasks; if a backend restarts, the results are still in Redis for it to consume when back up (depending on implementation).
Core Design Decisions (ADR Highlights)
Several architecture decisions were made early to meet the platform’s requirements. (Detailed Architecture Decision Records are kept in docs/adr/.)
Microservices vs Monolith: Decision: Microservices. Rationale: We needed independent scaling of components (e.g., scale out workers separately from the API server) and the flexibility to use different languages best suited for each task (Go for concurrency, Python for data science)

. This choice allows faster iteration on individual services and isolation of faults (one service crash doesn’t bring down the whole system). ADR-001 covers this decision, comparing trade-offs with a monolithic approach.
Database Choice (SQL vs NoSQL): Decision: PostgreSQL (with TimescaleDB) over a NoSQL or specialized time-series DB. Rationale: We require strong consistency for financial data and complex relational queries (joining user data with strategy results, etc.). PostgreSQL provides reliability and SQL familiarity, while TimescaleDB extensions give the needed time-series performance (efficient storage, compression, and query ops on time-series)

. ADR-002 details this choice, noting that alternatives like MongoDB or InfluxDB were considered but found less suitable for our mix of relational and time-series needs.
Frontend Framework: Decision: SvelteKit over React/Vue. Rationale: Svelte’s compiler-driven approach results in smaller, faster web apps, which is beneficial for the real-time, data-intensive UI we needed. Developer experience and simplicity in reactivity were also factors

. ADR-003 compares SvelteKit with other frameworks, focusing on performance and bundle size.
AI Strategy Implementation: Decision: Have the AI generate raw Python strategies and require in-engine calculation of indicators rather than providing a library of technical indicators. Rationale: This forces the AI (and thus the code) to handle technical indicator computation from first principles, yielding two benefits: (1) ensures the AI’s output is fully understood and controlled (no black-box indicators), and (2) encourages innovation and learning, as the AI can create custom indicators. It also avoids maintaining a large library of indicators in the platform. ADR-004 discusses this design, including the challenges of this approach (like potential performance issues) and how we mitigated them with PyPy and numpy optimizations

.
Event-Driven Communication: Decision: Use Redis and WebSockets for real-time eventing, instead of polling or heavy synchronous calls. Rationale: Real-time trading data and alerts must propagate instantly to users

. By using a push model (WebSocket) and background processing (queue), we minimize latency and prevent blocking API operations. This also made it easier to scale horizontally – new worker instances can subscribe to the queue without changes to other services. ADR-005 covers this, including how we structured message formats and ensured reliability (e.g., what if a result is lost or delayed).
(For more details, refer to specific ADR files in docs/adr/ directory, which explain the context, decision, and consequences of each major architectural choice.)
Integration & Scalability
All services are containerized (Docker) and deployed on Kubernetes for production

. They communicate over internal networks within the cluster, with environment-specific configurations. We have ensured statelessness in services (except the database), meaning we can run multiple replicas of backend and worker to handle increased load. The database is the single source of truth and is scaled vertically (powerful instances) and via partitioning rather than sharding, given the strong consistency requirements. Scaling the Worker: We can increase the number of Python worker pods to handle more concurrent strategy executions. The Redis queue distributes tasks among them. Because strategy tasks are CPU-bound (numerical computations), the workers benefit from multi-core scaling. Each worker process might also internally parallelize tasks (e.g., using Python’s asyncio or multiprocessing for certain operations). Scaling the Backend: The Go backend can handle a large number of concurrent connections and requests (thanks to goroutines). For throughput scaling, multiple backend instances can be run behind a load balancer (they are stateless except for caching, and share state via Redis and database). WebSocket connections can also be load-balanced (sticky sessions or any instance can subscribe to the same Redis pubsub if needed to broadcast messages). High Availability: The use of separate services confines failures. If the worker service is temporarily down, the rest of the system remains up (though strategy runs will be delayed). If the backend is down, the frontend will show an error but the worker can continue any running tasks. We also employ health checks (Kubernetes liveness/readiness probes and custom health endpoints) for each service to allow automatic restarts or traffic shifting when needed. The database is perhaps the most critical component, and we rely on backups, replication (if configured), and the automated recovery scripts to maintain availability


.
Security Considerations
Security is baked into the architecture (Security-by-Design principle

):
Secure Coding & Execution: All code generated by AI is treated as untrusted. The worker’s sandbox prevents malicious actions (no filesystem or network access, limited globals)

. We also log and audit any execution of user code for anomalies.
Authentication & Authorization: The platform uses OAuth 2.0 (Google sign-in) as well as email/password with strong hashing. JWT tokens are issued for session management with appropriate expiration and claims (user ID, etc.). Backend endpoints validate JWTs and enforce authorization checks for user-specific resources (e.g., you cannot access another user’s strategy).
Transport Security: All client-server communication is over HTTPS/WSS in production. We also enforce secure cookies for session where applicable and use HSTS headers.
Data Protection: Sensitive data in the database (passwords, API keys if stored) are encrypted or hashed. The .env configuration ensures secrets (like API keys for third-party services, JWT secret, etc.) are not hard-coded and can be managed via Kubernetes secrets.
Dependency and Vulnerability Scanning: Our CI pipeline includes automated security scans. For Python, we run Bandit and safety; for Go, we run static analysis and gosec; and for frontend, we check npm audit and known vulnerabilities

. All issues identified are fixed or mitigated promptly (e.g., Bandit-reported issues were brought to zero by adding safe #nosec where code was acceptable)

. We also keep dependencies up-to-date (using tools like Dependabot or similar internal checks).
