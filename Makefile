include .env
export

.PHONY: run build test lint docker-up docker-down

run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api

test:
	go test ./... -count=1 -coverprofile=coverage.out

lint:
	golangci-lint run

docker-up:
	docker-compose up --build

docker-down:
	docker-compose down
