#!/usr/bin/env bash
# Update vendored frontend dependencies (Leaflet, Chart.js).
# Edit the version variables below to upgrade a library, then run: make vendor

set -euo pipefail

LEAFLET_VERSION="1.9.4"
CHARTJS_VERSION="4.4.2"
CHARTJS_ANNOTATION_VERSION="3.0.1"

VENDOR_DIR="$(dirname "$0")/../frontend/vendor"
IMAGES_DIR="$VENDOR_DIR/images"

mkdir -p "$VENDOR_DIR" "$IMAGES_DIR"

echo "Downloading Leaflet $LEAFLET_VERSION..."
curl -fsSL "https://unpkg.com/leaflet@${LEAFLET_VERSION}/dist/leaflet.js"  -o "$VENDOR_DIR/leaflet.js"
curl -fsSL "https://unpkg.com/leaflet@${LEAFLET_VERSION}/dist/leaflet.css" -o "$VENDOR_DIR/leaflet.css"

echo "Downloading Leaflet images..."
for img in layers.png layers-2x.png marker-icon.png marker-icon-2x.png marker-shadow.png; do
  curl -fsSL "https://unpkg.com/leaflet@${LEAFLET_VERSION}/dist/images/$img" -o "$IMAGES_DIR/$img"
done

echo "Downloading Chart.js $CHARTJS_VERSION..."
curl -fsSL "https://cdn.jsdelivr.net/npm/chart.js@${CHARTJS_VERSION}/dist/chart.umd.min.js" \
  -o "$VENDOR_DIR/chart.umd.min.js"

echo "Downloading chartjs-plugin-annotation $CHARTJS_ANNOTATION_VERSION..."
curl -fsSL "https://cdn.jsdelivr.net/npm/chartjs-plugin-annotation@${CHARTJS_ANNOTATION_VERSION}/dist/chartjs-plugin-annotation.min.js" \
  -o "$VENDOR_DIR/chartjs-plugin-annotation.min.js"

echo "Done. Vendor files updated in frontend/vendor/."
