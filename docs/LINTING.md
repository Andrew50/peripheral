Our project has a comprehensive linting setup to enforce code quality and consistency across all languages. Linting runs automatically via  Actions on each push and pull request to main, and developers are encouraged to run linters locally as well.
Linting Overview
Linting is triggered on:
Every push to any branch (to give early feedback to developers).
All pull requests into main (required to pass before merge).
Manual invocation via workflow dispatch (for on-demand full checks).
Note: All linters run in check-only mode – they report issues but do not auto-fix code in CI

. Developers should fix issues manually (or use local auto-fix tools where available) and commit the changes. The lint workflow is defined in ./workflows/lint-and-build.yml. It runs linters for each part of the stack in parallel and then reports results. If any linter fails, the whole workflow is marked as failed.
Linters and Tools by Language
Go (Backend) – Uses golangci-lint (which includes many linters like revive, errcheck, etc.). Configuration in services/backend/.golangci.yml. Key enforcements:
Code must be formatted (gofmt).
No unused variables/imports (govet, staticcheck).
CamelCase naming for exported and unexported identifiers (custom revive rules)

.
Security: runs gosec to catch common security issues (e.g., insecure random, SQL injection risk).
Complexity: warns if functions are too complex (high cyclomatic complexity).
Python (Worker) – Uses Pylint and flake8 (plus plugins) with configuration in services/worker/.pylintrc and setup.cfg. Key checks:
Enforces PEP8 style (with our CamelCase override for naming)

.
Checks for unused imports, undefined variables.
Runs bandit security analyzer to catch insecure code (e.g., use of eval, weak cryptography).
Type hints checked by mypy: ensures function signatures and variable types are consistent.
Any # TODO or # FIXME must have an associated issue (ensured via a regex check plugin).
TypeScript/Svelte (Frontend) – Uses ESLint (.eslintrc.js) and Stylelint for styles.
ESLint: Checks for code issues, enforces our style via Prettier integration (indentation, quotes, etc.), and CamelCase naming in JS/TS code (using naming-convention rules)

