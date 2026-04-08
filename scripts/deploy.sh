#!/usr/bin/env bash
# deploy.sh — push updates to the Compute Engine VM
#
# Run from your local machine after pushing commits to the repo.
# SSHes into the VM, pulls the latest code, and restarts the stack.
#
# Usage:
#   ./scripts/deploy.sh [INSTANCE_NAME] [ZONE] [PROJECT_ID]
#
# Defaults match what provision-gce.sh created.

set -euo pipefail

INSTANCE="${1:-tracker}"
ZONE="${2:-us-west1-a}"
PROJECT="${3:-$(gcloud config get-value project 2>/dev/null)}"

if [[ -z "$PROJECT" ]]; then
  echo "ERROR: no project set. Pass a project ID or run: gcloud config set project PROJECT_ID"
  exit 1
fi

APP_DIR="\$HOME/tracker"

echo "==> Deploying to $INSTANCE ($ZONE, $PROJECT)..."

gcloud compute ssh "$INSTANCE" \
  --zone="$ZONE" \
  --project="$PROJECT" \
  --command="
    set -euo pipefail
    echo '--- pulling latest code ---'
    git -C $APP_DIR pull --ff-only

    echo '--- rebuilding and restarting ---'
    docker compose -f $APP_DIR/docker-compose.yml up -d --build

    echo '--- removing old images ---'
    docker image prune -f

    echo '--- done ---'
    docker compose -f $APP_DIR/docker-compose.yml ps
  "

echo ""
echo "Deploy complete."
