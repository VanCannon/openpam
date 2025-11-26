.PHONY: help run build test migrate-up migrate-down migrate-status dev-up dev-down clean

help:
	@echo "Available commands:"
	@echo "  make run             - Run the gateway server"
	@echo "  make build           - Build the gateway binary"
	@echo "  make test            - Run tests"
	@echo "  make migrate-up      - Run all pending migrations"
	@echo "  make migrate-down    - Rollback the last migration"
	@echo "  make migrate-status  - Show migration status"
	@echo "  make dev-up          - Start dev environment (PostgreSQL + Vault)"
	@echo "  make dev-down        - Stop dev environment"
	@echo "  make clean           - Clean build artifacts"

migrate-up:
	cd gateway && go run cmd/migrate/main.go -action=up

migrate-down:
	cd gateway && go run cmd/migrate/main.go -action=down

migrate-status:
	cd gateway && go run cmd/migrate/main.go -action=status

gateway-dev:
	@echo "Starting gateway in development mode..."
	cd gateway && DEV_MODE=true go run cmd/server/main.go

kill-gateway:
	@echo "Killing any running gateway processes..."
	@pkill -9 -f "cmd/server/main.go" 2>/dev/null || echo "No cmd/server/main.go processes found"
	@pkill -9 -f "gateway.*go run" 2>/dev/null || true
	@pkill -9 -f "openpam-gateway" 2>/dev/null || true
	@pgrep -f "cmd/server" | xargs -r kill -9 2>/dev/null || true
	@echo "Gateway processes stopped"

run:
	cd gateway && go run cmd/server/main.go

build:
	cd gateway && go build -o ../bin/openpam-gateway cmd/server/main.go
	@echo "Binary built: bin/openpam-gateway"

test:
	cd gateway && go test -v ./...

dev-up:
	docker compose up -d
	@echo "Waiting for services to be ready..."
	@sleep 5
	@echo "Dev environment is up!"
	@echo "PostgreSQL: localhost:5432"
	@echo "Vault: http://localhost:8200"

dev-down:
	docker compose down

clean:
	rm -rf bin/
	cd gateway && go clean
