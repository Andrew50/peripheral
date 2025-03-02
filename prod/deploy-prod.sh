#!/bin/bash
set -e

# Configuration - these are now set from GitHub secrets or passed from workflow
REMOTE_HOST="ssh.atlantis.trading" # Will be replaced by the GitHub workflow
REMOTE_USER="aj" # Will be replaced by the GitHub workflow
REMOTE_DIR="/home/aj/dev/study"
BRANCH="${GITHUB_REF_NAME:-prod}" # Get branch name from GitHub Actions, default to prod if not set
DOCKER_USER="billin19" # Will be replaced by the GitHub workflow
DOCKER_TAG="${BRANCH}" # Use branch name as the Docker tag

# Function to log messages with timestamps
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

log "Starting deployment process for branch: ${BRANCH}..."

# SSH into the remote server and execute deployment
# Using Cloudflare Access SSH
ssh ${REMOTE_USER}@${REMOTE_HOST} << EOF
    cd ${REMOTE_DIR}

    # Pull latest code
    log "Pulling latest code from ${BRANCH}..."
    git remote set-url origin git@github.com:Andrew50/study.git
    git checkout ${BRANCH}
    git pull origin ${BRANCH}

    # Build new Docker images with branch-specific tag
    log "Building Docker images for ${BRANCH}..."
    docker build -t ${DOCKER_USER}/frontend:${DOCKER_TAG} services/frontend
    docker build -t ${DOCKER_USER}/backend:${DOCKER_TAG} services/backend
    docker build -t ${DOCKER_USER}/worker:${DOCKER_TAG} services/worker
    docker build -t ${DOCKER_USER}/tf:${DOCKER_TAG} services/tf
    docker build -t ${DOCKER_USER}/db:${DOCKER_TAG} services/db

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
    # Use Docker credentials from environment variables
    echo "\${DOCKER_TOKEN}" | docker login -u \${DOCKER_USER} --password-stdin
    
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
        if [ -d "deployment/dev/config" ]; then
            kubectl apply -f deployment/dev/config
        else
            # Fall back to prod config if no dev config exists
            kubectl apply -f deployment/prod/config
        fi
    else
        # Default to prod config for prod branch or any other branch
        kubectl apply -f deployment/prod/config
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
EOF

log "Remote deployment process initiated for branch: ${BRANCH}. Check server logs for details." 