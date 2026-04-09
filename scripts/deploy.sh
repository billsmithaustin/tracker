#!/usr/bin/env bash
# deploy.sh — build images locally, push to Artifact Registry, restart on the VM
#
# Run from your local machine after pushing commits to the repo.
#
# Usage:
#   ./scripts/deploy.sh [INSTANCE_NAME] [ZONE] [PROJECT_ID]
#
# Defaults match what provision-gce.sh created.
# PROJECT_ID can also be set in .env.

set -euo pipefail

INSTANCE="${1:-tracker}"
ZONE="${2:-us-west1-a}"

# Load PROJECT_ID from .env if not passed and not already set
if [[ -z "${PROJECT_ID:-}" ]]; then
  if [[ -f ".env" ]]; then
    PROJECT_ID=$(grep -E '^PROJECT_ID=' .env | cut -d= -f2)
  fi
fi
PROJECT_ID="${3:-${PROJECT_ID:-$(gcloud config get-value project 2>/dev/null)}}"

if [[ -z "$PROJECT_ID" ]]; then
  echo "ERROR: PROJECT_ID not set. Add it to .env or pass it as the third argument."
  exit 1
fi

echo "==> Pulling and restarting on $INSTANCE ($ZONE)..."
gcloud compute ssh "$INSTANCE" \
  --zone="$ZONE" \
  --project="$PROJECT_ID" \
  --command="
    set -euo pipefail
    cd \$HOME/tracker
    git pull --ff-only
    docker compose pull
    docker compose up -d --no-build
    docker image prune -f
    docker compose ps
  "

echo ""
echo "Deploy complete."