.
Also runs Svelte-specific linting (via eslint-plugin-svelte) to catch issues in <script> and template.
Jest tests: The lint workflow also runs npm run test -- --passWithNoTests to ensure no test failures (though not strictly a linter, it's part of quality checks).
Stylelint: Lints CSS (including styles in Svelte files) for formatting and our naming convention (no snake_case class names, etc.).
SQL (Database) – Uses SQLFluff with the PostgreSQL dialect. Config in .sqlfluff. Rules:
Keywords must be UPPERCASE.
Identifiers (table/column) should be CamelCase (it flags snake_case names)

.
Ensures consistent indentation and line breaks for complex queries.
Ignores certain safe patterns or vendor-specific syntax as configured.
YAML/JSON (Configs) – Uses yamllint for YAML files (Kubernetes manifests,  workflows) and perhaps jsonlint for JSON if needed.
Yamllint config in .yamllint: ensures 2-space indent, no trailing spaces, etc.
Also checks that our  workflows and config files are well-formed.
Each of these linters is configured to fail on any errors or significant warnings. Minor warnings (info-level) might be allowed but should be addressed if possible.
Running Linters Locally
It’s best to run linting before pushing, to catch issues early:
Go (golangci-lint): Ensure golangci-lint is installed (go install .com/golangci/golangci-lint/cmd/golangci-lint@latest). Then run:
bash
Copy
Edit
golangci-lint run --config services/backend/.golangci.yml ./services/backend/...
This checks all Go files in the backend service. It will output any issues with file path and line number. You can also run golangci-lint run --fix to auto-fix some issues (like formatting) but most issues require manual fix.
Python (Pylint/Bandit):
bash
Copy
Edit
# From repo root, ensure venv is activated with dependencies installed
pylint services/worker/ --rcfile=services/worker/.pylintrc
bandit -r services/worker/
mypy services/worker/
We have a Makefile (if provided) or script to run all of these together. These will check code style, security, and types.
Frontend (ESLint/Stylelint):
bash
Copy
Edit
cd services/frontend
npm install  # if not already done
npm run lint    # runs ESLint on all .svelte/.ts files
npm run lint:styles  # runs Stylelint on styles
Our package.json has scripts configured for these. ESLint will also apply Prettier formatting fixes if possible (you can also run npm run format to auto-format TS/Svelte files).
SQL (SQLFluff):
bash
Copy
Edit
pip install sqlfluff
sqlfluff lint --config .sqlfluff services/db/**/**/*.sql
This will lint all .sql files (migrations, etc.). To auto-fix some issues (like indentation), you can run sqlfluff fix.
YAML (yamllint):
bash
Copy
Edit
yamllint -c .yamllint .
This checks all YAML files in the repo (you can restrict to config/deploy/ or ./ as needed). It will flag indentation or formatting problems.
Note: Running all linters might be integrated into a single command (e.g., npm run lint:all or a make lint). Check the repository README or scripts for any shortcuts.
Common Lint Rules and Policies
No snake_case: Any identifier in code should not be snake_case

. Our linters will flag snake_case function names, variable names, and even JSON keys in our own structures. (External JSON from APIs can be snake_case, but we convert or ignore those cases in lint config.)
Unused Code: Variables imported or defined and not used will cause lint errors (except in _ blank identifiers in Go). Remove unused code promptly.
Console Logs/Prints: Generally discouraged in committed code. Use proper logging (Go's log, Python logging, or console in frontend only when necessary). The linter may warn on stray console.log statements.
Todo/FIXME without issue: We treat TODO comments seriously. If you put TODO: in code, ideally reference an issue number or tracking ticket. The Python linter is configured to warn if "TODO" is found without a ticket reference.
Cyclomatic complexity: If a function is too complex (by lint metrics), consider refactoring. For example, a deeply nested or very long function in Go may trigger gocyclo via golangci-lint.
Deprecations: Lint will warn if you use deprecated APIs from libraries. Update to newer methods or note a plan to address it.
Security patterns: Bandit and gosec have their own rules (e.g., Bandit B101 will warn on use of assert in production code, or B303 on md5 usage). In cases where we deliberately use something safe (e.g., a false positive), we can use #nosec in Go or # nosec in Python comments to tell the linter to ignore that line. Use these sparingly and document why.
Handling Lint Issues
Fix immediately: The expectation is that all lint issues should be fixed before code is merged. Do not ignore or postpone fixes unless absolutely necessary.
Inline ignore: If there's a compelling reason to bypass a lint rule for a specific line (rare), use an inline comment to disable the rule and include a justification. Example for ESLint: // eslint-disable-next-line no-explicit-any -- needed for JSON parsing. For Python bandit: # nosec: reason....
Update the rule set if needed: If a particular rule is too noisy or not fitting our style, bring it up for discussion. We can adjust the lint configs (in .golangci.yml, .pylintrc, etc.) if warranted. Changes to lint rules should be approved by the team.
Continuous Integration (CI) Aspect
Our CI pipeline (Lint and Build workflow) will run all these checks in parallel


. It provides a summary of results:
If all pass, great!
If some fail, the CI annotations or logs will point to the file and line of the issue. Use that info to fix the code and push a new commit.
The CI also runs the build (and possibly tests) to ensure that code not only lints but also compiles/passes tests.
We treat a failing lint as a failing build – the code cannot be merged until it's resolved.
Summary of Config Files
.golangci.yml – config for golangci-lint (Go rules).
services/worker/.pylintrc and setup.cfg – configs for Pylint/flake8 (Python rules).
.eslintrc.js – ESLint configuration (extends recommended and Prettier).
.prettierrc – Prettier formatting rules (if separate).
.stylelintrc.json – Stylelint for CSS.
.sqlfluff – SQLFluff config (specifies dialect and rules).
.yamllint – Yamllint config.
These are all kept in version control. If you modify any of these configs, be sure to get it reviewed, as it affects all developers.
Enforcing Style vs. Functional Issues
Remember, linters are mostly about style consistency and catching common mistakes. They complement but do not replace code reviews and testing. Always consider the logic and design of your code in addition to making it pass the linters. Our goal is not just lint-free code, but clean, robust code – linting is a means to that end. By following this linting guide and using the automated tools, you help keep the codebase maintainable and error-free. Consistent style also makes it easier for everyone (including automated tools like our AI assistants) to read and understand the code. Happy linting!
API.md
This document outlines the main API endpoints of the Peripheral Trading Platform backend. All API endpoints are served by the Backend (Go) service over HTTP. The base URL for the API (in development) is http://localhost:5058/api/ (assuming a prefix of /api for routes), and in production it would be an HTTPS URL on the deployed server. All requests and responses use JSON format. Clients must include an Authorization: Bearer <token> header for endpoints that require authentication (after logging in to get a JWT). Note: This is a summary of key endpoints and their request/response structures. For comprehensive details, refer to the OpenAPI specification (to be provided) or the code documentation.
Authentication & User Accounts
POST /api/auth/signup – Register a new user.
Request: JSON body with email, password, and (optionally) inviteCode. Example:
json
Copy
Edit
{ "email": "user@example.com", "password": "MySecurePass123", "inviteCode": "INVITE-XYZ" }
Response: on success, returns 201 Created with user info or a minimal success message. Typically:
json
Copy
Edit
{ "message": "Signup successful, please log in." }
(If using invite codes, the backend will validate the code and possibly provision a trial subscription.)
POST /api/auth/login – Log in an existing user.
Request: JSON with email and password.
Response: 200 OK with authentication details:
json
Copy
Edit
{
  "token": "<JWT_TOKEN>",
  "settings": "{...}",            // A JSON string or object of user settings/preferences
  "setups": [ ... ],             // Array of preset configurations (if applicable)
  "profilePic": "https://..."    // URL to profile image, if available
}
The token should be saved by the client for authenticated requests. The other fields provide initial data (settings could include UI preferences, etc.).
POST /api/auth/logout – Log out the current user (optional, since if using JWT, client can simply discard token; but this endpoint could invalidate the token on server side if using token blacklist).
Request: (Header only, JWT identifies user).
Response: 200 OK with { "message": "Logged out" }.
GET /api/account – Get the authenticated user's account profile. (Auth required)
Response: 200 OK with user profile data, e.g.:
json
Copy
Edit
{
  "userId": 42,
  "email": "user@example.com",
  "name": "Alice",
  "createdAt": "2024-01-15T10:00:00Z",
  "plan": "trial",    // subscription plan
  "planExpiry": "2024-02-14T10:00:00Z",
  "preferences": { ... }
}
(The exact fields may include subscription info if integrated with Stripe, etc.)
PUT /api/account – Update account information. (Auth required)
Request: JSON with fields to update (e.g., name, password, etc.).
Response: 200 OK with updated user profile (or a success message). Password updates may require additional verification (like current password).
Strategy Management
GET /api/strategies – List all strategies for the authenticated user. (Auth required)
Response: 200 OK:
json
Copy
Edit
[
  {
    "strategyId": 101,
    "name": "Tech Momentum",
    "description": "Buy tech stocks with increasing momentum",
    "score": 85,
    "createdAt": "2024-03-01T12:00:00Z",
    "isAlertActive": true
  },
  {
    "strategyId": 102,
    "name": "Undervalued Large Caps",
    "description": "Value strategy for large cap stocks",
    "score": 90,
    "createdAt": "2024-03-05T09:30:00Z",
    "isAlertActive": false
  }
  // ...more strategies
]
Each strategy in the list includes summary info. The score might represent a backtest performance metric or AI-given quality score, and isAlertActive indicates if continuous alerting is enabled for that strategy.
POST /api/strategies – Create a new strategy. (Auth required)
There are two ways to create:
From scratch/code: Provide a strategy definition directly.
From natural language prompt: Provide a prompt that the AI will use to generate a strategy.
Request: JSON either containing explicit fields or a prompt. For example, to create from code:
json
Copy
Edit
{
  "name": "Mean Reversion",
  "description": "Buys stocks that dip below their 20-day MA and sells after rebound",
  "pythonCode": "...",  // Possibly a code template if user wrote it directly
  "prompt": ""          // empty or omitted if providing code directly
}
Or to create from an AI prompt:
json
Copy
Edit
{
  "prompt": "A strategy to buy S&P 500 stocks when RSI < 30 and sell when RSI > 50"
}
The backend will either save the provided code or call the internal agent to generate the pythonCode from the prompt (using GPT/Gemini), then save the strategy. Response: 201 Created with the created strategy object, including the generated strategyId and possibly the code if it was AI-generated:
json
Copy
Edit
{
  "strategyId": 103,
  "name": "Mean Reversion",
  "description": "Buys stocks below 20-day MA, sells after rebound",
  "prompt": "A strategy to buy ... RSI < 30 ...", 
  "pythonCode": "# Python code here",
  "createdAt": "2024-03-10T14:20:00Z"
}
GET /api/strategies/{id} – Get details of a specific strategy (that belongs to the user). (Auth required)
Response: 200 OK:
json
Copy
Edit
{
  "strategyId": 101,
  "name": "Tech Momentum",
  "description": "Buy tech stocks with increasing momentum",
  "prompt": "Find tech stocks with recent momentum upticks",
  "pythonCode": "def strategy(df): ...",   // The full strategy code
  "createdAt": "2024-03-01T12:00:00Z",
  "lastRun": "2024-03-08T15:00:00Z",        // when it was last executed
  "lastRunResult": { ... }                 // summary of last run (maybe performance or alerts triggered)
}
This lets the user view or edit the strategy code and metadata.
PUT /api/strategies/{id} – Update a strategy’s name, description, or code. (Auth required)
Request: JSON with fields to update (e.g., new name or description, or updated pythonCode after user edits).
Response: 200 OK with updated strategy object (or { "message": "Strategy updated" }).
DELETE /api/strategies/{id} – Delete a strategy. (Auth required)
Response: 200 OK with confirmation message. The strategy and any associated results are removed from the database.
POST /api/strategies/{id}/run – Execute a strategy screening/backtest. (Auth required)
This endpoint triggers the strategy to run on a specified universe or historical period. Request: JSON allowing parameters:
json
Copy
Edit
{
  "universe": ["AAPL", "MSFT", "GOOG"],   // list of symbols to screen (for screening strategies)
  "limit": 50,                            // e.g., only return top 50 results
  "startDate": "2023-01-01",              // for backtest: start of period
  "endDate": "2023-12-31",                // for backtest: end of period
  "mode": "screen"                        // or "backtest"
}
If the strategy is a screening strategy (one outputting current opportunities), the universe param is used. If it’s a backtest (applying strategy over time), the date range is used. The mode might distinguish whether to run a one-time screen on current data or a historical backtest. Response:
On success, 200 OK. If synchronous, it returns the results directly:
json
Copy
Edit
{
  "rankedResults": [
     { "symbol": "AAPL", "score": 9.5, "currentPrice": 150.23, "sector": "Technology", "data": { ... } },
     { "symbol": "MSFT", "score": 8.7, "currentPrice": 250.10, "sector": "Technology", "data": { ... } }
  ],
  "scores": { "AAPL": 9.5, "MSFT": 8.7, "GOOG": 7.4, ... },  // maybe a dictionary of all scores
  "universeSize": 100
}
(This corresponds to a screening result structure, showing top matches and scores for entire universe.) For a backtest mode, the result might include a performance summary:
json
Copy
Edit
{
  "performance": {
     "startCapital": 100000,
     "endCapital": 124000,
     "ROI": 0.24,
     "maxDrawdown": 0.10
  },
  "tradeLog": [ ... ]  // list of trades or signals generated during backtest
}
Backtest results could be large, so sometimes the API might just return a summary or an ID to fetch detailed results.
If asynchronous (taking long): The API might immediately return 202 Accepted with a task ID:
json
Copy
Edit
{ "taskId": "screening_101_1697050000000", "status": "running" }
and the client would then either poll a result endpoint or listen on WebSocket for a "taskCompleted" message. However, given our implementation, it seems the backend waits for the result up to a timeout

, so likely the API call is synchronous for runs that complete under a few minutes.
GET /api/strategies/{id}/results – (If applicable) Fetch the last or specific results of a strategy run. (Auth required)
Some strategies might have ongoing alerts or last run stored. This endpoint would retrieve either:
The latest screening results (like what /run returns, if you didn't call it just now).
Or historical backtest results (maybe by date or run ID if multiple runs stored).
For simplicity, if implemented, Response: similar JSON as the /run endpoint returns, containing results or performance metrics.
Market Data Endpoints
(Note: Real-time market data is primarily delivered via WebSocket, but some REST endpoints exist for on-demand data or debugging.)
GET /api/quotes – Get real-time quote for one or multiple symbols.
Query params: ?symbol=AAPL or ?symbols=AAPL,MSFT,GOOG.
Response: 200 OK:
json
Copy
Edit
{
  "quotes": [
    { "symbol": "AAPL", "price": 150.12, "change": -0.5, "changePercent": -0.33,
      "timestamp": "2025-07-17T20:58:00Z" },
    { "symbol": "MSFT", "price": 250.55, "change": +1.2, ... },
    ...
  ]
}
Each quote might include additional fields like day high/low, volume, etc. This endpoint hits a cache or external API to fetch current prices if not streaming.
GET /api/history – Get historical price data for a symbol.
Query params: ?symbol=GOOG&interval=1d&start=2024-01-01&end=2024-06-30.
Response: 200 OK:
json
Copy
Edit
{
  "symbol": "GOOG",
  "interval": "1d",
  "start": "2024-01-01",
  "end": "2024-06-30",
  "prices": [
    { "date": "2024-01-03", "open": 100.0, "high": 102.5, "low": 99.5, "close": 101.0, "volume": 1200000 },
    ...
  ]
}
This would likely pull from the database (which stores historical data from Polygon or similar source).
GET /api/news – Get recent news headlines (or filings) for a symbol or market.
Query: ?symbol=AAPL or none for general market news.
Response:
json
Copy
Edit
{
  "symbol": "AAPL",
  "news": [
    { "title": "Apple releases new product...", "source": "Bloomberg", "publishedAt": "2025-07-17T15:00:00Z", "url": "https://..." },
    ...
  ]
}
(If integrated with a news API or using SEC filings from filings component, this could also show latest filings or significant events.)
GET /api/filings – Company filings (SEC EDGAR) for a given symbol.
Query: ?symbol=MSFT.
Response:
json
Copy
Edit
{
  "symbol": "MSFT",
  "filings": [
    { "type": "10-Q", "date": "2024-07-30", "title": "Quarterly Report", "url": "..." },
    { "type": "8-K", "date": "2024-06-15", "title": "Press Release", "url": "..." }
  ]
}
(This leverages the internal/app/filings/ service to fetch cached or live data.)
GET /api/watchlist – Get the user's watchlist symbols and maybe their latest prices. (Auth required)
Response: 200 OK:
json
Copy
Edit
{
  "watchlist": [
    { "symbol": "AAPL", "price": 150.12, "changePercent": -0.33, "notes": "Tech giant" },
    { "symbol": "TSLA", "price": 720.50, "changePercent": +2.1, "notes": "" }
  ]
}
The watchlist is stored per user in the DB. There may be a corresponding POST to add symbol, DELETE to remove symbol, or PUT to update notes.
AI Assistant & Chat Endpoints
Peripheral includes an AI-powered assistant (the "agent" system) to help users with strategy creation and Q&A:
POST /api/chat – Interact with the AI assistant. (Auth required)
Request: JSON with user’s message:
json
Copy
Edit
{ "message": "What do you think about Tesla's stock trend?" }
Response: The assistant’s reply:
json
Copy
Edit
{ "response": "Tesla has been on an upward trend due to strong earnings. Technical indicators show ...", "sources": [] }
The backend uses GPT (via the planner in app/agent) to generate a response. Optionally, it might include sources or references if it pulls in data (or blank if just model-generated). Internally, the conversation context may be maintained (possibly via session or conversation ID), but if stateless, each call includes recent context. A WebSocket is possibly used for real-time streaming of assistant responses (for a more interactive feel).
POST /api/chat/strategy – Ask AI to generate a strategy from a description (alternate to creating via /strategies prompt).
Request:
json
Copy
Edit
{ "query": "I want a strategy that buys when RSI < 30 and sells when > 50." }
Response:
json
Copy
Edit
{
  "strategyId": 110,
  "name": "RSI Swing",
  "description": "Buys when RSI < 30, sells when RSI > 50",
  "pythonCode": "def strategy(df): ...",  // the AI-generated code
  "info": "Make sure to feed daily OHLCV data into this strategy."
}
This creates a new strategy (similar to POST /strategies with prompt) and returns it directly for convenience, so the user can run it or tweak it further.
Alerts & Monitoring Endpoints
GET /api/alerts – Get active alerts triggered by user’s strategies. (Auth required)
Response:
json
Copy
Edit
{
  "alerts": [
    { "strategyId": 101, "strategyName": "Tech Momentum", "symbol": "AMD", "trigger": "Price above 5% of 20-day avg", "triggeredAt": "2025-07-17T20:00:00Z" },
    { "strategyId": 102, "strategyName": "Undervalued Large Caps", "symbol": "IBM", "trigger": "PE below threshold", "triggeredAt": "2025-07-17T19:30:00Z" }
  ]
}
Each alert entry shows which strategy and condition triggered. These alerts might be generated by background jobs (internal/jobs/alerts).
POST /api/alerts/toggle – Enable or disable alerts for a strategy. (Auth required)
Request: e.g. { "strategyId": 101, "enable": false } to disable.
Response: { "message": "Alerts disabled for strategy 101" }.
(This updates a flag in the strategy like isAlertActive which the background alert job uses to decide which strategies to run continuously.)
WebSocket Endpoint
Though not an HTTP endpoint, it’s important to mention:
WebSocket /ws (or /api/ws): Clients can open a WebSocket connection to receive real-time updates. After connecting, the client likely sends an authentication message (if required, e.g., sending the JWT) and a subscription request for certain data (or the server automatically pushes relevant data). Over the WebSocket, the server may send messages such as:
json
Copy
Edit
{ "type": "quote", "symbol": "AAPL", "price": 151.00, "change": +0.5 }
for live quote updates, or
json
Copy
Edit
{ "type": "alert", "strategyId": 101, "symbol": "AMD", "trigger": "Price above 5% of 20-day avg", "triggeredAt": "2025-07-17T20:00:00Z" }
when an alert triggers, or
json
Copy
Edit
{ "type": "chat", "response": "Continuing response chunk...", "done": false }
if streaming AI chat responses. The exact message schema can vary. Typically, type indicates what event or data it is. The frontend’s stream handlers (see stream/socket.ts) define how these are handled.
Error Handling
All endpoints generally return standard HTTP error codes:
400 Bad Request for invalid input (with a JSON error message like { "error": "Invalid date format" }).
401 Unauthorized if token missing/invalid (for protected endpoints).
403 Forbidden if user lacks permission (e.g., accessing another’s strategy).
404 Not Found if a resource doesn’t exist (wrong ID).
500 Internal Server Error for unexpected issues (the server will log details; the response might just say { "error": "Internal server error" }).
The client should handle error responses gracefully. The error JSON typically contains an "error" or "message" field with a human-readable explanation.
Future/API Notes
OpenAPI Spec: We plan to maintain an OpenAPI (Swagger) specification file (openapi.yaml) to exactly document request/response schemas. This will help with client generation and ensure up-to-date docs. (Currently, the docs above reflect the implementation but refer to code for edge-case details.)
Versioning: At present, the API is v1 (no explicit version in URL). If breaking changes are made, we will introduce versioned endpoints (e.g., /api/v2/...).
Rate Limiting: For production, certain endpoints (especially public ones) may be rate-limited for security. Authenticated user actions are less likely to be rate-limited except to prevent abuse (like excessive login attempts or massive data pulls).
Webhooks: The backend might expose webhooks for Stripe (e.g., /api/webhook/stripe) to handle subscription events. These are secured by secret and not for client use, so they’re not covered here.
This covers the key API surfaces of Peripheral. Developers working on the frontend or external integrations should use these endpoints to interact with the system. Always ensure you have a valid JWT for protected calls, and refer to this document or the OpenAPI spec for data format details. If you’re extending the API, update this document accordingly so our “source of truth” remains accurate.