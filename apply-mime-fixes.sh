#!/bin/bash
set -e

echo "Applying MIME type fixes for JavaScript modules..."

# Apply the ConfigMap with our improved server.js
kubectl apply -f server-configmap.yaml

# Create a patch file for the deployment
cat > frontend-patch.json << 'EOF'
{
  "spec": {
    "template": {
      "spec": {
        "volumes": [
          {
            "name": "server-js",
            "configMap": {
              "name": "frontend-server-js"
            }
          }
        ],
        "containers": [
          {
            "name": "frontend",
            "image": "billin19/frontend:prod",
            "volumeMounts": [
              {
                "name": "server-js",
                "mountPath": "/app/server.js",
                "subPath": "server.js"
              }
            ]
          }
        ]
      }
    }
  }
}
EOF

# Apply the patch to the deployment
kubectl patch deployment frontend --patch "$(cat frontend-patch.json)" --type=merge

# Restart the deployment to apply changes
kubectl rollout restart deployment frontend

echo "MIME type fixes applied. The deployment will restart automatically."
echo "Waiting for the new pod to be ready..."
kubectl get pods -w 