# GitHub Actions Workflow for Kubernetes Deployment

This repository contains a GitHub Actions workflow for deploying applications to a Kubernetes cluster.

## Required GitHub Secrets

To use this workflow, you need to set up the following secrets in your GitHub repository:

1. `DOCKERHUB_USERNAME`: Your Docker Hub username
2. `DOCKERHUB_TOKEN`: Your Docker Hub access token (not your password)
3. `KUBE_CONFIG`: Your Kubernetes configuration file (base64 encoded)

### How to Set Up Docker Hub Secrets

1. Go to [Docker Hub](https://hub.docker.com/) and log in to your account
2. Click on your username in the top-right corner and select "Account Settings"
3. In the left sidebar, click on "Security"
4. Under "Access Tokens", click "New Access Token"
5. Give your token a name (e.g., "GitHub Actions") and select the appropriate permissions
6. Click "Generate" and copy the token that is displayed
7. In your GitHub repository, go to "Settings" > "Secrets and variables" > "Actions"
8. Click "New repository secret"
9. Create a secret named `DOCKERHUB_USERNAME` with your Docker Hub username
10. Create another secret named `DOCKERHUB_TOKEN` with the access token you generated

### How to Set Up Kubernetes Config Secret

1. On your local machine, locate your Kubernetes config file (usually at `~/.kube/config`)
2. Encode the file to base64:
   ```bash
   cat ~/.kube/config | base64
   ```
3. Copy the entire output
4. In your GitHub repository, go to "Settings" > "Secrets and variables" > "Actions"
5. Click "New repository secret"
6. Create a secret named `KUBE_CONFIG` with the base64-encoded config as the value

## Workflow Overview

The workflow performs the following steps:

1. Checks out the code
2. Sets up Docker Buildx
3. Logs in to Docker Hub
4. Builds and pushes Docker images for backend, frontend, and worker services
5. Sets up kubectl
6. Configures Kubernetes credentials
7. Updates image tags in Kubernetes manifests
8. Applies Kubernetes manifests in the correct order

## Troubleshooting

If you encounter the error "Username and password required" during the Docker login step, make sure you have correctly set up the `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` secrets in your GitHub repository. 