# Makefile for verifi

# Build variables
BINARY_NAME=verifi
BUILD_DIR=.
VERSION?=dev
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Go build flags
LDFLAGS=-ldflags "-X github.com/princespaghetti/verifi/internal/cli.Version=$(VERSION) \
	-X github.com/princespaghetti/verifi/internal/cli.GitCommit=$(GIT_COMMIT) \
	-X github.com/princespaghetti/verifi/internal/cli.BuildDate=$(BUILD_DATE)"

.PHONY: all build install test test-coverage test-verbose lint fmt clean clean-store help

# Default target
all: build

# Build the CLI binary
build:
	@echo "Building $(BINARY_NAME)..."
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) cmd/verifi/main.go

# Install the CLI locally for testing
install:
	@echo "Installing $(BINARY_NAME)..."
	go install $(LDFLAGS) ./cmd/verifi

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

# Run all tests
test:
	@echo "Running tests..."
	go test ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -cover ./...

# Generate coverage report
coverage:
	@echo "Generating coverage report..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# Run tests with verbose output
test-verbose:
	@echo "Running tests (verbose)..."
	go test -v ./...

# Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Install from https://golangci-lint.run/usage/install/" && exit 1)
	golangci-lint run

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	gofmt -w .

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BINARY_NAME)
	rm -f coverage.out

# Clean the certificate store (useful for local development/testing)
clean-store:
	@echo "Removing certificate store at ~/.verifi..."
	@rm -rf ~/.verifi
	@echo "Certificate store removed"

# Display help
help:
	@echo "Available targets:"
	@echo "  build          - Build the CLI binary"
	@echo "  install        - Install the CLI locally"
	@echo "  deps           - Download and tidy dependencies"
	@echo "  test           - Run all tests"
	@echo "  test-coverage  - Run tests with coverage summary"
	@echo "  coverage       - Generate HTML coverage report"
	@echo "  test-verbose   - Run tests with verbose output"
	@echo "  lint           - Run golangci-lint"
	@echo "  fmt            - Format code with go fmt"
	@echo "  clean          - Remove build artifacts"
	@echo "  clean-store    - Remove ~/.verifi certificate store (for local dev/testing)"
	@echo "  help           - Display this help message"
