.PHONY: up down build logs reset gpx stamp vendor provision push deploy setup

up: frontend/js/route-data.js stamp ## Start all services (build if needed)
	docker compose up --build

down: ## Stop all services
	docker compose down

build: frontend/js/route-data.js ## Rebuild images without starting
	docker compose build

logs: ## Tail logs from all services
	docker compose logs -f

reset: ## Stop and delete all data (wipes check-ins)
	docker compose down -v

stamp: ## Rewrite ?v= cache-busters in HTML from asset file mtimes
	go run ./tools/stamp

frontend/js/route-data.js: ## Generate route-data.js from GPX files if not present
	go run ./tools/process-gpx

gpx: ## Force-regenerate frontend/js/route-data.js from GPX files
	go run ./tools/process-gpx

vendor: ## Download/update vendored frontend libraries (Leaflet, Chart.js) into frontend/vendor/
	./scripts/update-vendor.sh

provision: ## Create GCE e2-micro VM, Artifact Registry repo, static IP, firewall (run once)
	./scripts/provision-gce.sh

PROJECT_ID := $(shell grep '^PROJECT_ID=' .env | cut -d= -f2)
REGISTRY   := us-west1-docker.pkg.dev/$(PROJECT_ID)/tracker
INSTANCE   := tracker
ZONE       := us-west1-a

push: ## Build images for linux/amd64 and push to Artifact Registry
	docker buildx build --platform linux/amd64 -t $(REGISTRY)/api:latest --push ./api
	docker buildx build --platform linux/amd64 -t $(REGISTRY)/frontend:latest --push ./frontend

deploy: push ## Build, push images, then pull and restart on the GCE VM
	./scripts/deploy.sh

setup: push ## First-time VM setup: push images, copy .env, install Docker+SSL, start app
	gcloud compute scp .env $(INSTANCE):~ --zone=$(ZONE) --project=$(PROJECT_ID)
	gcloud compute scp scripts/setup-vm.sh $(INSTANCE):~ --zone=$(ZONE) --project=$(PROJECT_ID)
	gcloud compute ssh $(INSTANCE) --zone=$(ZONE) --project=$(PROJECT_ID) --command="bash ~/setup-vm.sh"
