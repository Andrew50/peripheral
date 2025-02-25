#!/bin/bash
set -e

echo "Applying MIME type fixes for JavaScript modules..."

# Rebuild and redeploy the frontend
cd frontend
docker build -t frontend-app:latest -f Dockerfile.prod .
kubectl rollout restart deployment frontend

echo "MIME type fixes applied. Wait for the pods to restart..."
kubectl get pods -w 