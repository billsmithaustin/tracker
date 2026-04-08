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

# 1. Reserve a static external IP (free while attached to a running instance)
echo "[1/4] Reserving static external IP '$STATIC_IP_NAME'..."
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

# 2. Create the VM
echo "[2/4] Creating instance '$INSTANCE_NAME'..."
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

# 3. Open firewall ports
echo "[3/4] Creating firewall rule '$FIREWALL_RULE_NAME' (ports 80, 443, 8080)..."
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

# 4. Print next steps
echo ""
echo "[4/4] Done."
echo ""
echo "Next steps:"
echo "  1. Copy your .env file to the VM:"
echo "       gcloud compute scp .env $INSTANCE_NAME:~ --zone=$ZONE --project=$PROJECT"
echo ""
echo "  2. SSH in and run the setup script:"
echo "       gcloud compute ssh $INSTANCE_NAME --zone=$ZONE --project=$PROJECT"
echo "       # then on the VM:"
echo "       bash <(curl -fsSL https://raw.githubusercontent.com/billsmithaustin/tracker/main/scripts/setup-vm.sh)"
echo "       # or copy the script up the same way you did .env and run it directly."
echo ""
echo "  3. The tracker will be at: http://$STATIC_IP:8080"
echo "     (see DEPLOY_OPTIONS.md for custom domain + SSL steps)"
