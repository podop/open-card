.PHONY: build run test lint fmt migrate-up migrate-down docker-up docker-down tidy

BINARY    := acs-server
CMD       := ./cmd/server
GOFLAGS   := -ldflags="-w -s"
MIGRATION_DIR := ./migrations
DB_DSN    ?= postgres://acs_user:secret@localhost:5432/acs_db?sslmode=disable

## build: compile the binary
build:
	CGO_ENABLED=0 go build $(GOFLAGS) -o bin/$(BINARY) $(CMD)

## run: run locally with config.yaml
run:
	go run $(CMD) --config config.yaml

## test: run all unit tests with coverage
test:
	go test ./... -v -race -coverprofile=coverage.out
	go tool cover -func=coverage.out

## lint: run golangci-lint (install: https://golangci-lint.run)
lint:
	golangci-lint run ./...

## fmt: format all Go source files
fmt:
	gofmt -w .
	goimports -w .

## tidy: tidy and verify module dependencies
tidy:
	go mod tidy
	go mod verify

## migrate-up: apply all pending database migrations
migrate-up:
	goose -dir $(MIGRATION_DIR) postgres "$(DB_DSN)" up

## migrate-down: roll back the last migration
migrate-down:
	goose -dir $(MIGRATION_DIR) postgres "$(DB_DSN)" down

## migrate-status: show migration status
migrate-status:
	goose -dir $(MIGRATION_DIR) postgres "$(DB_DSN)" status

## docker-up: start PostgreSQL + ACS via Docker Compose
docker-up:
	docker compose up --build -d

## docker-down: stop and remove containers
docker-down:
	docker compose down

## docker-logs: tail ACS container logs
docker-logs:
	docker compose logs -f acs

## generate-keys: generate random AES-256 keys for local dev (requires openssl)
generate-keys:
	@echo "master_key_hex:"; openssl rand -hex 32
	@echo "pan_encryption_key_hex:"; openssl rand -hex 32
