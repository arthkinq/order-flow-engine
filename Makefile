.PHONY: proto build run-api run-worker test docker-up docker-down clean help

PROTO_DIR=proto
COMPOSE_FILE=deployments/docker-compose.yml

proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/order/v1/order.proto

build:
	go build -o bin/api ./cmd/api
	go build -o bin/worker ./cmd/worker

run-api:
	go run ./cmd/api

run-worker:
	go run ./cmd/worker

test:
	go test -v ./...

docker-up:
	docker compose -f $(COMPOSE_FILE) up -d --build

docker-down:
	docker compose -f $(COMPOSE_FILE) down

clean:
	rm -rf bin/

