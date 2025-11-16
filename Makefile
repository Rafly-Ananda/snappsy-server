.PHONY: help dev dev-build dev-down dev-logs dev-restart prod prod-build prod-down prod-logs test clean

# Default target
help:
	@echo "Available commands:"
	@echo "  make dev          - Start development environment"
	@echo "  make dev-build    - Rebuild and start development environment"
	@echo "  make dev-down     - Stop development environment"
	@echo "  make dev-logs     - View development logs"
	@echo "  make dev-restart  - Restart development services"
	@echo "  make prod         - Start production environment"
	@echo "  make prod-build   - Rebuild and start production environment"
	@echo "  make prod-down    - Stop production environment"
	@echo "  make prod-logs    - View production logs"
	@echo "  make test         - Run tests"
	@echo "  make clean        - Clean up volumes and images"

# Development commands
dev:
	@echo "Starting development environment..."
	cp .env.example .env
	docker compose -f docker compose.dev.yml up -d
	@echo ""
	@echo "✓ Development environment started!"
	@echo ""
	@echo "Services available at:"
	@echo "  - API:          http://localhost:8080"
	@echo "  - MinIO Console: http://localhost:9001 (minioadmin/minioadmin)"
	@echo "  - Mongo Express: http://localhost:8081 (admin/admin)"
	@echo "  - Mailhog UI:    http://localhost:8025"
	@echo "  - Redis:         localhost:6379"
	@echo ""
	@echo "View logs: make dev-logs"

dev-build:
	@echo "Rebuilding development environment..."
	cp .env.example .env
	docker compose -f docker compose.dev.yml up -d --build

dev-down:
	@echo "Stopping development environment..."
	docker compose -f docker compose.dev.yml down

dev-logs:
	docker compose -f docker compose.dev.yml logs -f

dev-restart:
	@echo "Restarting development services..."
	docker compose -f docker compose.dev.yml restart goapp

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
	docker compose -f docker compose.dev.yml exec mongo mongosh -u admin -p devpassword123

db-backup:
	@echo "Creating database backup..."
	docker compose exec mongo mongodump --uri="mongodb://admin:devpassword123@localhost:27017" --out=/data/backup
	@echo "Backup created in mongo container at /data/backup"

# Cleanup
clean:
	@echo "Cleaning up..."
	docker compose -f docker compose.dev.yml down -v
	docker compose down -v
	docker system prune -f
	@echo "✓ Cleanup complete!"

clean-all:
	@echo "WARNING: This will remove all containers, volumes, and images!"
	@read -p "Are you sure? [y/N] " -n 1 -r; \
	echo; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		docker compose -f docker compose.dev.yml down -v --remove-orphans; \
		docker compose down -v --remove-orphans; \
		docker system prune -a -f --volumes; \
		echo "✓ All cleaned up!"; \
	fi

# Installation
install-air:
	@echo "Installing Air for hot reload..."
	go install github.com/air-verse/air@latest

install-tools:
	@echo "Installing development tools..."
	go install github.com/air-verse/air@latest
	go install github.com/go-delve/delve/cmd/dlv@latest
	go install golang.org/x/tools/gopls@latest
	@echo "✓ Tools installed!"
