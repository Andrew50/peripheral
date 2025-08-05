This document describes how the Peripheral Trading Platform is deployed and the infrastructure processes around building, testing, and releasing the application. It covers continuous integration/deployment (CI/CD), environment configuration, and server infrastructure (Docker, Kubernetes, etc.).
Environments & Branches
We maintain separate deployment environments corresponding to Git branches:
Development – The latest code in the main branch is considered the development version. Developers test changes here locally or on ephemeral environments (like using Docker Compose). There isn't a long-lived shared dev server; each dev usually runs services locally or uses feature branches.
Staging (Demo) – The demo branch corresponds to the staging environment (often called "demo"). This is where integration testing with production-like configuration happens. QA and stakeholders can test new features here. Deployments to staging happen whenever changes are merged from main into demo (manual step by maintainers, typically).
Production – The prod branch corresponds to the production environment. Only thoroughly tested changes from staging (demo) are merged into prod for release.
Branch Protection: Both demo and prod (and main) are protected; you cannot directly push to them. All changes flow through pull requests and merges

. This ensures that only reviewed code is deployed.
CI/CD Pipeline
Our CI/CD pipeline is defined via  Actions in the ./workflows/ directory. Key workflows:
Lint and Test: Runs on every push and PR (targeting any branch). It lints code and runs the test suite

. This must pass before any merge.
Build and Publish: On merges to main, demo, or prod, we trigger a build pipeline. This pipeline does the following:
Build Docker images for each service (backend, frontend, worker, etc.) using the Dockerfiles.
Run unit/integration tests inside containers if any (for extra verification).
If on main: push images to the container registry tagged with :main (and possibly the commit SHA).
If on demo: push images tagged :demo and then deploy to the staging environment.
If on prod: push images tagged with a version or :prod and deploy to production.
Deployment triggers: Merging to demo or prod might either automatically trigger a deployment job (if configured to auto-deploy) or notify us to deploy manually. Currently, we have an automated deploy on these merges:
The workflow uses kubectl or an API to update the Kubernetes deployment images to the newly built image tags.
Alternatively, we might use an ArgoCD or Flux setup where pushing to a branch updates a manifest that ArgoCD picks up. In our case, we keep Kubernetes manifests in config/deploy/, and they reference image tags like :demo or :prod.
Docker Image Build: Each service has a Dockerfile optimized for production (e.g., multi-stage builds to minimize image size). We also have a Dockerfile.dev for a more flexible dev image (with hot-reload, etc.)


:
The CI uses the production Dockerfile when building for demo/prod to ensure we test the same image that will run in production.
Images are tagged as Peripheral-backend:<branch> (and similarly for other services).
We use  Packages or Docker Hub as the registry (configured via secrets DOCKER_USERNAME, etc., in CI).
Kubernetes Deployment
Peripheral is deployed on a Kubernetes cluster (e.g., on a cloud provider). We have Kubernetes manifest files in config/deploy/ that describe our services, deployments, config maps, and secrets. Kubernetes Resources:
Namespace: e.g., Peripheral-demo and Peripheral-prod for isolation.
Deployments: We have a Deployment for each service (backend, frontend, worker, db, cache). For example, backend-deployment.yaml defines the container (using image Peripheral-backend:demo or :prod accordingly), replicas, resource limits, and environment variables.
Services: Kubernetes Service objects to expose internal communication and (for frontend or API) LoadBalancer/Ingress for external access.
ConfigMaps & Secrets:
ConfigMap might hold non-sensitive configs (like feature flags or static config).
Secrets store sensitive values (database password, JWT secret, API keys). These are populated from our environment (we might use Kubernetes secrets manually, or helm with a values file).
Our .env.example reflects what env variables need to be set. In Kubernetes, those come from the Secret. For instance, JWT_SECRET, third-party API keys, etc., are in a Secret and mounted as env vars in the pods.
Persistent Volume: The Postgres database uses a PersistentVolumeClaim (PVC) to store data. Our backup system mounts an additional PVC for backup files

. In production, ensure the volume is backed up or replicated as needed.
Scaling Config: The worker Deployment might have a higher replica count (e.g., 3 or more workers) to handle multiple tasks in parallel

