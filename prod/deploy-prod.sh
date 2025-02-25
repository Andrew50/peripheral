#!/bin/bash
set -e

# Configuration - these are now set from GitHub secrets
REMOTE_HOST="ssh.atlantis.trading" # Will be replaced by the GitHub workflow
REMOTE_USER="aj" # Will be replaced by the GitHub workflow
REMOTE_DIR="/home/aj/dev/study"
BRANCH="prod"
DOCKER_USER="billin19" # Will be replaced by the GitHub workflow

# Function to log messages with timestamps
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

log "Starting production deployment process..."

# SSH into the remote server and execute deployment
# Using Cloudflare Access SSH
ssh ${REMOTE_USER}@${REMOTE_HOST} << EOF
    cd ${REMOTE_DIR}

    # Pull latest code
    log "Pulling latest code from ${BRANCH}..."
    git remote set-url origin git@github.com:Andrew50/study.git
    git checkout ${BRANCH}
    git pull origin ${BRANCH}

    # Build new Docker images
    log "Building Docker images..."
    docker build -t ${DOCKER_USER}/frontend:latest services/frontend
    docker build -t ${DOCKER_USER}/backend:latest services/backend
    docker build -t ${DOCKER_USER}/worker:latest services/worker
    docker build -t ${DOCKER_USER}/tf:latest services/tf
    docker build -t ${DOCKER_USER}/db:latest services/db

    # Push Docker images to registry
    log "Pushing Docker images to registry..."
    # Use Docker credentials from environment variables
    echo "\${DOCKER_TOKEN}" | docker login -u \${DOCKER_USER} --password-stdin
    
    docker push ${DOCKER_USER}/frontend:latest
    docker push ${DOCKER_USER}/backend:latest
    docker push ${DOCKER_USER}/worker:latest
    docker push ${DOCKER_USER}/tf:latest
    docker push ${DOCKER_USER}/db:latest

    # Apply Kubernetes configurations
    log "Applying Kubernetes configurations..."
    kubectl apply -f deployment/prod/config

    # Perform rolling updates for zero downtime
    log "Performing rolling updates for zero downtime..."
    
    # Update backend with zero downtime
    kubectl set image deployment/backend backend=${DOCKER_USER}/backend:latest
    kubectl rollout status deployment/backend
    
    # Update frontend with zero downtime
    kubectl set image deployment/frontend frontend=${DOCKER_USER}/frontend:latest
    kubectl rollout status deployment/frontend
    
    # Update worker pods with zero downtime
    kubectl set image deployment/worker worker=${DOCKER_USER}/worker:latest
    kubectl rollout status deployment/worker
    
    # Update tf pods with zero downtime
    kubectl set image deployment/tf tf=${DOCKER_USER}/tf:latest
    kubectl rollout status deployment/tf
    
    log "Deployment completed successfully!"
EOF

log "Remote deployment process initiated. Check server logs for details." 