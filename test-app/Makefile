# Makefile for kubewise-test-app

# Variables
BINARY_DIR := bin
DOCKER_REGISTRY ?= kubewise-test
VERSION ?= latest
GO := go
GOFLAGS := -ldflags="-s -w"

# Component list
COMPONENTS := pattern-controller load-generator memory-hog cpu-burster steady-worker metrics-validator

.PHONY: all build clean test lint docker-build docker-push help $(COMPONENTS)

# Default target
all: build

# Build all components
build: $(COMPONENTS)

# Build individual components
$(COMPONENTS):
	@echo "Building $@..."
	@mkdir -p $(BINARY_DIR)
	$(GO) build $(GOFLAGS) -o $(BINARY_DIR)/$@ ./cmd/$@

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BINARY_DIR)

# Run tests
test:
	@echo "Running tests..."
	$(GO) test -v ./...

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run ./...

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GO) mod download
	$(GO) mod tidy

# Build all Docker images
docker-build: $(addprefix docker-build-,$(COMPONENTS))

docker-build-%:
	@echo "Building Docker image for $*..."
	docker build -t $(DOCKER_REGISTRY)/$*:$(VERSION) -f Dockerfile --build-arg COMPONENT=$* .

# Push all Docker images
docker-push: $(addprefix docker-push-,$(COMPONENTS))

docker-push-%:
	@echo "Pushing Docker image for $*..."
	docker push $(DOCKER_REGISTRY)/$*:$(VERSION)

# Run a specific component locally
run-%:
	@echo "Running $*..."
	$(GO) run ./cmd/$*

# Cluster setup and E2E testing
.PHONY: setup-cluster install-kubewise install-test-app e2e-test

# Create kind cluster with all dependencies
setup-cluster:
	@echo "Setting up kind cluster..."
	./hack/setup-kind.sh

# Install Kubewise to the cluster
install-kubewise:
	@echo "Installing Kubewise..."
	./hack/install-kubewise.sh

# Install test application to the cluster
install-test-app:
	@echo "Installing test application..."
	./hack/install-test-app.sh

# Run E2E tests
e2e-test:
	@echo "Running E2E tests..."
	./hack/run-e2e-tests.sh

# Full E2E setup and test
e2e-full: setup-cluster install-kubewise docker-build install-test-app e2e-test

# Load images into kind cluster
kind-load:
	@echo "Loading images into kind cluster..."
	@for component in $(COMPONENTS); do \
		kind load docker-image $(DOCKER_REGISTRY)/$$component:$(VERSION) --name kubewise-test 2>/dev/null || true; \
	done

# Help
help:
	@echo "Available targets:"
	@echo "  all             - Build all components (default)"
	@echo "  build           - Build all components"
	@echo "  clean           - Remove build artifacts"
	@echo "  test            - Run tests"
	@echo "  lint            - Run linter"
	@echo "  deps            - Download and tidy dependencies"
	@echo "  docker-build    - Build all Docker images"
	@echo "  docker-push     - Push all Docker images"
	@echo "  run-<name>      - Run a specific component locally"
	@echo ""
	@echo "Cluster and E2E targets:"
	@echo "  setup-cluster   - Create kind cluster with dependencies"
	@echo "  install-kubewise- Install Kubewise to cluster"
	@echo "  install-test-app- Install test application to cluster"
	@echo "  e2e-test        - Run E2E tests"
	@echo "  e2e-full        - Full setup and E2E test run"
	@echo "  kind-load       - Load Docker images into kind cluster"
	@echo ""
	@echo "Components: $(COMPONENTS)"
	@echo ""
	@echo "Variables:"
	@echo "  DOCKER_REGISTRY - Docker registry (default: kubewise-test)"
	@echo "  VERSION         - Image version tag (default: latest)"
