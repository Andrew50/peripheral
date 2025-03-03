# Database Security and Health Check Improvements

This repository contains improvements to database security and health monitoring for our Kubernetes-based application.

## Key Improvements

### 1. Removed Hardcoded Database Credentials

- Eliminated hardcoded database credentials from all configuration files
- Implemented environment variables for database connection parameters
- Created Kubernetes secrets for secure credential storage

### 2. Database Health Check System

We've implemented a comprehensive database health check system with the following components:

#### Database Check Script (`db_check.py`)

A Python script that verifies connectivity to both the database and Redis. This script:
- Uses environment variables for connection parameters
- Provides detailed error messages for connection failures
- Is used by the worker pod's init container to ensure services are available before startup

#### Health Check Sidecar Container

A dedicated low-resource container that continuously monitors database and Redis connectivity:
- Runs alongside the worker pods
- Implements adaptive backoff for failed connection attempts
- Consumes minimal resources (CPU/memory)
- Provides detailed logs for troubleshooting

## Deployment Changes

The deployment workflow has been updated to:
1. Build and push the new worker-healthcheck image
2. Create Kubernetes secrets with secure database credentials
3. Deploy the updated worker configuration with the health check sidecar

## Environment Variables

The following environment variables are now required:

| Variable | Description | Used In |
|----------|-------------|---------|
| DB_HOST | Database hostname | Worker, Health Check |
| DB_PORT | Database port | Worker, Health Check |
| DB_USER | Database username | Worker, Health Check |
| DB_PASSWORD | Database password | Worker, Health Check |
| REDIS_HOST | Redis hostname | Worker, Health Check |
| REDIS_PORT | Redis port | Worker, Health Check |
| HEALTHCHECK_INTERVAL | Interval between health checks (seconds) | Health Check |
| HEALTHCHECK_MAX_INTERVAL | Maximum backoff interval (seconds) | Health Check |

## GitHub Secrets

The following GitHub secrets must be configured:

- `DOCKER_USERNAME`: Docker Hub username
- `DOCKER_TOKEN`: Docker Hub access token
- `DB_PASSWORD`: Database password
- `REDIS_PASSWORD`: Redis password

## Local Development

For local development, use the provided `build_and_push.sh` script to build and push the Docker images:

```bash
# Set your Docker Hub username
export DOCKER_USERNAME=yourusername

# Build and push the images
./build_and_push.sh
```

## Troubleshooting

If worker pods fail to start:

1. Check the init container logs:
   ```
   kubectl logs <worker-pod-name> -c wait-for-db
   ```

2. Check the health check container logs:
   ```
   kubectl logs <worker-pod-name> -c db-healthcheck
   ```

3. Verify that the database and Redis services are running:
   ```
   kubectl get pods -l app=db
   kubectl get pods -l app=cache
   ```

4. Verify that the Kubernetes secrets are properly configured:
   ```
   kubectl describe secret db-secret
   kubectl describe secret redis-secret
   ``` 