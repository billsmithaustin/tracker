#!/usr/bin/env bash
# provision-gce.sh — create a free-tier Compute Engine e2-micro VM for the tracker
#
# Run this once from your local machine (requires gcloud CLI + auth).
# After it completes, SSH in and run scripts/setup-vm.sh.
#
# Usage:
#   ./scripts/provision-gce.sh [PROJECT_ID]
#
# If PROJECT_ID is omitted, uses the current gcloud default project.

set -euo pipefail

PROJECT="${1:-$(gcloud config get-value project 2>/dev/null)}"
if [[ -z "$PROJECT" ]]; then
  echo "ERROR: no project set. Pass a project ID or run: gcloud config set project PROJECT_ID"
  exit 1
fi

# --- config (edit these if needed) ---
INSTANCE_NAME="tracker"
ZONE="us-west1-a"          # us-west1 / us-central1 / us-east1 are free-tier eligible
REGION="${ZONE%-*}"
MACHINE_TYPE="e2-micro"
DISK_SIZE="30GB"
DISK_TYPE="pd-standard"
IMAGE_FAMILY="debian-12"
IMAGE_PROJECT="debian-cloud"
STATIC_IP_NAME="tracker-ip"
FIREWALL_RULE_NAME="allow-tracker"
# --------------------------------------

echo "==> Project : $PROJECT"
echo "==> Instance: $INSTANCE_NAME ($MACHINE_TYPE, $ZONE)"
echo ""

# 1. Enable required APIs
echo "[1/5] Enabling required APIs..."
gcloud services enable compute.googleapis.com artifactregistry.googleapis.com \
  --project="$PROJECT" --quiet

# 2. Set up Artifact Registry
AR_REPO="tracker"
AR_LOCATION="us-west1"
echo "[2/5] Setting up Artifact Registry repository '$AR_REPO'..."
if gcloud artifacts repositories describe "$AR_REPO" \
    --location="$AR_LOCATION" --project="$PROJECT" &>/dev/null; then
  echo "      (already exists, skipping)"
else
  gcloud artifacts repositories create "$AR_REPO" \
    --repository-format=docker \
    --location="$AR_LOCATION" \
    --project="$PROJECT" \
    --description="Tracker Docker images"
fi
echo "      Configuring local Docker auth..."
gcloud auth configure-docker "${AR_LOCATION}-docker.pkg.dev" --quiet

# 3. Reserve a static external IP (free while attached to a running instance)
echo "[3/5] Reserving static external IP '$STATIC_IP_NAME'..."
if gcloud compute addresses describe "$STATIC_IP_NAME" --region="$REGION" --project="$PROJECT" &>/dev/null; then
  echo "      (already exists, skipping)"
else
  gcloud compute addresses create "$STATIC_IP_NAME" \
    --region="$REGION" \
    --project="$PROJECT"
fi

STATIC_IP=$(gcloud compute addresses describe "$STATIC_IP_NAME" \
  --region="$REGION" \
  --project="$PROJECT" \
  --format="value(address)")
echo "      IP: $STATIC_IP"

# 4. Create the VM
echo "[4/5] Creating instance '$INSTANCE_NAME'..."
if gcloud compute instances describe "$INSTANCE_NAME" --zone="$ZONE" --project="$PROJECT" &>/dev/null; then
  echo "      (already exists, skipping)"
else
  gcloud compute instances create "$INSTANCE_NAME" \
    --project="$PROJECT" \
    --zone="$ZONE" \
    --machine-type="$MACHINE_TYPE" \
    --boot-disk-size="$DISK_SIZE" \
    --boot-disk-type="$DISK_TYPE" \
    --image-family="$IMAGE_FAMILY" \
    --image-project="$IMAGE_PROJECT" \
    --address="$STATIC_IP_NAME" \
    --tags="tracker-server" \
    --metadata="enable-oslogin=TRUE"
fi

# 5. Open firewall ports
echo "[5/5] Creating firewall rule '$FIREWALL_RULE_NAME' (ports 80, 443, 8080)..."
if gcloud compute firewall-rules describe "$FIREWALL_RULE_NAME" --project="$PROJECT" &>/dev/null; then
  echo "      (already exists, skipping)"
else
  gcloud compute firewall-rules create "$FIREWALL_RULE_NAME" \
    --project="$PROJECT" \
    --direction=INGRESS \
    --action=ALLOW \
    --rules=tcp:80,tcp:443,tcp:8080 \
    --source-ranges=0.0.0.0/0 \
    --target-tags="tracker-server" \
    --description="Allow HTTP/HTTPS/8080 traffic for the trip tracker"
fi

echo ""
echo "Done."
echo ""
echo "Next steps:"
echo "  1. Add PROJECT_ID to your .env file:"
echo "       echo 'PROJECT_ID=$PROJECT' >> .env"
echo ""
echo "  2. Build and push images to Artifact Registry:"
echo "       make push"
echo ""
echo "  3. Copy your .env to the VM and run setup:"
echo "       gcloud compute scp .env $INSTANCE_NAME:~ --zone=$ZONE --project=$PROJECT"
echo "       gcloud compute ssh $INSTANCE_NAME --zone=$ZONE --project=$PROJECT"
echo "       # then on the VM:"
echo "       bash <(curl -fsSL https://raw.githubusercontent.com/billsmithaustin/tracker/main/scripts/setup-vm.sh)"
echo ""
echo "  4. The tracker will be at: http://$STATIC_IP:8080"
echo "     (see DEPLOY_OPTIONS.md for custom domain + SSL steps)"
