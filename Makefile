.PHONY: up down build logs reset gpx stamp

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
