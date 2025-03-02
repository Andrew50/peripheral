#!/bin/bash
set -e

# This script ensures that all required Kubernetes configuration files exist
# Usage: ./ensure-config-files.sh <dockerhub_username> <db_root_password>

DOCKERHUB_USERNAME=$1
DB_ROOT_PASSWORD=$2

# Ensure prod/config directory exists
mkdir -p prod/config

# Create db.yaml if it doesn't exist
if [ ! -f "prod/config/db.yaml" ]; then
  echo "Creating db.yaml..."
  cat > prod/config/db.yaml << EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: db
spec:
  replicas: 1
  selector:
    matchLabels:
      app: db
  template:
    metadata:
      labels:
        app: db
    spec:
      containers:
        - name: db
          image: postgres:15
          ports:
            - containerPort: 5432
          volumeMounts:
            - name: db-persistent-storage
              mountPath: /var/lib/postgresql/data
          env:
            - name: POSTGRES_PASSWORD
              value: "${DB_ROOT_PASSWORD}"
            - name: POSTGRES_DB
              value: "app_db"
          resources:
            requests:
              cpu: 500m
              memory: 512Mi
            limits:
              cpu: 1000m
              memory: 1024Mi
      volumes:
        - name: db-persistent-storage
          persistentVolumeClaim:
            claimName: db-pvc
---
apiVersion: v1
kind: Service
metadata:
  name: db
spec:
  selector:
    app: db
  ports:
    - protocol: TCP
      port: 5432
      targetPort: 5432
  type: ClusterIP
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: db-pvc
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
EOF
fi

# Create cache.yaml if it doesn't exist
if [ ! -f "prod/config/cache.yaml" ]; then
  echo "Creating cache.yaml..."
  cat > prod/config/cache.yaml << EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cache
spec:
  replicas: 1
  selector:
    matchLabels:
      app: cache
  template:
    metadata:
      labels:
        app: cache
    spec:
      containers:
        - name: redis
          image: redis:7-alpine
          ports:
            - containerPort: 6379
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 200m
              memory: 256Mi
---
apiVersion: v1
kind: Service
metadata:
  name: cache
spec:
  selector:
    app: cache
  ports:
    - protocol: TCP
      port: 6379
      targetPort: 6379
  type: ClusterIP
EOF
fi

# Create backend.yaml if it doesn't exist
if [ ! -f "prod/config/backend.yaml" ]; then
  echo "Creating backend.yaml..."
  cat > prod/config/backend.yaml << EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: backend
spec:
  replicas: 2
  selector:
    matchLabels:
      app: backend
  template:
    metadata:
      labels:
        app: backend
    spec:
      containers:
        - name: backend
          image: ${DOCKERHUB_USERNAME}/backend:latest
          ports:
            - containerPort: 8000
          env:
            - name: DB_HOST
              value: "db"
            - name: DB_PORT
              value: "5432"
            - name: DB_NAME
              value: "app_db"
            - name: DB_USER
              value: "postgres"
            - name: DB_PASSWORD
              value: "${DB_ROOT_PASSWORD}"
            - name: REDIS_HOST
              value: "cache"
            - name: REDIS_PORT
              value: "6379"
          resources:
            requests:
              cpu: 200m
              memory: 256Mi
            limits:
              cpu: 500m
              memory: 512Mi
---
apiVersion: v1
kind: Service
metadata:
  name: backend
spec:
  selector:
    app: backend
  ports:
    - protocol: TCP
      port: 8000
      targetPort: 8000
  type: ClusterIP
EOF
fi

# Create worker.yaml if it doesn't exist
if [ ! -f "prod/config/worker.yaml" ]; then
  echo "Creating worker.yaml..."
  cat > prod/config/worker.yaml << EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: worker
spec:
  replicas: 1
  selector:
    matchLabels:
      app: worker
  template:
    metadata:
      labels:
        app: worker
    spec:
      containers:
        - name: worker
          image: ${DOCKERHUB_USERNAME}/worker:latest
          env:
            - name: DB_HOST
              value: "db"
            - name: DB_PORT
              value: "5432"
            - name: DB_NAME
              value: "app_db"
            - name: DB_USER
              value: "postgres"
            - name: DB_PASSWORD
              value: "${DB_ROOT_PASSWORD}"
            - name: REDIS_HOST
              value: "cache"
            - name: REDIS_PORT
              value: "6379"
          resources:
            requests:
              cpu: 200m
              memory: 256Mi
            limits:
              cpu: 500m
              memory: 512Mi
EOF
fi

# Create frontend.yaml if it doesn't exist
if [ ! -f "prod/config/frontend.yaml" ]; then
  echo "Creating frontend.yaml..."
  cat > prod/config/frontend.yaml << EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: frontend
spec:
  replicas: 2
  selector:
    matchLabels:
      app: frontend
  template:
    metadata:
      labels:
        app: frontend
    spec:
      containers:
        - name: frontend
          image: ${DOCKERHUB_USERNAME}/frontend:latest
          ports:
            - containerPort: 80
          env:
            - name: BACKEND_URL
              value: "http://backend:8000"
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 200m
              memory: 256Mi
---
apiVersion: v1
kind: Service
metadata:
  name: frontend
spec:
  selector:
    app: frontend
  ports:
    - protocol: TCP
      port: 80
      targetPort: 80
  type: ClusterIP
EOF
fi

# Create nginx.yaml if it doesn't exist
if [ ! -f "prod/config/nginx.yaml" ]; then
  echo "Creating nginx.yaml..."
  cat > prod/config/nginx.yaml << EOF
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: app-ingress
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
spec:
  rules:
  - host: atlantis.trading
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: frontend
            port:
              number: 80
      - path: /api
        pathType: Prefix
        backend:
          service:
            name: backend
            port:
              number: 8000
EOF
fi

echo "All required configuration files have been created or verified." 