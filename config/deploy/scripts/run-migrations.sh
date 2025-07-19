#!/bin/bash
set -euo pipefail

echo "🗄️  Starting database migrations for environment: ${ENVIRONMENT}"

# Set up variables
MIGRATION_JOB_NAME="db-migrate-job-$(date +%s)"  # Unique name
MIGRATION_CONFIG="config/deploy/k8s/migration-job.yaml"
MIGRATION_TEMP="${TMP_DIR}/migration-job-${ENVIRONMENT}.yaml"

# Ensure temp directory exists
mkdir -p "${TMP_DIR}"

# Check if migration job template exists
if [ ! -f "${MIGRATION_CONFIG}" ]; then
    echo "❌ Migration job template not found at ${MIGRATION_CONFIG}"
    exit 1
fi

echo "📝 Preparing migration job configuration..."

# Substitute environment variables in the migration job template
envsubst '${K8S_NAMESPACE},${DOCKER_USERNAME},${DOCKER_TAG}' < "${MIGRATION_CONFIG}" > "${MIGRATION_TEMP}"

# Update the job name to be unique
sed -i "s/name: db-migrate-job/name: ${MIGRATION_JOB_NAME}/" "${MIGRATION_TEMP}"

echo "🚀 Deploying migration job: ${MIGRATION_JOB_NAME}"

# Apply the migration job
kubectl apply -f "${MIGRATION_TEMP}"

echo "⏳ Waiting for migration to complete..."

# Wait for the job to complete (with timeout)
if kubectl wait --for=condition=complete job/${MIGRATION_JOB_NAME} -n ${K8S_NAMESPACE} --timeout=7200s; then
    echo "✅ Database migration completed successfully!"
    
    # Show migration logs
    echo "📋 Migration logs:"
    kubectl logs job/${MIGRATION_JOB_NAME} -n ${K8S_NAMESPACE}
    
    # Clean up the completed job
    echo "🧹 Cleaning up migration job..."
    kubectl delete job ${MIGRATION_JOB_NAME} -n ${K8S_NAMESPACE} || true
    
else
    echo "❌ Migration failed or timed out!"
    
    # Show failure logs
    echo "📋 Migration failure logs:"
    kubectl logs job/${MIGRATION_JOB_NAME} -n ${K8S_NAMESPACE} || echo "Could not retrieve logs"
    
    # Show job status
    echo "📊 Job status:"
    kubectl describe job ${MIGRATION_JOB_NAME} -n ${K8S_NAMESPACE} || echo "Could not describe job"
    
    # Don't clean up failed job for debugging
    echo "🔍 Leaving failed job for debugging. Clean up manually with:"
    echo "kubectl delete job ${MIGRATION_JOB_NAME} -n ${K8S_NAMESPACE}"
    
    exit 1
fi

echo "🎉 Database migrations completed for ${ENVIRONMENT}!"