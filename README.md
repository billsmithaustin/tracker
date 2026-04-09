# TransAmerica Trail Tracker

A cycling trip tracker/dashboard styled after the Artemis II mission tracker. NASA mission-control aesthetics applied to a bicycle trip.

**Route**: Yorktown, VA → Astoria, OR (~4,198 miles)

## Running locally

```bash
cp .env.example .env    # set CHECKIN_PASSWORD
make up                 # build and start → http://localhost:8080
make down               # stop
make reset              # stop and wipe all check-in data
make gpx                # regenerate route-data.js from GPX files
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

**2. Add `PROJECT_ID` to your `.env`**

```bash
echo "PROJECT_ID=$(gcloud config get-value project)" >> .env
```

**3. Build and push images**

```bash
make push
```

**4. Upload your `.env` and run setup on the VM**

```bash
gcloud compute scp .env tracker:~ --zone=us-west1-a
gcloud compute ssh tracker --zone=us-west1-a
```

Then on the VM:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/billsmithaustin/tracker/main/scripts/setup-vm.sh)
```

The tracker will be running at `http://<static-ip>:8080`.

### Deploying updates

After making changes locally:

```bash
make deploy
```

This builds updated images on your Mac, pushes them to Artifact Registry, then SSHes into the VM to pull and restart.

### Custom domain + SSL (optional)

1. The static IP is printed at the end of `make provision`. Add an `A` record at your registrar pointing to it.
2. SSH into the VM and install Certbot:
   ```bash
   sudo apt-get install -y certbot python3-certbot-nginx
   sudo certbot --nginx -d yourdomain.com
   ```
3. Update `docker-compose.yml` to expose port 80 instead of 8080, then `make deploy`.
