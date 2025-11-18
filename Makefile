# Makefile for hyperfleet-adapter

# Project metadata
PROJECT_NAME := hyperfleet-adapter
VERSION ?= 0.0.1
IMAGE_REGISTRY ?= quay.io/openshift-hyperfleet
IMAGE_TAG ?= latest

# Build metadata
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_TAG := $(shell git describe --tags --exact-match 2>/dev/null || echo "")
BUILD_DATE := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

# LDFLAGS for build
LDFLAGS := -w -s
LDFLAGS += -X github.com/openshift-hyperfleet/hyperfleet-adapter/cmd/adapter.version=$(VERSION)
LDFLAGS += -X github.com/openshift-hyperfleet/hyperfleet-adapter/cmd/adapter.commit=$(GIT_COMMIT)
LDFLAGS += -X github.com/openshift-hyperfleet/hyperfleet-adapter/cmd/adapter.buildDate=$(BUILD_DATE)
ifneq ($(GIT_TAG),)
LDFLAGS += -X github.com/openshift-hyperfleet/hyperfleet-adapter/cmd/adapter.tag=$(GIT_TAG)
endif

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOMOD := $(GOCMD) mod
GOFMT := gofmt
GOIMPORTS := goimports

# Test parameters
TEST_TIMEOUT := 30m
RACE_FLAG := -race
COVERAGE_OUT := coverage.out
COVERAGE_HTML := coverage.html

# Container runtime detection
DOCKER_AVAILABLE := $(shell docker info >/dev/null 2>&1 && echo "true" || echo "false")
PODMAN_AVAILABLE := $(shell podman info >/dev/null 2>&1 && echo "true" || echo "false")

ifeq ($(DOCKER_AVAILABLE),true)
    CONTAINER_RUNTIME := docker
    CONTAINER_CMD := docker
else ifeq ($(PODMAN_AVAILABLE),true)
    CONTAINER_RUNTIME := podman
    CONTAINER_CMD := podman
    # Find Podman socket for testcontainers compatibility
    PODMAN_SOCK := $(shell find /var/folders -name "podman-machine-*-api.sock" 2>/dev/null | head -1)
    ifeq ($(PODMAN_SOCK),)
        PODMAN_SOCK := $(shell find ~/.local/share/containers/podman/machine -name "*.sock" 2>/dev/null | head -1)
    endif
    ifneq ($(PODMAN_SOCK),)
        export DOCKER_HOST := unix://$(PODMAN_SOCK)
        export TESTCONTAINERS_RYUK_DISABLED := true
    endif
else
    CONTAINER_RUNTIME := none
    CONTAINER_CMD := echo "No container runtime found. Please install Docker or Podman." && exit 1 &&
endif

# Directories
# Find all Go packages, excluding vendor and test directories
PKG_DIRS := $(shell $(GOCMD) list ./... 2>/dev/null | grep -v /vendor/ | grep -v /test/ || echo "./...")

.PHONY: help
help: ## Display this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.PHONY: container-info
container-info: ## Show detected container runtime information
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "Container Runtime Information"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "Runtime detected: $(CONTAINER_RUNTIME)"
	@echo "Command: $(CONTAINER_CMD)"
ifeq ($(CONTAINER_RUNTIME),podman)
	@echo "Podman socket: $(PODMAN_SOCK)"
	@echo "DOCKER_HOST: $(DOCKER_HOST)"
	@echo "TESTCONTAINERS_RYUK_DISABLED: $(TESTCONTAINERS_RYUK_DISABLED)"
endif
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

.PHONY: test
test: ## Run unit tests with race detection
	@echo "Running unit tests..."
	$(GOTEST) -v $(RACE_FLAG) -timeout $(TEST_TIMEOUT) $(PKG_DIRS)

.PHONY: test-coverage
test-coverage: ## Run unit tests with coverage report
	@echo "Running unit tests with coverage..."
	$(GOTEST) -v $(RACE_FLAG) -timeout $(TEST_TIMEOUT) -coverprofile=$(COVERAGE_OUT) -covermode=atomic $(PKG_DIRS)
	@echo "Coverage report generated: $(COVERAGE_OUT)"
	@echo "To view HTML coverage report, run: make test-coverage-html"

.PHONY: test-coverage-html
test-coverage-html: test-coverage ## Generate HTML coverage report
	@echo "Generating HTML coverage report..."
	$(GOCMD) tool cover -html=$(COVERAGE_OUT) -o $(COVERAGE_HTML)
	@echo "HTML coverage report generated: $(COVERAGE_HTML)"

