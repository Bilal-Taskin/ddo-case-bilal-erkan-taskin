.PHONY: all build test up down verify reset-demo

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
	@echo "==> Running verification suite..."
	cd backend && go run ./cmd/verify

reset-demo:
	@echo "==> Resetting database to initial demo state..."
	cd backend && go run ./cmd/seed --reset
