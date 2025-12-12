.PHONY: build docker-build docker-push deploy clean

# Variables
SERVICE_NAME := devboxmonitor
IMAGE_NAME := ghcr.io/labring/sealos-$(SERVICE_NAME)-service
TAG ?= latest

# Build the binary
build:
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o $(SERVICE_NAME) .

# Build Docker image
docker-build:
	docker build -t $(IMAGE_NAME):$(TAG) .

# Push Docker image
docker-push:
	docker push $(IMAGE_NAME):$(TAG)

# Deploy to Kubernetes
deploy:
	kubectl apply -f deploy/manifests/

# Clean build artifacts
clean:
	rm -f $(SERVICE_NAME)
	go clean

# Run locally
run:
	go run main.go

# Run tests
test:
	go test -v ./...
