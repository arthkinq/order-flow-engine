.PHONY: proto build run-api run-worker test lint docker-up docker-down k8s-up k8s-down clean help

PROTO_DIR=proto
COMPOSE_FILE=deployments/docker-compose.yml
KIND_CLUSTER=order-flow

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
	go test -v ./... -cover

lint:
	golangci-lint run ./...

docker-up:
	docker compose -f $(COMPOSE_FILE) up -d --build

docker-down:
	docker compose -f $(COMPOSE_FILE) down

k8s-up:
	docker build --target api -t order-flow-engine-api:latest .
	docker build --target worker -t order-flow-engine-worker:latest .
	kind create cluster --name $(KIND_CLUSTER) || true
	kind load docker-image order-flow-engine-api:latest --name $(KIND_CLUSTER)
	kind load docker-image order-flow-engine-worker:latest --name $(KIND_CLUSTER)
	kubectl apply -f deployments/k8s/configmap.yaml -f deployments/k8s/infrastructure.yaml
	kubectl wait --for=condition=ready pod -l app=postgres --timeout=90s
	kubectl wait --for=condition=ready pod -l app=rabbitmq --timeout=90s
	kubectl apply -f deployments/k8s/deployment.yaml -f deployments/k8s/service.yaml

k8s-down:
	kind delete cluster --name $(KIND_CLUSTER)

clean:
	rm -rf bin/
