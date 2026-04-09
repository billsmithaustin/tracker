# TransAmerica Trail Tracker

A cycling trip tracker/dashboard styled after the Artemis II mission tracker. NASA mission-control aesthetics applied to a bicycle trip.

**Route**: Yorktown, VA → Astoria, OR (~4,198 miles)

## GPX files

The route data (`frontend/js/route-data.js`) is pre-generated and checked in, so you don't need GPX files just to run the app. You only need them if you want to regenerate the route from scratch.

The tool expects 12 GPX files from the **TransAmerica Trail** sold by [Adventure Cycling Association](https://www.adventurecycling.org/routes-and-maps/adventure-cycling-route-network/transamerica-trail/). Purchase the route, then copy the **Westbound Main** section files into the `gpx/` directory:

```
gpx/
  TA_01_WB_Main_YYYY.gpx
  TA_02_WB_Main_YYYY.gpx
  ...
  TA_12_WB_Main_YYYY.gpx
```

(`gpx/` is gitignored.) Then run `make gpx` to regenerate `route-data.js`.

## Running locally

```bash
cp .env.example .env    # set CHECKIN_PASSWORD
make up                 # build and start → http://localhost:8080
make down               # stop
make reset              # stop and wipe all check-in data
make gpx                # regenerate route-data.js from GPX files (see above)
```

## Deploying to Google Cloud (free tier)

Google's always-free tier includes one `e2-micro` VM (1 vCPU, 1 GB RAM, 30 GB disk) in `us-west1`, `us-central1`, or `us-east1`. The existing Docker Compose stack runs on it unchanged.

Docker images are cross-compiled locally on your Mac for `linux/amd64` and pushed to Google Artifact Registry, so the VM only pulls pre-built images — no compilation on the e2-micro. (`make up` still builds natively for your Mac.)

### Prerequisites

- [gcloud CLI](https://cloud.google.com/sdk/docs/install) installed and authenticated (`gcloud auth login`)
- A Google Cloud project (`gcloud config set project PROJECT_ID`)
- Docker Desktop running locally

### One-time setup

**1. Provision the VM and Artifact Registry**

```bash
make provision
# or: ./scripts/provision-gce.sh [PROJECT_ID]
```

This creates:
- An Artifact Registry Docker repository (`us-west1`)
- A static external IP address (free while attached to a running instance)
- An `e2-micro` VM running Debian 12 in `us-west1-a`
- A firewall rule opening ports 80, 443, and 8080

The static IP is printed at the end. Note it for the next step.

**2. Update your `.env`**

```bash
echo "PROJECT_ID=$(gcloud config get-value project)" >> .env
```

To enable HTTPS, also add:

```
DOMAIN=yourdomain.com
CERTBOT_EMAIL=you@example.com
```

If using a custom domain, add a DNS `A` record pointing to the static IP from step 1 and wait for it to propagate before continuing.

**3. Run first-time setup**

```bash
make setup
```

This builds images on your Mac, pushes them to Artifact Registry, copies your `.env` to the VM, installs Docker, optionally obtains an SSL certificate via Let's Encrypt (if `DOMAIN` and `CERTBOT_EMAIL` are set), and starts the tracker.

### Deploying updates

After making changes locally:

```bash
make deploy
```

This builds updated images on your Mac, pushes them to Artifact Registry, then SSHes into the VM to pull and restart.
