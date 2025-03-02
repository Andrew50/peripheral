#!/bin/bash
set -e

# Configuration - these are now set from GitHub secrets or passed from workflow
DOCKER_USER="${DOCKER_USER:-billin19}" # Will be replaced by the GitHub workflow
BRANCH="${GITHUB_REF_NAME:-prod}" # Get branch name from GitHub Actions, default to prod if not set

# Sanitize branch name for Docker tags (replace / with - and other invalid characters)
DOCKER_TAG=$(echo "${BRANCH}" | sed 's/\//-/g' | sed 's/[^a-zA-Z0-9_.-]/-/g')

# Function to log messages with timestamps
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

# Function to log errors
error_log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: $1" >&2
}

# Print diagnostic information
log "Deployment script diagnostics:"
log "Current working directory: $(pwd)"
log "Current user: $(whoami)"
log "Current branch: ${BRANCH}"
log "Docker tag: ${DOCKER_TAG}"
log "Files in current directory: $(ls -la)"

log "Starting deployment process for branch: ${BRANCH}..."

# Validate that we have the required variables
if [ -z "${BRANCH}" ]; then
    error_log "Branch name is not set. Please check GITHUB_REF_NAME or set a default branch."
    exit 1
fi

# Run deployment steps directly on the runner
log "Executing deployment process on runner for branch: ${BRANCH}"

set -e  # Exit immediately if a command exits with a non-zero status
echo "Running deployment as $(whoami) in directory $(pwd)"

# For GitHub Actions, the code is already checked out, so we can skip the git operations
log "Using already checked out code for deployment..."

# Check if we're in a GitHub Actions environment
if [ -n "${GITHUB_ACTIONS}" ]; then
    log "Running in GitHub Actions environment, skipping git operations..."
else
    # Only perform git operations if not in GitHub Actions
    log "Not in GitHub Actions, performing git operations..."
    
    # Use HTTPS instead of SSH to avoid password prompts
    git remote set-url origin https://github.com/Andrew50/study.git || { error_log "Failed to set git remote URL"; exit 1; }

    # Fetch all branches to ensure we have the reference
    echo "Fetching all branches to ensure ${BRANCH} exists..."
    git fetch --all || { error_log "Failed to fetch all branches"; exit 1; }

    # Check if branch exists
    if git show-ref --verify --quiet refs/remotes/origin/${BRANCH}; then
        echo "Branch ${BRANCH} found in remote"
    else
        error_log "Branch ${BRANCH} not found in remote. Available branches:"
        git branch -a
        exit 1
    fi

    # Checkout and pull
    git checkout ${BRANCH} || { error_log "Failed to checkout branch ${BRANCH}"; exit 1; }
    git pull origin ${BRANCH} || { error_log "Failed to pull latest code from branch ${BRANCH}"; exit 1; }
fi

# Build new Docker images with branch-specific tag
log "Building Docker images with tag: ${DOCKER_TAG}..."
docker build -t ${DOCKER_USER}/frontend:${DOCKER_TAG} -f frontend/Dockerfile.prod frontend
docker build -t ${DOCKER_USER}/backend:${DOCKER_TAG} -f backend/Dockerfile.prod backend
docker build -t ${DOCKER_USER}/worker:${DOCKER_TAG} -f worker/Dockerfile.prod worker
docker build -t ${DOCKER_USER}/tf:${DOCKER_TAG} -f tf/Dockerfile.prod tf
docker build -t ${DOCKER_USER}/db:${DOCKER_TAG} -f db/Dockerfile.prod db

# For prod branch, also tag as latest
# For dev branch, also tag as development
if [ "${BRANCH}" = "prod" ]; then
    log "Tagging images as 'latest' for production..."
    docker tag ${DOCKER_USER}/frontend:${DOCKER_TAG} ${DOCKER_USER}/frontend:latest
    docker tag ${DOCKER_USER}/backend:${DOCKER_TAG} ${DOCKER_USER}/backend:latest
    docker tag ${DOCKER_USER}/worker:${DOCKER_TAG} ${DOCKER_USER}/worker:latest
    docker tag ${DOCKER_USER}/tf:${DOCKER_TAG} ${DOCKER_USER}/tf:latest
    docker tag ${DOCKER_USER}/db:${DOCKER_TAG} ${DOCKER_USER}/db:latest
elif [ "${BRANCH}" = "dev" ]; then
    log "Tagging images as 'development' for development environment..."
    docker tag ${DOCKER_USER}/frontend:${DOCKER_TAG} ${DOCKER_USER}/frontend:development
    docker tag ${DOCKER_USER}/backend:${DOCKER_TAG} ${DOCKER_USER}/backend:development
    docker tag ${DOCKER_USER}/worker:${DOCKER_TAG} ${DOCKER_USER}/worker:development
    docker tag ${DOCKER_USER}/tf:${DOCKER_TAG} ${DOCKER_USER}/tf:development
    docker tag ${DOCKER_USER}/db:${DOCKER_TAG} ${DOCKER_USER}/db:development
fi

# Push Docker images to registry
log "Pushing Docker images to registry with tag: ${DOCKER_TAG}..."