.PHONY: test-integration
image-integration-test: ## 🔨 Build integration test image with envtest
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "🔨 Building Integration Test Image"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo ""
ifeq ($(CONTAINER_CMD),)
	@echo "❌ ERROR: No container runtime found (docker/podman required)"
	@exit 1
else
	@echo "📦 Building image: localhost/hyperfleet-integration-test:latest"
	@echo "   This downloads ~100MB of Kubernetes binaries (one-time operation)"
	@echo ""
ifeq ($(CONTAINER_RUNTIME),podman)
	@PROXY_HTTP=$$(podman machine ssh 'echo $$HTTP_PROXY' 2>/dev/null) && \
	PROXY_HTTPS=$$(podman machine ssh 'echo $$HTTPS_PROXY' 2>/dev/null) && \
	if [ -n "$$PROXY_HTTP" ] || [ -n "$$PROXY_HTTPS" ]; then \
		echo "   Using proxy: $$PROXY_HTTP"; \
		$(CONTAINER_CMD) build \
			--build-arg HTTP_PROXY=$$PROXY_HTTP \
			--build-arg HTTPS_PROXY=$$PROXY_HTTPS \
			-t localhost/hyperfleet-integration-test:latest \
			-f test/Dockerfile.integration \
			test/ || exit 1; \
	else \
		$(CONTAINER_CMD) build \
			-t localhost/hyperfleet-integration-test:latest \
			-f test/Dockerfile.integration \
			test/ || exit 1; \
	fi
else
	$(CONTAINER_CMD) build \
		-t localhost/hyperfleet-integration-test:latest \
		-f test/Dockerfile.integration \
		test/ || exit 1
endif
	@echo ""
	@echo "✅ Integration test image built successfully!"
	@echo "   Image: localhost/hyperfleet-integration-test:latest"
	@echo ""
endif

build-integration-image: image-integration-test ## 🔨 Alias for image-integration-test (deprecated, use image-integration-test)

integration-image: image-integration-test ## 🔨 Alias for image-integration-test (deprecated, use image-integration-test)

test-integration: ## 🐳 Run integration tests (requires Docker/Podman)
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "🐳 Running Integration Tests with Testcontainers"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo ""
ifeq ($(CONTAINER_RUNTIME),none)
	@echo "❌ ERROR: Neither Docker nor Podman is running"
	@echo ""
	@echo "Please start Docker or Podman:"
	@echo "  Docker: Start Docker Desktop or run 'dockerd'"
	@echo "  Podman: Run 'podman machine start'"
	@echo ""
	@exit 1
else
	@echo "✅ Container runtime: $(CONTAINER_RUNTIME)"
ifeq ($(CONTAINER_RUNTIME),podman)
ifneq ($(PODMAN_SOCK),)
	@echo "   Using Podman socket: $(DOCKER_HOST)"
else
	@echo "⚠️  WARNING: Podman socket not found, tests may fail"
endif
endif
	@echo ""
	@echo "🚀 Starting integration tests..."
	@echo "   Checking integration image configuration..."
ifeq ($(CONTAINER_RUNTIME),podman)
	@echo "📡 Detecting proxy configuration from Podman machine..."
	@echo "   Setting TESTCONTAINERS_RYUK_DISABLED=true (Podman compatibility)"
	@PROXY_HTTP=$$(podman machine ssh 'echo $$HTTP_PROXY' 2>/dev/null); \
	PROXY_HTTPS=$$(podman machine ssh 'echo $$HTTPS_PROXY' 2>/dev/null); \
	if [ -z "$$INTEGRATION_ENVTEST_IMAGE" ]; then \
		echo "   INTEGRATION_ENVTEST_IMAGE not set, using local image"; \
		if ! $(CONTAINER_CMD) images localhost/hyperfleet-integration-test:latest 2>/dev/null | grep -q "hyperfleet-integration-test"; then \
			echo "   ⚠️  Local integration image not found. Building it..."; \
		echo ""; \
			$(MAKE) image-integration-test || exit 1; \
		echo ""; \
		else \
			echo "   ✅ Local integration image found"; \
		fi; \
		INTEGRATION_ENVTEST_IMAGE="localhost/hyperfleet-integration-test:latest"; \
		fi; \
	echo "   Using INTEGRATION_ENVTEST_IMAGE=$$INTEGRATION_ENVTEST_IMAGE"; \
		echo ""; \
	if [ -n "$$PROXY_HTTP" ] || [ -n "$$PROXY_HTTPS" ]; then \
		echo "   Using HTTP_PROXY=$$PROXY_HTTP"; \
		echo "   Using HTTPS_PROXY=$$PROXY_HTTPS"; \
		HTTP_PROXY=$$PROXY_HTTP HTTPS_PROXY=$$PROXY_HTTPS INTEGRATION_ENVTEST_IMAGE=$$INTEGRATION_ENVTEST_IMAGE TESTCONTAINERS_RYUK_DISABLED=true TESTCONTAINERS_LOG_LEVEL=INFO $(GOTEST) -v -count=1 -tags=integration ./test/integration/... -timeout $(TEST_TIMEOUT) || exit 1; \
	else \
		INTEGRATION_ENVTEST_IMAGE=$$INTEGRATION_ENVTEST_IMAGE TESTCONTAINERS_RYUK_DISABLED=true TESTCONTAINERS_LOG_LEVEL=INFO $(GOTEST) -v -count=1 -tags=integration ./test/integration/... -timeout $(TEST_TIMEOUT) || exit 1; \
	fi
