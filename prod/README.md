# Production Deployment

This directory contains scripts and configuration files for deploying the application to production environments.

## Files

- `deploy-prod.sh`: Main deployment script that builds and pushes Docker images, then applies Kubernetes configurations
- `rollout`: Helper script for managing Kubernetes rollouts
- `config/`: Directory containing Kubernetes configuration files
- `apply-fixes.sh`: Script to fix deployment issues and ensure proper configuration

## Usage

The deployment process is typically handled by GitHub Actions workflows, but you can also run the deployment script manually:

```bash
# Set required environment variables
export DOCKER_USER=your_docker_username
export DOCKER_TOKEN=your_docker_token
export GITHUB_REF_NAME=branch_name

# Run the deployment script
./deploy-prod.sh
```

## Troubleshooting

If you encounter issues with the deployment script not being found in GitHub Actions, ensure that:

1. The `prod` directory is properly committed to the repository
2. The GitHub Actions workflow is checking out the correct branch
3. The deployment script has executable permissions (`chmod +x deploy-prod.sh`)

## Fixing Deployment Issues

If you encounter issues with deployments, you can use the `apply-fixes.sh` script to fix common problems:

```bash
# Apply fixes using prod branch images (default)
./apply-fixes.sh

# Or specify a branch to use those images
./apply-fixes.sh prod  # Use production images
./apply-fixes.sh dev   # Use development images

# You can also override the Docker username
export DOCKER_USERNAME=your_docker_username
./apply-fixes.sh dev

# For cloudflared, you can specify your tunnel token
export CLOUDFLARE_TUNNEL_TOKEN=your-tunnel-token
./apply-fixes.sh
```

The script will:

1. Update the image tags in the deployment files
2. Process any environment variables in the YAML files
3. Fix the Redis cache deployment to ensure proper configuration
4. Fix the database deployment to ensure proper configuration
5. Fix the TensorFlow deployment to include the required configuration files
6. Fix the worker deployment to improve Redis connection handling
7. Fix the backend, frontend, nginx, and cloudflared deployments
8. Check the status of all deployments and pods

### Environment Variables

The script supports the following environment variables:

- `DOCKER_USERNAME`: The Docker username for image references (default: billin19)
- `IMAGE_TAG`: Automatically set based on the branch parameter
- `NGINX_IMAGE`: The Nginx image to use (default: k8s.gcr.io/ingress-nginx/controller:v1.2.1)
- `CLOUDFLARED_IMAGE`: The Cloudflared image to use (default: cloudflare/cloudflared:2023.8.0)
- `CLOUDFLARE_TUNNEL_TOKEN`: Your Cloudflare tunnel token (required for cloudflared)

If your Kubernetes YAML files use these variables (e.g., `${DOCKER_USERNAME}` or `${IMAGE_TAG}`), they will be automatically replaced with the appropriate values. 