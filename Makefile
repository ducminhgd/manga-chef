# Manga Chef — Makefile
#
# Usage:
#   make build       Build the binary
#   make test        Run unit tests
#   make test-race   Run unit tests with race detector
#   make lint        Run golangci-lint
#   make fmt         Format all Go files
#   make generate    Regenerate mocks (requires mockery v2.40+)
#   make clean       Remove build artifacts
#   make check       Run fmt-check + lint + test (full pre-push gate)

# ── Variables ──────────────────────────────────────────────────────────────────
BINARY         := manga-chef
CMD_PATH       := ./cmd/manga-chef
BUILD_DIR      := ./bin

# Inject version info at build time from git
VERSION        := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT         := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE     := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS        := -ldflags="-s -w \
                    -X github.com/manga-chef/manga-chef/internal/version.Version=$(VERSION) \
                    -X github.com/manga-chef/manga-chef/internal/version.Commit=$(COMMIT) \
                    -X github.com/manga-chef/manga-chef/internal/version.BuildDate=$(BUILD_DATE)"

# Go toolchain
GO             := go
GOFLAGS        :=
COVERPROFILE   := coverage.out

# Linting
GOLANGCI_LINT  := golangci-lint
GOLANGCI_VER   := v1.57.2

# Mocks
MOCKERY        := mockery

# ── Phony targets ──────────────────────────────────────────────────────────────
.PHONY: all build test test-race test-integration lint fmt fmt-check \
        generate clean check install-tools help

# Default target
all: check build

# ── Build ──────────────────────────────────────────────────────────────────────
build:  ## Build the manga-chef binary into ./bin/
	@echo "→ Building $(BINARY) $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) $(CMD_PATH)
	@echo "✓ Binary: $(BUILD_DIR)/$(BINARY)"

build-all:  ## Cross-compile for Linux, macOS, and Windows (amd64 + arm64)
	@echo "→ Cross-compiling for all platforms..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux   GOARCH=amd64  $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-amd64   $(CMD_PATH)
	GOOS=linux   GOARCH=arm64  $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-arm64   $(CMD_PATH)
	GOOS=darwin  GOARCH=amd64  $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-amd64  $(CMD_PATH)
	GOOS=darwin  GOARCH=arm64  $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-arm64  $(CMD_PATH)
	GOOS=windows GOARCH=amd64  $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-windows-amd64.exe $(CMD_PATH)
	@echo "✓ All binaries written to $(BUILD_DIR)/"

# ── Test ───────────────────────────────────────────────────────────────────────
test:  ## Run unit tests (excludes integration tests)
	@echo "→ Running unit tests..."
	$(GO) test $(GOFLAGS) -count=1 ./... -coverprofile=$(COVERPROFILE)
	@echo "✓ Tests passed. Coverage report: $(COVERPROFILE)"

test-race:  ## Run unit tests with the race detector enabled
	@echo "→ Running unit tests (race detector)..."
	$(GO) test $(GOFLAGS) -race -count=1 ./...
	@echo "✓ Race detector tests passed."

test-integration:  ## Run integration tests (requires network access)
	@echo "→ Running integration tests..."
	$(GO) test $(GOFLAGS) -tags=integration -count=1 -timeout=120s ./...
	@echo "✓ Integration tests passed."

coverage:  ## Show HTML coverage report in the browser
	@$(MAKE) test
	$(GO) tool cover -html=$(COVERPROFILE)

# ── Lint ───────────────────────────────────────────────────────────────────────
lint:  ## Run golangci-lint (must be installed: make install-tools)
	@echo "→ Running golangci-lint..."
	$(GOLANGCI_LINT) run ./...
	@echo "✓ Lint passed."

lint-fix:  ## Run golangci-lint with --fix to auto-correct fixable issues
	@echo "→ Running golangci-lint --fix..."
	$(GOLANGCI_LINT) run --fix ./...

# ── Formatting ────────────────────────────────────────────────────────────────
fmt:  ## Format all Go files with gofmt
	@echo "→ Formatting Go files..."
	gofmt -w .
	@echo "✓ All files formatted."

fmt-check:  ## Check formatting without modifying files (used in CI)
	@echo "→ Checking Go formatting..."
	@./scripts/check-fmt.sh
	@echo "✓ Formatting check passed."

# ── Code Generation ───────────────────────────────────────────────────────────
generate:  ## Regenerate mocks using mockery (requires: go install github.com/vektra/mockery/v2@latest)
	@echo "→ Regenerating mocks..."
	$(GO) generate ./...
	@echo "✓ Mocks regenerated."

# ── Full pre-push gate ────────────────────────────────────────────────────────
check: fmt-check lint test-race  ## Run all checks (fmt + lint + test with race detector)
	@echo ""
	@echo "✓ All checks passed. Ready to push."

# ── Cleanup ───────────────────────────────────────────────────────────────────
clean:  ## Remove build artifacts and coverage reports
	@echo "→ Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR) $(COVERPROFILE)
	@echo "✓ Clean."

# ── Tool Installation ─────────────────────────────────────────────────────────
install-tools:  ## Install required dev tools (golangci-lint, mockery)
	@echo "→ Installing golangci-lint $(GOLANGCI_VER)..."
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
	  | sh -s -- -b $(shell go env GOPATH)/bin $(GOLANGCI_VER)
	@echo "→ Installing mockery..."
	$(GO) install github.com/vektra/mockery/v2@latest
	@echo "✓ Dev tools installed."

# ── Help ──────────────────────────────────────────────────────────────────────
help:  ## Show this help message
	@echo "Manga Chef — available make targets:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
	  | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""