. The backend might typically run 2+ replicas behind a load balancer for redundancy.
Ingress: For production, an Ingress or cloud LB is configured to route traffic:
Web requests to frontend (maybe served statically or via SvelteKit SSR container).
API requests to backend service.
WebSocket to backend (possibly same domain with /ws path).
We may use a single domain, e.g., app.Peripheral.com where frontend assets and API live together (since SvelteKit can serve the frontend and proxy API, or we host them separately).
Deployment Process:
For staging (demo): After CI builds the :demo images, it updates the K8s manifests (either by kubectl set image or applying updated YAML) to use those images. We then run kubectl apply -f config/deploy/demo (if separate manifests or a Kustomize overlay for demo) or a helm release upgrade. The CI might do this automatically with the credentials (kubeconfig or token stored in  Actions secrets).
For production: Similarly, CI (or a manual step) updates images to :prod tag and applies the manifests in the prod namespace. We might do a rolling update deployment. Zero downtime is achieved via readiness probes and rolling update configuration (Kubernetes by default will spin up new pods and then terminate old ones).
We ensure migrations are run on the database before a new release that requires them. This can be done by:
Running migrations as a Kubernetes Job or init-container.
Or running a migration CLI (maybe our backend has a command or we use a tool) as part of the deployment pipeline.
Infrastructure Config:
The cluster has monitoring (Prometheus & Grafana) and logging (ELK or cloud logging) set up. Each service logs to STDOUT, and those are collected by cluster logging. Alerts are configured for downtime or high error rates.
Environment-specific values: Some environment variables differ between demo and prod (like VITE_ENVIRONMENT=staging vs production, different API keys possibly). We manage that via separate Kubernetes Secret for demo vs prod, or using Helm values for each environment.
Secrets and Configurations
We maintain an .env.example to list required environment variables. In deployment:
For local/dev: .env file is used by docker-compose (as seen in docker-compose.yaml, it loads .env

).
For Kubernetes: we create secrets from these values. For example, a Kubernetes Secret manifest or using kubectl create secret generic. We never commit actual secret values to the repo; those are set via the deployment pipeline or manual ops in a secure store.
CI has necessary secrets (like DOCKER_USERNAME, DOCKER_PASSWORD for registry, possibly K8s credentials). These are stored in  Actions secrets, not in code.
Key secrets:
DB_PASSWORD (for Postgres)
JWT_SECRET (for auth token signing) – must be strong and kept secret.
API keys: POLYGON_API_KEY, STRIPE_SECRET_KEY, GOOGLE_CLIENT_ID/SECRET (for OAuth), etc.
SMTP_PASSWORD (if we send emails via SMTP).
These are all configured in Kubernetes secrets and injected as env vars.
We provide a config/dev/.env.example and likely separate secret management for demo and prod. For instance, the demo environment might use test API keys, whereas prod uses live keys.
Deployment Steps for Maintainers
Merge to main: Developer merges a feature PR to main. CI runs tests. This code is now slated for next demo deploy.
Merge to demo: At a scheduled release time (or after accumulating a few features), maintainers create a PR from main into demo. After ensuring CI passes and perhaps doing final checks, merge it. This triggers the staging deployment workflow.
Staging Verification: The demo environment (on a staging URL, e.g., demo.Peripheral.internal) updates to the new code. The team performs sanity checks, regression tests, and any exploratory testing. If issues are found, they can be fixed on main and then merged again into demo.
Merge to prod: Once satisfied, open a PR from demo to prod. Merge it. This triggers production deployment.
Production Monitoring: After deploy, closely monitor logs, metrics, and alerts. Ensure that pods are healthy (Kubernetes will handle rolling update; check that all new pods are running and old ones terminated). Verify a few core functionalities on the live site.
Post-Deploy: Communicate the release to stakeholders if needed (release notes). Update any version numbers or documentation.
Rollback Plan: In case a deployment goes bad:
We can rollback to previous version quickly by either:
Checking out the previous commit (or using git revert on prod branch to the last known good commit) and redeploying.
Or using Kubernetes kubectl rollout undo deployment/<service> to revert to prior ReplicaSet (works if the previous spec is retained).
Data migrations: If a migration has been run that is not backward compatible, rollback is harder. We design migrations to be backward-compatible when possible or have a backup. In worst case, restoring the DB from backup might be needed if a migration corrupted data (this is rare and we try to avoid it).
Feature flags: For risky features, we might employ feature flags to turn them off without redeploying. These can be toggled via config if an issue is discovered.
Docker & Containerization
Each service has its own Dockerfile:
Backend Dockerfile.prod: Based on a lightweight Go runtime (e.g., golang:1.19-alpine for build then scratch or alpine for run). It compiles the Go binary (CGO_ENABLED=0) and then uses a smaller image to run it. Exposes port 5058.
Frontend Dockerfile: Might use node:18-alpine to build the static site or SvelteKit server, then use nginx or Node to serve. If SvelteKit is in SSR mode, we may run it as a Node process. Alternatively, we pre-build a static bundle and serve via nginx (if using SvelteKit in static export mode). The dev Dockerfile mounts source for live reload, whereas prod is optimized.
Worker Dockerfile: Based on Python 3.10 slim, installs requirements. Possibly uses PyPy base image for performance. Ensures no internet access in sandbox (though that's at code level too). Exposes no port (it’s a worker process), but might for metrics.
DB Dockerfile: Actually for TimescaleDB, we might use the official image (with some custom entrypoint to apply migrations or configure). In dev, services/db/Dockerfile.dev extends a Postgres image to create a user, etc. In prod, we likely use official Postgres/Timescale images and handle config via manifest.
Cache Dockerfile: Not much, possibly uses official Redis image. In dev compose, they had a services/cache/Dockerfile (maybe to enforce certain config). In prod, use official Redis and config via ConfigMap.
Networking: In docker-compose, all services share a network and can refer by name (backend, db, etc.)

. In Kubernetes, they reside in the same namespace and communicate via service DNS (db.Peripheral-demo.svc.cluster.local etc.).
Monitoring & Logging
Deployment is not complete without monitoring:
We have health checks endpoints (e.g., /api/health on backend, and perhaps simple checks on other services) which Kubernetes uses for liveness/readiness.
A separate docs/cluster-monitoring.md describes how we monitor cluster and app health (it likely covers Prometheus alerts on CPU/memory, Telegram alerts on backup events, etc. as hint from backup doc).
Logging: Each service logs to console. The backend uses structured logging for important events (like each request, errors, etc.). Worker logs strategy execution and any errors (with strategy ID context).
We integrate with an alerting channel (the backup system has Telegram alerts

; similarly, critical app alerts could notify developers via Slack or email).
Deployment Summary / Cheat Sheet
For reference, to deploy manually (if CI is not used):
Build images: docker build -t Peripheral-backend:prod -f services/backend/Dockerfile.prod ./services/backend (and similarly for other services).
Push images: docker push your-registry/Peripheral-backend:prod.
Update K8s manifests image tags (or use Helm values to set the new image digest).
kubectl apply -f config/deploy/prod (assuming manifests are parametric or pre-set to use :prod tag).
Watch rollout: kubectl rollout status deploy/backend -n Peripheral-prod for each deployment.
If needed, run migrations: e.g., kubectl run migrate -i --rm --image Peripheral-backend:prod --command -- ./backend migrate (if we have a CLI command for migration). Or connect to DB and run SQL if manual.
But ideally, our CI handles these steps automatically, requiring only merging to the correct branch.
Miscellaneous
Demo Data: In staging, we might use scrubbed or sample data. E.g., using a smaller database snapshot or mock external API keys (so as not to incur cost or unwanted external actions). Ensure in demo environment config that any external integrations (Twitter, email, payments) are in test mode.
Scaling Config (HPA): In production, if load varies, consider Kubernetes Horizontal Pod Autoscaler (HPA). For example, auto-scale worker pods based on CPU or queue depth (if we export that as a metric), auto-scale backend based on CPU or request rate. This can handle spikes (like heavy usage hours).
Cron Jobs: We have some cron jobs for backup, etc. (as described in backup doc). Those are deployed in K8s as CronJob resources (e.g., db-backup-cronjob runs pg_dump, etc.

). Ensure these are created in the cluster and monitored.
Ingress TLS: Use Let’s Encrypt or cloud-specific certs for app.Peripheral.com. The deployment includes either an annotation for cert-manager or a separate step to manage TLS. All user traffic is HTTPS.
The deployment pipeline and infrastructure ensure that Peripheral is delivered reliably to end-users with minimal downtime and a clear path from development to production. By following this guide, team members can understand how code progresses through environments and how to perform deployments or troubleshoot if something goes wrong during the release process.
.env.example
This is an example environment configuration for Peripheral. Do not commit actual secrets. Copy this file to .env (for local development) and fill in the values, or use it as reference for setting environment variables in your deployment environment (e.g., in Kubernetes secrets or CI). Database Configuration
ini
Copy
Edit
DB_HOST=localhost        # Hostname for PostgreSQL
DB_PORT=5432             # Port for PostgreSQL
DB_USER=postgres         # Database username
DB_PASSWORD=devpassword  # Database password
POSTGRES_DB=postgres     # Default database name
Cache (Redis) Configuration
makefile
Copy
Edit
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=          # (optional, if Redis is password-protected)
Auth & Security
ini
Copy
Edit
JWT_SECRET=changemeinprod123!        # Secret key for signing JWT tokens (set a long random string in production)
GOOGLE_CLIENT_ID=<your-google-oauth-client-id>     # Google OAuth 2.0 client ID (for login)
GOOGLE_CLIENT_SECRET=<your-google-oauth-secret>   # Google OAuth 2.0 client secret
GOOGLE_REDIRECT_URL=http://localhost:5173/auth/google/callback  # OAuth redirect (change for prod)
Email (SMTP) for notifications/password reset
ini
Copy
Edit
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USER=no-reply@example.com
SMTP_PASSWORD=<smtp-password>
EMAIL_FROM_ADDRESS="Peripheral Support <no-reply@example.com>"
External API Keys
ini
Copy
Edit
POLYGON_API_KEY=<your-polygon-api-key>      # Market data provider (Polygon.io) API key
ALPHA_VANTAGE_API_KEY=<your-alpha-vantage-key>  # (if another data source is used)
TWITTER_API_IO_KEY=<your-twitter-api-key>   # Twitter API key (if integrating with Twitter for news or sentiment)
X_API_KEY=<your-x-api-key>                 # X (Twitter) API key for v2 (if different from above)
X_API_SECRET=<your-x-api-secret>
X_ACCESS_TOKEN=<your-x-access-token>
X_ACCESS_SECRET=<your-x-access-secret>
(The above Twitter/X keys might be used for reading tweets or financial news sentiment.)
ini
Copy
Edit
GEMINI_API_KEY=<your-gpt-provider-key>     # API key for Gemini (internal AI service or OpenAI key if that's what Gemini refers to)
GEMINI_FREE_KEYS=<optional-free-tier-key>  # If there's a separate key or multiple keys for AI usage
GROK_API_KEY=<your-other-ai-key>           # API key for another AI service, if used (e.g., Grok)
Stripe (Payment) Keys (for subscription management):
ini
Copy
Edit
STRIPE_SECRET_KEY=<your-stripe-secret-key>           # Secret key for Stripe API (server side)
STRIPE_PUBLISHABLE_KEY=<your-stripe-publishable-key> # Publishable key (for frontend)
Application Settings
ini
Copy
Edit
VITE_ENVIRONMENT=development    # Used in frontend to differentiate dev/prod (development|staging|production)
BACKEND_URL=http://localhost:5058   # Frontend uses this to proxy or call backend in dev
Miscellaneous
ini
Copy
Edit
# If the project integrates any other third-party service, include their keys:
SENDGRID_API_KEY=<sendgrid-if-used>
SLACK_WEBHOOK_URL=<webhook-for-alerts>
TELEGRAM_BOT_TOKEN=<telegram-bot-token-for-alerts>
TELEGRAM_CHAT_ID=<telegram-chat-id-for-alerts>
Hot Reload (Dev only)
ini
Copy
Edit
HOT_RELOAD=true    # For the worker service to auto-restart on code changes (development only)
Notes:
In a production deployment, these values would be supplied via secure means (not a .env file). For example, Kubernetes secrets or a cloud secrets manager. This file is mainly for local development and as documentation of required config.
All keys/secrets here are placeholders. You must change JWT_SECRET in production (and ideally for dev as well, just keep it consistent across backend and any other service that needs to validate tokens).
Some variables are only needed in certain contexts:
OAuth Google keys only needed if enabling Google login.
SMTP needed if sending emails (invite codes, password resets, alerts via email).
Twitter/X keys only if pulling data from Twitter (or posting).
GEMINI/GROK keys are project-specific; ensure you know what service these refer to (could be OpenAI or other AI services).
VITE_PUBLIC_STRIPE_KEY might be used in the Svelte app (note: any var prefixed with VITE_ and especially VITE_PUBLIC_ is exposed to the frontend).
The BACKEND_URL is used by the dev frontend to route API calls. In production, this may not be needed if the frontend is served under same domain or uses relative API paths.
Double-check each variable and provide the correct values before running or deploying Peripheral. Missing or incorrect env vars are a common source of runtime errors (the backend will often fail to start if something like JWT_SECRET or DB creds are not set, by design).