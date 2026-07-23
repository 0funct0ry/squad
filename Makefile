# Project variables
BINARY_NAME=squad
BIN_DIR=bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT_SHA ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
LDFLAGS=-ldflags "-X github.com/0funct0ry/squad/cmd.Version=$(VERSION) -X github.com/0funct0ry/squad/cmd.CommitSHA=$(COMMIT_SHA)"

# Go commands
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
GOFMT=gofmt

.PHONY: all build clean test vet fmt help

all: help

## build: Build the binary
build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) main.go

## clean: Remove build artifacts
clean: ## Remove build artifacts
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BIN_DIR)

## test: Run tests
test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) -v ./...

## vet: Run go vet
vet: ## Run go vet
	@echo "Running go vet..."
	$(GOVET) ./...

## fmt: Run go fmt
fmt: ## Run go fmt
	@echo "Formatting code..."
	$(GOFMT) -w .

## help: Show this help message
help: ## Show this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'
