SWAG := $(shell go env GOPATH)/bin/swag

.PHONY: generate build run

generate:
	$(SWAG) init -g cmd/server/main.go --output docs

build: generate
	go build ./...

run: generate
	go run cmd/server/main.go
