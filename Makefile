.PHONY: up down build logs reset gpx town-pops stamp vendor provision push deploy setup

# [Local dev] Build images if needed, generate route data and cache-busters, then start the full stack.
up: frontend/js/route-data.js stamp ## Start all services (build if needed)
	docker compose up --build

# [Local dev] Stop all running containers without removing data.
down: ## Stop all services
	docker compose down

# [Local dev] Rebuild Docker images without starting them (useful to pre-warm before `make up`).
build: frontend/js/route-data.js ## Rebuild images without starting
	docker compose build

# [Local dev] Stream live logs from all running containers.
logs: ## Tail logs from all services
	docker compose logs -f

# [Local dev] Stop containers and delete the data volume — wipes all check-ins and photos.
reset: ## Stop and delete all data (wipes check-ins)
	docker compose down -v

# [Local dev] Rewrite the ?v= cache-buster query strings in HTML files based on current asset mtimes.
stamp: ## Rewrite ?v= cache-busters in HTML from asset file mtimes
	go run ./tools/stamp

# [Local dev] Generate route-data.js from GPX files (runs automatically via `make up` if the file is absent).
frontend/js/route-data.js: ## Generate route-data.js from GPX files if not present
	go run ./tools/process-gpx

# [Local dev] Force-regenerate frontend/js/route-data.js even if it already exists.
gpx: ## Force-regenerate frontend/js/route-data.js from GPX files
	go run ./tools/process-gpx

# [Local dev] One-time data prep: fetch GeoNames population data for route towns and write data/town-populations.json.
# Requires GEONAMES_USER to be set in .env before running.
town-pops: ## Fetch GeoNames population data for route towns; writes data/town-populations.json (run once, set GEONAMES_USER in .env first)
	go run ./tools/fetch-town-pops

# [Local dev] Download or update vendored frontend libraries (Leaflet, Chart.js) into frontend/vendor/.
# Edit version variables in scripts/update-vendor.sh first, then run `make up` to rebuild.
vendor: ## Download/update vendored frontend libraries (Leaflet, Chart.js) into frontend/vendor/
	./scripts/update-vendor.sh

# [Production – run once] Provision the GCE e2-micro VM, Artifact Registry repo, static IP, and firewall rules.
provision: ## Create GCE e2-micro VM, Artifact Registry repo, static IP, firewall (run once)
	./scripts/provision-gce.sh

PROJECT_ID := $(shell grep '^PROJECT_ID=' .env | cut -d= -f2)
REGISTRY   := us-west1-docker.pkg.dev/$(PROJECT_ID)/tracker
INSTANCE   := tracker
ZONE       := us-west1-a

# [Production] Cross-compile and push linux/amd64 images for the API and frontend to Artifact Registry.
push: ## Build images for linux/amd64 and push to Artifact Registry
	docker buildx build --platform linux/amd64 -t $(REGISTRY)/api:latest --push ./api
	docker buildx build --platform linux/amd64 -t $(REGISTRY)/frontend:latest --push ./frontend

# [Production] Push updated images then SSH into the GCE VM to pull and restart the running containers.
deploy: push ## Build, push images, then pull and restart on the GCE VM
	./scripts/deploy.sh

# [Production – run once] First-time VM setup: push images, copy .env and setup script to the VM, then run it to install Docker, configure SSL, and start the app.
setup: push ## First-time VM setup: push images, copy .env, install Docker+SSL, start app
	gcloud compute scp .env $(INSTANCE):~ --zone=$(ZONE) --project=$(PROJECT_ID)
	gcloud compute scp scripts/setup-vm.sh $(INSTANCE):~ --zone=$(ZONE) --project=$(PROJECT_ID)
	gcloud compute ssh $(INSTANCE) --zone=$(ZONE) --project=$(PROJECT_ID) --command="bash ~/setup-vm.sh"
