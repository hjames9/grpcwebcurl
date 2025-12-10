# grpcwebcurl Makefile

# Binary name
BINARY_NAME=grpcwebcurl

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOVET=$(GOCMD) vet
GOFMT=gofmt
GHCMD=gh

# Build directory
BUILD_DIR=build

# Version info (read from VERSION file)
VERSION=$(shell cat VERSION 2>/dev/null || echo "0.0.0")
VERSION_MAJOR=$(shell echo $(VERSION) | cut -d. -f1)
VERSION_MINOR=$(shell echo $(VERSION) | cut -d. -f2)
VERSION_PATCH=$(shell echo $(VERSION) | cut -d. -f3)
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.gitCommit=$(GIT_COMMIT) -X main.buildTime=$(BUILD_TIME)"

# Platforms for cross-compilation
PLATFORMS=linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

# GitHub repository (for releases)
GITHUB_REPO?=hjames9/grpcwebcurl

.PHONY: all build build-all test test-verbose test-coverage vet fmt fmt-check \
        tidy deps install clean run version bump-major bump-minor bump-patch \
        release release-publish help

all: fmt vet test build ## Build after formatting, vetting, and testing

build: ## Build the binary to build/ directory
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/grpcwebcurl/

build-all: clean ## Build for all platforms (cross-compile)
	@echo "Building for all platforms..."
	@mkdir -p $(BUILD_DIR)
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*} GOARCH=$${platform#*/} \
		$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-$${platform%/*}-$${platform#*/}$$([ "$${platform%/*}" = "windows" ] && echo ".exe") \
		./cmd/grpcwebcurl/ || exit 1; \
		echo "  Built: $(BUILD_DIR)/$(BINARY_NAME)-$${platform%/*}-$${platform#*/}"; \
	done

test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) ./...

test-verbose: ## Run tests with verbose output
	@echo "Running tests (verbose)..."
	$(GOTEST) -v ./...

test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	@mkdir -p $(BUILD_DIR)
	$(GOTEST) -coverprofile=$(BUILD_DIR)/coverage.out ./...
	$(GOCMD) tool cover -html=$(BUILD_DIR)/coverage.out -o $(BUILD_DIR)/coverage.html
	@echo "Coverage report generated: $(BUILD_DIR)/coverage.html"

vet: ## Run go vet
	@echo "Running go vet..."
	$(GOVET) ./...

fmt: ## Format code
	@echo "Formatting code..."
	$(GOFMT) -s -w .

fmt-check: ## Check code formatting (for CI)
	@echo "Checking code formatting..."
	@test -z "$$($(GOFMT) -s -l . | tee /dev/stderr)" || (echo "Code is not formatted. Run 'make fmt'" && exit 1)

tidy: ## Tidy go.mod dependencies
	@echo "Tidying dependencies..."
	$(GOMOD) tidy

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download

install: ## Install binary to GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	$(GOCMD) install $(LDFLAGS) ./cmd/grpcwebcurl/

clean: ## Remove build artifacts
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)

run: build ## Build and run with --help
	$(BUILD_DIR)/$(BINARY_NAME) --help

version: ## Show current version
	@echo "Current version: $(VERSION)"
	@echo "  Major: $(VERSION_MAJOR)"
	@echo "  Minor: $(VERSION_MINOR)"
	@echo "  Patch: $(VERSION_PATCH)"

bump-major: ## Bump major version (X.0.0)
	@echo "Bumping major version..."
	@NEW_VERSION="$$(($(VERSION_MAJOR) + 1)).0.0"; \
	echo "$$NEW_VERSION" > VERSION; \
	echo "Version bumped to $$NEW_VERSION"

bump-minor: ## Bump minor version (x.X.0)
	@echo "Bumping minor version..."
	@NEW_VERSION="$(VERSION_MAJOR).$$(($(VERSION_MINOR) + 1)).0"; \
	echo "$$NEW_VERSION" > VERSION; \
	echo "Version bumped to $$NEW_VERSION"

bump-patch: ## Bump patch version (x.x.X)
	@echo "Bumping patch version..."
	@NEW_VERSION="$(VERSION_MAJOR).$(VERSION_MINOR).$$(($(VERSION_PATCH) + 1))"; \
	echo "$$NEW_VERSION" > VERSION; \
	echo "Version bumped to $$NEW_VERSION"

release: clean ## Create draft GitHub release with binaries
	@echo "Building release binaries for v$(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*} GOARCH=$${platform#*/} \
		$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-v$(VERSION)-$${platform%/*}-$${platform#*/}$$([ "$${platform%/*}" = "windows" ] && echo ".exe") \
		./cmd/grpcwebcurl/ || exit 1; \
		echo "  Built: $(BUILD_DIR)/$(BINARY_NAME)-v$(VERSION)-$${platform%/*}-$${platform#*/}"; \
	done
	@echo ""
	@echo "Creating draft GitHub release v$(VERSION)..."
	@$(GHCMD) release create v$(VERSION) \
		--repo $(GITHUB_REPO) \
		--title "v$(VERSION)" \
		--notes "Release v$(VERSION)" \
		--draft \
		$(BUILD_DIR)/*
	@echo ""
	@echo "Draft release v$(VERSION) created!"
	@echo "Review and publish with: make release-publish"

release-publish: ## Publish the draft release and push git tag
	@echo "Creating and pushing git tag v$(VERSION)..."
	@if git rev-parse v$(VERSION) >/dev/null 2>&1; then \
		echo "  Tag v$(VERSION) already exists locally"; \
	else \
		git tag -a v$(VERSION) -m "Release v$(VERSION)"; \
		echo "  Created tag v$(VERSION)"; \
	fi
	@git push origin v$(VERSION) 2>/dev/null || echo "  Tag v$(VERSION) already pushed or push failed"
	@echo ""
	@echo "Publishing GitHub release v$(VERSION)..."
	@$(GHCMD) release edit v$(VERSION) \
		--repo $(GITHUB_REPO) \
		--draft=false
	@echo ""
	@echo "Release v$(VERSION) published!"
	@echo "View at: https://github.com/$(GITHUB_REPO)/releases/tag/v$(VERSION)"

help: ## Show this help message
	@echo "$(BINARY_NAME) - Makefile help"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Build info:"
	@echo "  Version:    $(VERSION)"
	@echo "  Git Commit: $(GIT_COMMIT)"
	@echo "  Build Time: $(BUILD_TIME)"
