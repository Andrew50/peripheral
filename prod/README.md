# Production Deployment

This directory contains scripts and configuration files for deploying the application to production environments.

## Files

- `deploy-prod.sh`: Main deployment script that builds and pushes Docker images, then applies Kubernetes configurations
- `rollout`: Helper script for managing Kubernetes rollouts
- `config/`: Directory containing Kubernetes configuration files

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