#!/bin/bash
# hack/create-cluster.sh - Create a GKE Autopilot cluster and configure kubectl
# This script creates a GKE Autopilot cluster in the current gcloud project
# and configures the local kubectl context to use it.

set -euo pipefail

# Configurable variables with defaults
CLUSTER_NAME=${CLUSTER_NAME:-"scion-agents"}
REGION=${REGION:-"us-central1"}
PROJECT_ID=${PROJECT_ID:-$(gcloud config get-value project 2>/dev/null)}

if [[ -z "$PROJECT_ID" ]]; then
    echo "Error: PROJECT_ID is not set and could not be determined from gcloud config."
    exit 1
fi

echo "=== Creating GKE Autopilot Cluster ==="
echo "Cluster Name: ${CLUSTER_NAME}"
echo "Region:       ${REGION}"
echo "Project:      ${PROJECT_ID}"

# Create the cluster if it doesn't exist
if ! gcloud container clusters describe "${CLUSTER_NAME}" --region "${REGION}" --project "${PROJECT_ID}" >/dev/null 2>&1; then
    echo "Creating cluster (this may take several minutes)..."
    gcloud container clusters create-auto "${CLUSTER_NAME}" \
        --region "${REGION}" \
        --project "${PROJECT_ID}"
else
    echo "Cluster '${CLUSTER_NAME}' already exists."
fi

echo "=== Configuring kubectl authentication ==="
gcloud container clusters get-credentials "${CLUSTER_NAME}" \
    --region "${REGION}" \
    --project "${PROJECT_ID}"

echo ""
echo "=== Success ==="
echo "You can now use 'kubectl' to interact with your GKE Autopilot cluster."
