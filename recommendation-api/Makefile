# Makefile for recommendation-api

.PHONY: all build test lint clean run docker-build help

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Binary name
BINARY_NAME=recommendation-api
BINARY_PATH=bin/$(BINARY_NAME)

# Docker parameters
DOCKER_IMAGE=container-resource-predictor/recommendation-api
DOCKER_TAG?=latest

# Build flags
LDFLAGS=-ldflags "-s -w"

all: lint test build

## build: Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p bin
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_PATH) ./cmd/server

## build-linux: Build for Linux (amd64)
build-linux:
	@echo "Building $(BINARY_NAME) for Linux amd64..."
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_PATH)-linux-amd64 ./cmd/server

## build-linux-arm64: Build for Linux (arm64)
build-linux-arm64:
	@echo "Building $(BINARY_NAME) for Linux arm64..."
	@mkdir -p bin
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_PATH)-linux-arm64 ./cmd/server

## test: Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -cover ./...

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

## lint: Run linters
lint: fmt vet
	@echo "Running golangci-lint..."
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

## fmt: Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GOVET) ./...

## tidy: Tidy go modules
tidy:
	@echo "Tidying modules..."
	$(GOMOD) tidy

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -f coverage.out coverage.html

## run: Run the application
run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BINARY_PATH)

## docker-build: Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

## docker-build-multiarch: Build multi-arch Docker image
docker-build-multiarch:
	@echo "Building multi-arch Docker image..."
	docker buildx build --platform linux/amd64,linux/arm64 -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

## proto: Generate protobuf code
proto:
	@echo "Generating protobuf code..."
	@which protoc > /dev/null || (echo "protoc not found, please install protobuf compiler" && exit 1)
	protoc --go_out=. --go-grpc_out=. api/proto/*.proto

## help: Show this help
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
