#!/usr/bin/env bash
# setup-vm.sh — one-time setup on a fresh Compute Engine e2-micro VM
#
# Run this on the VM after provisioning (scripts/provision-gce.sh).
# Installs Docker + Docker Compose, clones the repo, and starts the tracker.
#
# Assumes:
#   - Debian 12 (bookworm)
#   - Your .env file is already in ~ (uploaded via gcloud compute scp)
#   - If DOMAIN is set in .env, DNS A record already points to this VM's IP
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
echo "==> Pulling images..."
sg docker -c "docker compose -f $APP_DIR/docker-compose.yml pull"

# Read optional SSL config from .env
DOMAIN=$(grep -E '^DOMAIN=' "$APP_DIR/.env" 2>/dev/null | cut -d= -f2 || true)
CERTBOT_EMAIL=$(grep -E '^CERTBOT_EMAIL=' "$APP_DIR/.env" 2>/dev/null | cut -d= -f2 || true)

if [[ -n "$DOMAIN" && -n "$CERTBOT_EMAIL" ]]; then
  echo ""
  echo "==> Installing Certbot for $DOMAIN..."
  sudo apt-get install -y -qq python3 python3-venv libaugeas0
  if [[ ! -f /usr/bin/certbot ]]; then
    sudo python3 -m venv /opt/certbot/
    sudo /opt/certbot/bin/pip install --upgrade pip --quiet
    sudo /opt/certbot/bin/pip install certbot --quiet
    sudo ln -s /opt/certbot/bin/certbot /usr/bin/certbot
  fi

  if [[ ! -f "/etc/letsencrypt/live/$DOMAIN/fullchain.pem" ]]; then
    echo "==> Requesting SSL certificate (briefly starting standalone server on port 80)..."
    sudo certbot certonly --standalone -d "$DOMAIN" \
      --non-interactive --agree-tos -m "$CERTBOT_EMAIL"
  else
    echo "    Certificate already exists, skipping."
  fi

  echo ""
  echo "==> Starting tracker with SSL..."
  sg docker -c "docker compose -f $APP_DIR/docker-compose.yml -f $APP_DIR/docker-compose.prod.yml up -d --no-build"

  echo ""
  echo "==> Installing cert renewal cron job..."
  CRON_CMD="0 3 * * * certbot renew --standalone --pre-hook \"docker compose -f $APP_DIR/docker-compose.yml -f $APP_DIR/docker-compose.prod.yml down\" --post-hook \"docker compose -f $APP_DIR/docker-compose.yml -f $APP_DIR/docker-compose.prod.yml up -d --no-build\""
  EXISTING_CRON=$(sudo crontab -l 2>/dev/null || true)
  (echo "$EXISTING_CRON" | grep -v certbot || true; echo "$CRON_CMD") | sudo crontab -
  echo "    Renewal cron installed."
else
  echo ""
  echo "==> Starting tracker (no DOMAIN set, HTTP only)..."
  sg docker -c "docker compose -f $APP_DIR/docker-compose.yml up -d --no-build"
fi

echo ""
echo "==> Enabling Docker to start on boot..."
sudo systemctl enable docker

echo ""
echo "==> Done! The tracker is running."
EXTERNAL_IP=$(curl -sf "http://metadata.google.internal/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip" -H "Metadata-Flavor: Google" || echo "<your-static-ip>")
if [[ -n "$DOMAIN" ]]; then
  echo "    https://$DOMAIN"
else
  echo "    http://$EXTERNAL_IP"
fi