else
	@if [ -z "$$INTEGRATION_ENVTEST_IMAGE" ]; then \
		echo "   INTEGRATION_ENVTEST_IMAGE not set, using local image"; \
		if ! $(CONTAINER_CMD) images localhost/hyperfleet-integration-test:latest 2>/dev/null | grep -q "hyperfleet-integration-test"; then \
			echo "   ⚠️  Local integration image not found. Building it..."; \
			echo ""; \
			$(MAKE) image-integration-test || exit 1; \
		echo ""; \
		else \
			echo "   ✅ Local integration image found"; \
		fi; \
		INTEGRATION_ENVTEST_IMAGE="localhost/hyperfleet-integration-test:latest"; \
	fi; \
	echo "   Using INTEGRATION_ENVTEST_IMAGE=$$INTEGRATION_ENVTEST_IMAGE"; \
	echo ""; \
	INTEGRATION_ENVTEST_IMAGE=$$INTEGRATION_ENVTEST_IMAGE $(GOTEST) -v -count=1 -tags=integration ./test/integration/... -timeout $(TEST_TIMEOUT) || exit 1
endif
	@echo ""
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "✅ Integration tests passed!"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
endif

# Run integration tests using K3s strategy (privileged, more realistic Kubernetes)
# This uses testcontainers to spin up a real K3s cluster
# NOTE: Requires privileged containers, may not work in all CI/CD environments
.PHONY: test-integration-k3s
test-integration-k3s: ## 🚀 Run integration tests with K3s (faster, may need privileges)
ifeq ($(CONTAINER_RUNTIME),none)
	@echo "⚠️  ERROR: No container runtime found (docker/podman)"
	@echo "   Please install Docker Desktop or Podman to run integration tests"
	@exit 1
else
	@echo "✅ Container runtime: $(CONTAINER_RUNTIME)"
ifeq ($(CONTAINER_RUNTIME),podman)
ifneq ($(PODMAN_SOCK),)
	@echo "   Using Podman socket: $(DOCKER_HOST)"
else
	@echo "⚠️  WARNING: Podman socket not found, tests may fail"
endif
	@echo ""
	@echo "🔍 Checking Podman configuration for K3s compatibility..."
	@ROOTFUL=$$(podman machine inspect --format '{{.Rootful}}' 2>/dev/null || echo "unknown"); \
	if [ "$$ROOTFUL" = "false" ]; then \
		echo ""; \
		echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; \
		echo "⚠️  WARNING: Podman is in ROOTLESS mode"; \
		echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; \
		echo ""; \
		echo "K3s requires rootful Podman or proper cgroup v2 delegation for testcontainers."; \
		echo "Rootless Podman may fail with errors like:"; \
		echo "  • 'failed to find cpuset cgroup (v2)'"; \
		echo "  • 'container exited with code 1 or 255'"; \
		echo ""; \
		echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; \
		echo "✅ RECOMMENDED: Use pre-built envtest instead (works in all environments)"; \
		echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; \
		echo ""; \
		echo "  make test-integration"; \
		echo ""; \
		echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; \
		echo "🔧 ALTERNATIVE: Switch Podman to rootful mode"; \
		echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; \
		echo ""; \
		echo "  # Stop Podman machine and switch to rootful mode with adequate resources"; \
		echo "  podman machine stop"; \
		echo "  podman machine set --rootful=true --cpus 4 --memory 4096"; \
		echo "  podman machine start"; \
		echo ""; \
		echo "  # Verify it's rootful"; \
		echo "  podman machine inspect --format '{{.Rootful}}'  # Should output: true"; \
		echo ""; \
		echo "  # Then run K3s tests"; \
		echo "  make test-integration-k3s"; \
		echo ""; \
		echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; \
		echo ""; \
		echo "⚠️  Stopping here to prevent K3s failures. This is not a build error!"; \
		false; \
	elif [ "$$ROOTFUL" = "true" ]; then \
		echo "   ✅ Podman is in ROOTFUL mode (compatible with K3s)"; \
	else \
		echo "   ⚠️  Could not determine Podman mode (machine may not be running)"; \
	fi
