# Common tasks. Postgres and Redis must be running (see README).
DATABASE_URL ?= postgres://postgres@localhost:55421/remitlane?sslmode=disable
TEST_DATABASE_URL ?= postgres://postgres@localhost:55421/remitlane_test?sslmode=disable

.PHONY: run build test lint sqlc migrate seed

run:
	go run ./cmd/remitlane

build:
	CGO_ENABLED=0 go build -o bin/remitlane ./cmd/remitlane

test:
	go test ./...

lint:
	golangci-lint run ./...

# Regenerates internal/db from internal/db/queries and migrations.
sqlc:
	sqlc generate

migrate:
	DATABASE_URL=$(DATABASE_URL) go run ./cmd/remitlane migrate

seed:
	DATABASE_URL=$(DATABASE_URL) go run ./cmd/remitlane seed
