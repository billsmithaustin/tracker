#!/usr/bin/env bash
# setup-vm.sh — one-time setup on a fresh Compute Engine e2-micro VM
#
# Run this on the VM after provisioning (scripts/provision-gce.sh).
# Installs Docker + Docker Compose, clones the repo, and starts the tracker.
#
# Assumes:
#   - Debian 12 (bookworm)
#   - Your .env file is already in ~ (uploaded via gcloud compute scp)
#
# Usage (on the VM):
#   bash setup-vm.sh

set -euo pipefail

REPO_URL="${REPO_URL:-https://github.com/billsmithaustin/tracker.git}"
APP_DIR="$HOME/tracker"

echo "==> Installing Docker..."
if command -v docker &>/dev/null; then
  echo "    Docker already installed, skipping."
else
  sudo apt-get update -qq
  sudo apt-get install -y -qq ca-certificates curl gnupg

  sudo install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/debian/gpg \
    | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
  sudo chmod a+r /etc/apt/keyrings/docker.gpg

  echo \
    "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
    https://download.docker.com/linux/debian \
    $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
    | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

  sudo apt-get update -qq
  sudo apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-compose-plugin

  # Allow running docker without sudo
  sudo usermod -aG docker "$USER"
  echo "    Docker installed. NOTE: group change takes effect on next login."
fi

echo ""
echo "==> Cloning repo to $APP_DIR..."
if [[ -d "$APP_DIR" ]]; then
  echo "    Directory already exists, pulling latest..."
  git -C "$APP_DIR" pull --ff-only
else
  git clone "$REPO_URL" "$APP_DIR"
fi

echo ""
echo "==> Copying .env..."
if [[ -f "$HOME/.env" ]]; then
  cp "$HOME/.env" "$APP_DIR/.env"
  echo "    Copied ~/.env → $APP_DIR/.env"
elif [[ -f "$APP_DIR/.env" ]]; then
  echo "    $APP_DIR/.env already exists, leaving it alone."
else
  echo "    WARNING: no .env found in ~ or $APP_DIR."
  echo "    Copy it up before starting: gcloud compute scp .env INSTANCE:~"
  echo "    Then re-run this script."
  exit 1
fi

echo ""
echo "==> Authenticating Docker with Artifact Registry..."
gcloud auth configure-docker us-west1-docker.pkg.dev --quiet

echo ""
echo "==> Pulling images and starting the tracker..."
# Use sg so the docker group membership is active even if usermod was just applied.
sg docker -c "docker compose -f $APP_DIR/docker-compose.yml pull && docker compose -f $APP_DIR/docker-compose.yml up -d --no-build"

echo ""
echo "==> Enabling Docker to start on boot..."
sudo systemctl enable docker

echo ""
echo "==> Done! The tracker is running."
EXTERNAL_IP=$(curl -sf "http://metadata.google.internal/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip" -H "Metadata-Flavor: Google" || echo "<your-static-ip>")
echo "    http://$EXTERNAL_IP:8080"
echo ""
echo "To tail logs: docker compose -C $APP_DIR logs -f"
echo "To redeploy:  ./scripts/deploy.sh (from your local machine)"
