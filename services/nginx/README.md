# Nginx Configuration

This directory contains the Nginx configuration files for the Atlantis Trading application.

## Files

- `Dockerfile`: Builds the Nginx image with the configuration files
- `nginx.conf`: The main Nginx configuration file
- `mime.types`: MIME type definitions for serving static files

## Changes Made

The Nginx configuration has been moved from Kubernetes ConfigMaps into separate files to simplify the Kubernetes YAML files and make the configuration easier to manage. This approach has several benefits:

1. **Improved Readability**: Configuration is stored in its native format
2. **Better Tooling Support**: Editors can provide syntax highlighting and validation for Nginx configuration files
3. **Simplified Kubernetes Manifests**: Kubernetes YAML files are smaller and more focused
4. **Easier Maintenance**: Changes to the Nginx configuration can be made without modifying Kubernetes resources

## Deployment

The Nginx configuration is built into a Docker image and deployed to Kubernetes. The GitHub workflow has been updated to build and push the Nginx image.

## Related Kubernetes Resources

- `prod/config/nginx-deployment.yaml`: Deploys the Nginx container
- `prod/config/nginx-configmap.yaml`: Contains only the essential configuration parameters
- `prod/config/ingress.yaml`: Defines the Ingress rules for routing traffic

## Cleanup

Several redundant files have been removed from the `prod/config` directory:
- Removed `nginx.yaml` (replaced by `nginx-deployment.yaml`)
- Removed `app-ingress.yaml` (replaced by `ingress.yaml`)
- Removed `nginx-service.yaml` (service is now included in `nginx-deployment.yaml`)
- Removed `mime-config.yaml` (MIME types are now in `mime.types`)
- Removed redundant Nginx controller and RBAC files 