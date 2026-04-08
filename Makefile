.PHONY: up down build logs reset gpx stamp provision deploy

up: stamp ## Start all services (build if needed)
	docker compose up --build

down: ## Stop all services
	docker compose down

build: ## Rebuild images without starting
	docker compose build

logs: ## Tail logs from all services
	docker compose logs -f

reset: ## Stop and delete all data (wipes check-ins)
	docker compose down -v

stamp: ## Rewrite ?v= cache-busters in HTML from asset file mtimes
	go run ./tools/stamp

gpx: ## Process GPX files → frontend/js/route-data.js (requires gpx/ directory)
	go run ./tools/process-gpx

provision: ## Create GCE e2-micro VM, static IP, and firewall rule (run once)
	./scripts/provision-gce.sh

deploy: ## Pull latest code and restart the stack on the GCE VM
	./scripts/deploy.sh