endif
	@echo ""
	@echo "🚀 Starting K3s integration tests..."
	@echo "   Strategy: K3s (testcontainers)"
	@echo "   Note: This may require privileged containers"
	@echo "   Note: K3s startup takes 30-60 seconds"
ifeq ($(CONTAINER_RUNTIME),podman)
	@echo "📡 Detecting proxy configuration from Podman machine..."
	@echo "   Setting TESTCONTAINERS_RYUK_DISABLED=true (Podman compatibility)"
	@PROXY_HTTP=$$(podman machine ssh 'echo $$HTTP_PROXY' 2>/dev/null); \
	PROXY_HTTPS=$$(podman machine ssh 'echo $$HTTPS_PROXY' 2>/dev/null); \
	echo ""; \
	if [ -n "$$PROXY_HTTP" ] || [ -n "$$PROXY_HTTPS" ]; then \
		echo "   Using HTTP_PROXY=$$PROXY_HTTP"; \
		echo "   Using HTTPS_PROXY=$$PROXY_HTTPS"; \
		HTTP_PROXY=$$PROXY_HTTP HTTPS_PROXY=$$PROXY_HTTPS INTEGRATION_STRATEGY=k3s TESTCONTAINERS_RYUK_DISABLED=true TESTCONTAINERS_LOG_LEVEL=INFO $(GOTEST) -v -count=1 -tags=integration ./test/integration/... -timeout $(TEST_TIMEOUT) || exit 1; \
	else \
		INTEGRATION_STRATEGY=k3s TESTCONTAINERS_RYUK_DISABLED=true TESTCONTAINERS_LOG_LEVEL=INFO $(GOTEST) -v -count=1 -tags=integration ./test/integration/... -timeout $(TEST_TIMEOUT) || exit 1; \
	fi
else
	@echo ""; \
	INTEGRATION_STRATEGY=k3s $(GOTEST) -v -count=1 -tags=integration ./test/integration/... -timeout $(TEST_TIMEOUT) || exit 1
endif
	@echo ""
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "✅ K3s integration tests passed!"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
endif

.PHONY: test-all
test-all: test test-integration lint## ✅ Run ALL tests (unit + integration)
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "✅ All tests completed successfully!"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

.PHONY: lint
lint: ## Run golangci-lint
	@echo "Running golangci-lint..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "Error: golangci-lint not found. Please install it:"; \
		echo "  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi

.PHONY: fmt
fmt: ## Format code with gofmt and goimports
	@echo "Formatting code..."
	@if command -v $(GOIMPORTS) > /dev/null; then \
		$(GOIMPORTS) -w .; \
	else \
		$(GOFMT) -w .; \
	fi

.PHONY: mod-tidy
mod-tidy: ## Tidy Go module dependencies
	@echo "Tidying Go modules..."
	$(GOMOD) tidy
	$(GOMOD) verify

.PHONY: binary
binary: ## Build binary
	@echo "Building $(PROJECT_NAME)..."
	@echo "Version: $(VERSION), Commit: $(GIT_COMMIT), BuildDate: $(BUILD_DATE)"
	@mkdir -p bin
	CGO_ENABLED=0 $(GOBUILD) -ldflags="$(LDFLAGS)" -o bin/$(PROJECT_NAME) ./cmd/adapter

.PHONY: clean
clean: ## Clean build artifacts and test coverage files
	@echo "Cleaning..."
	rm -rf bin/
	rm -f $(COVERAGE_OUT) $(COVERAGE_HTML)

.PHONY: image
image: ## Build container image with Docker or Podman
ifeq ($(CONTAINER_RUNTIME),none)
	@echo "❌ ERROR: No container runtime found"
	@echo "Please install Docker or Podman"
	@exit 1
else
	@echo "Building container image with $(CONTAINER_RUNTIME)..."
	$(CONTAINER_CMD) build -t $(PROJECT_NAME):$(VERSION) .
	@echo "✅ Image built: $(PROJECT_NAME):$(VERSION)"
endif

.PHONY: verify
verify: lint test ## Run all verification checks (lint + test)

