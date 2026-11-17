AIR ?= ./bin/air
ADMIN_USERNAME ?= snappsy-admin
ADMIN_PASSWORD ?= snappsy-dev-password

.PHONY: help dev dev-local dev-full dev-build deps dev-stop dev-down dev-logs dev-restart prod prod-build prod-down prod-logs test clean

# Default target
help:
	@echo "Available commands:"
	@echo "  make dev          - Start MongoDB/SeaweedFS and run API with Air hot reload"
	@echo "  make dev-local    - Alias for make dev"
	@echo "  make dev-full     - Run the whole development stack in Docker"
	@echo "  make deps         - Start MongoDB, SeaweedFS and boomerang worker"
	@echo "  make dev-build    - Rebuild and start full Docker development stack"
	@echo "  make dev-down     - Stop development environment"
	@echo "  make dev-logs     - View development logs"
	@echo "  make dev-restart  - Restart Dockerized goapp (only for make dev-full)"
	@echo "  make prod         - Start production environment"
	@echo "  make prod-build   - Rebuild and start production environment"
	@echo "  make prod-down    - Stop production environment"
	@echo "  make prod-logs    - View production logs"
	@echo "  make test         - Run tests"
	@echo "  make clean        - Clean up volumes and images"

# Development commands
dev: deps dev-stop
	MONGODB_HOST=localhost:27017 SEAWEEDFS_ENDPOINT=localhost:9000 SEAWEEDFS_PUBLIC_URL=http://localhost:9000 ADMIN_USERNAME=$(ADMIN_USERNAME) ADMIN_PASSWORD=$(ADMIN_PASSWORD) $(AIR) -c .air.toml

dev-local: dev

deps:
	@echo "Starting development dependencies..."
	docker compose -f docker-compose.dev.yml up -d mongo seaweedfs
	docker compose -f docker-compose.dev.yml up -d --force-recreate boomerang-worker
	@echo ""
	@echo "✓ Dependencies started"
	@echo "  - MongoDB:       localhost:27017"
	@echo "  - SeaweedFS S3:  localhost:9000"
	@echo "  - SeaweedFS Admin: http://localhost:9001"
	@echo "  - Boomerang worker: Docker (ffmpeg)"
	@echo ""
	@echo "API will run on: http://localhost:8080"

dev-stop:
	@echo "Stopping previous local Air server..."
	@pkill -f "$(AIR) -c .air.toml" 2>/dev/null || true
	@pkill -f "tmp/main" 2>/dev/null || true
	@pid=$$(lsof -ti tcp:8080); \
	if [ -n "$$pid" ]; then \
		echo "Stopping process on port 8080: $$pid"; \
		kill $$pid 2>/dev/null || true; \
		sleep 1; \
		pid=$$(lsof -ti tcp:8080); \
		if [ -n "$$pid" ]; then kill -9 $$pid 2>/dev/null || true; fi; \
		sleep 1; \
	fi; \
	pid=$$(lsof -ti tcp:8080); \
	if [ -n "$$pid" ]; then \
		echo "Port 8080 is still in use by: $$pid"; \
		exit 1; \
	fi

dev-full:
	@echo "Starting full Docker development environment..."
	docker compose -f docker-compose.dev.yml up -d
	@echo ""
	@echo "✓ Full development environment started"
	@echo "  - API:          http://localhost:8080"
	@echo "  - SeaweedFS Admin: http://localhost:9001"
	@echo "  - Mongo Express: http://localhost:8081"
	@echo "  - Mailhog UI:    http://localhost:8025"
	@echo "  - Redis:         localhost:6378"
	@echo ""
	@echo "View logs: make dev-logs"

dev-build:
	@echo "Rebuilding full Docker development environment..."
	docker compose -f docker-compose.dev.yml up -d --build

dev-down:
	@echo "Stopping development environment..."
	docker compose -f docker-compose.dev.yml down

dev-logs:
	docker compose -f docker-compose.dev.yml logs -f

dev-restart:
	@echo "Restarting Dockerized goapp..."
	docker compose -f docker-compose.dev.yml restart goapp

# Production commands
prod:
	@echo "Starting production environment..."
	@if [ ! -f .env ]; then \
		echo "Error: .env file not found!"; \
		echo "Please create .env from .env.example"; \
		exit 1; \
	fi
	export BUILD_TIME=$$(date -u +"%Y-%m-%dT%H:%M:%SZ") && \
	docker compose up -d
	@echo ""
	@echo "✓ Production environment started!"
	@echo ""
	@echo "View logs: make prod-logs"

prod-build:
	@echo "Rebuilding production environment..."
	@if [ ! -f .env ]; then \
		echo "Error: .env file not found!"; \
		exit 1; \
	fi
	export BUILD_TIME=$$(date -u +"%Y-%m-%dT%H:%M:%SZ") && \
	docker compose up -d --build

prod-down:
	@echo "Stopping production environment..."
	docker compose down

prod-logs:
	docker compose logs -f

# Testing
test:
	@echo "Running tests..."
	go test -v -race ./internal/... ./cmd/...

test-coverage:
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out ./internal/... ./cmd/...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Database commands
db-shell:
	docker compose -f docker-compose.dev.yml exec mongo mongosh -u admin -p devpassword123

db-backup:
	@echo "Creating database backup..."
	docker compose exec mongo mongodump --uri="mongodb://admin:devpassword123@localhost:27017" --out=/data/backup
	@echo "Backup created in mongo container at /data/backup"

# Cleanup
clean:
	@echo "Cleaning up..."
	docker compose -f docker-compose.dev.yml down -v
	docker compose down -v
	docker system prune -f
	@echo "✓ Cleanup complete!"

clean-all:
	@echo "WARNING: This will remove all containers, volumes, and images!"
	@read -p "Are you sure? [y/N] " -n 1 -r; \
	echo; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		docker compose -f docker-compose.dev.yml down -v --remove-orphans; \
		docker compose down -v --remove-orphans; \
		docker system prune -a -f --volumes; \
		echo "✓ All cleaned up!"; \
	fi

# Installation
install-air:
	@echo "Installing Air for hot reload..."
	mkdir -p bin
	GOBIN=$$(pwd)/bin go install github.com/air-verse/air@v1.52.3

install-tools:
	@echo "Installing development tools..."
	GOBIN=$$(pwd)/bin go install github.com/air-verse/air@v1.52.3
	go install github.com/go-delve/delve/cmd/dlv@v1.24.1
	go install golang.org/x/tools/gopls@v0.18.1
	@echo "✓ Tools installed!"
