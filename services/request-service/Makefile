SWAG    := $(shell go env GOPATH)/bin/swag
MIGRATE := migrate
DB_URL  ?= postgres://appraisal:appraisal@localhost:5433/request_db?sslmode=disable

.PHONY: generate build run test migrate-up migrate-down

generate:
	$(SWAG) init -g cmd/server/main.go --output docs

build: generate
	go build ./...

run: generate
	go run cmd/server/main.go

test:
	go test ./...

migrate-up:
	$(MIGRATE) -path migrations -database "$(DB_URL)" up

migrate-down:
	$(MIGRATE) -path migrations -database "$(DB_URL)" down 1