# Debug information about environment variables (without revealing secrets)
log "Docker user: ${DOCKER_USER}"
log "Docker token status: $(if [ -n "${DOCKER_TOKEN}" ]; then echo "is set"; else echo "is NOT set"; fi)"
log "GitHub environment: $(if [ -n "${GITHUB_ACTIONS}" ]; then echo "Running in GitHub Actions"; else echo "Not running in GitHub Actions"; fi)"

# Check if Docker user is set
if [ -z "${DOCKER_USER}" ]; then
    error_log "DOCKER_USER environment variable is not set. Please set it before running this script."
    error_log "For GitHub Actions, ensure the secret DOCKER_USERNAME is properly configured in your repository settings."
    error_log "Repository Settings > Secrets and variables > Actions > Repository secrets"
    exit 1
fi

# Check if Docker token is set
if [ -z "${DOCKER_TOKEN}" ]; then
    error_log "DOCKER_TOKEN environment variable is not set. Please set it before running this script."
    error_log "For GitHub Actions, ensure the secret DOCKER_TOKEN is properly configured in your repository settings."
    error_log "Repository Settings > Secrets and variables > Actions > Repository secrets"
    
    # If in GitHub Actions, provide more specific guidance
    if [ -n "${GITHUB_ACTIONS}" ]; then
        error_log "This script is running in GitHub Actions but DOCKER_TOKEN is not set."
        error_log "Please add the DOCKER_TOKEN secret in your GitHub repository:"
        error_log "1. Go to your repository on GitHub"
        error_log "2. Navigate to Settings > Secrets and variables > Actions"
        error_log "3. Click 'New repository secret'"
        error_log "4. Name: DOCKER_TOKEN"
        error_log "5. Value: Your Docker Hub access token"
        error_log "6. Click 'Add secret'"
    fi
    
    # Check if we're in an interactive environment and offer manual login
    if [ -t 0 ]; then
        read -p "Do you want to log in to Docker manually? (y/n): " manual_login
        if [ "$manual_login" = "y" ] || [ "$manual_login" = "Y" ]; then
            log "Attempting manual Docker login..."
            docker login -u ${DOCKER_USER} || {
                error_log "Manual Docker login failed. Please check your credentials."
                exit 1
            }
            log "Manual Docker login successful, proceeding with image push..."
        else
            error_log "Aborting deployment due to missing Docker token."
            exit 1
        fi
    else
        error_log "Not in an interactive terminal and DOCKER_TOKEN is not set. Cannot proceed with deployment."
        exit 1
    fi
else
    # Perform Docker login with the token
    log "Attempting Docker login with token..."
    echo "${DOCKER_TOKEN}" | docker login -u ${DOCKER_USER} --password-stdin || {
        error_log "Docker login failed. Please check your credentials."
        error_log "If you're using GitHub Actions, verify that the DOCKER_TOKEN secret contains a valid Docker Hub access token."
        exit 1
    }
    log "Docker login successful, proceeding with image push..."
fi

docker push ${DOCKER_USER}/frontend:${DOCKER_TAG}
docker push ${DOCKER_USER}/backend:${DOCKER_TAG}
docker push ${DOCKER_USER}/worker:${DOCKER_TAG}
docker push ${DOCKER_USER}/tf:${DOCKER_TAG}
docker push ${DOCKER_USER}/db:${DOCKER_TAG}

# Push additional tags based on branch
if [ "${BRANCH}" = "prod" ]; then
    log "Pushing 'latest' tagged images..."
    docker push ${DOCKER_USER}/frontend:latest
    docker push ${DOCKER_USER}/backend:latest
    docker push ${DOCKER_USER}/worker:latest
    docker push ${DOCKER_USER}/tf:latest
    docker push ${DOCKER_USER}/db:latest
elif [ "${BRANCH}" = "dev" ]; then
    log "Pushing 'development' tagged images..."
    docker push ${DOCKER_USER}/frontend:development
    docker push ${DOCKER_USER}/backend:development
    docker push ${DOCKER_USER}/worker:development
    docker push ${DOCKER_USER}/tf:development
    docker push ${DOCKER_USER}/db:development
fi

# Apply Kubernetes configurations - you might want different config for dev vs prod
log "Applying Kubernetes configurations..."
if [ "${BRANCH}" = "dev" ]; then
    # Dev-specific configurations (if they exist)
    if [ -d "prod/config/dev" ]; then
        kubectl apply -f prod/config/dev
    else
        # Fall back to prod config if no dev config exists
        kubectl apply -f prod/config
    fi
else
    # Default to prod config for prod branch or any other branch
    kubectl apply -f prod/config
fi

# Perform rolling updates for zero downtime using branch-specific image tags
log "Performing rolling updates for zero downtime using images tagged with: ${DOCKER_TAG}..."

# Update deployments with zero downtime
kubectl set image deployment/backend backend=${DOCKER_USER}/backend:${DOCKER_TAG}
kubectl rollout status deployment/backend

kubectl set image deployment/frontend frontend=${DOCKER_USER}/frontend:${DOCKER_TAG}
kubectl rollout status deployment/frontend

kubectl set image deployment/worker worker=${DOCKER_USER}/worker:${DOCKER_TAG}
kubectl rollout status deployment/worker

kubectl set image deployment/tf tf=${DOCKER_USER}/tf:${DOCKER_TAG}
kubectl rollout status deployment/tf

log "Deployment of branch ${BRANCH} completed successfully!" 