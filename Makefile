.PHONY: all build test up down verify reset-demo load

all: build test

build:
	@echo "==> Building Backend..."
	cd backend && go build ./...
	@echo "==> Building Simulator..."
	cd simulator && go build ./...

test:
	@echo "==> Running Backend Tests..."
	cd backend && go test -v ./...

up:
	docker compose up --build

down:
	docker compose down

verify:
	@echo "==> Running verification suite (proje.md scope F-1..F-6 and tasks T-1..T-4)..."
	cd backend && go run ./cmd/verify

reset-demo:
	@echo "==> Resetting database to initial deterministic demo state (Seed=42)..."
	cd backend && go run ./cmd/seed --reset

load:
	@echo "==> Running turnstile decision load benchmark (Task 3: p95 < 50ms)..."
	cd backend && go run ./cmd/load
