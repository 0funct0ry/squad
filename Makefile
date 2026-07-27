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

# Chrome/Chromium binary VHS drives to render terminal-session GIFs.
# Override on the command line if yours lives elsewhere, e.g.:
#   make site-casts VHS_CHROME_PATH=/usr/bin/chromium
VHS_CHROME_PATH ?= /Applications/Google Chrome.app/Contents/MacOS/Google Chrome

.PHONY: all build clean test vet fmt help web site site-clean site-preview site-casts

all: help

web:
	cd web && npm run build

## site: Build the marketing + docs site (site/)
site: ## Build the marketing + docs site (site/)
	@echo "Building site..."
	cd site && npm ci && npm run build

## site-clean: Remove the site's build artifacts
site-clean: ## Remove the site's build artifacts
	@echo "Cleaning site..."
	rm -rf site/dist site/.astro

## site-preview: Build the site and serve it locally in preview mode
site-preview: site ## Build the site and serve it locally in preview mode
	cd site && npm run preview

## site-casts: Regenerate the squad cli terminal-demo GIFs from their VHS tapes
site-casts: build ## Regenerate the squad cli terminal-demo GIFs from their VHS tapes
	@command -v vhs >/dev/null 2>&1 || { echo "vhs not found — install with: brew install vhs ffmpeg"; exit 1; }
	@echo "Regenerating site casts..."
	cp examples/ecommerce.db /tmp/cli-demo-basics.db
	cp examples/ecommerce.db /tmp/cli-demo-dot.db
	cp examples/ecommerce.db /tmp/cli-demo-tpl.db
	VHS_CHROME_PATH="$(VHS_CHROME_PATH)" vhs site/scripts/vhs/cli-basics.tape
	VHS_CHROME_PATH="$(VHS_CHROME_PATH)" vhs site/scripts/vhs/cli-dotcommands.tape
	VHS_CHROME_PATH="$(VHS_CHROME_PATH)" vhs site/scripts/vhs/cli-templates.tape
	rm -f /tmp/cli-demo-basics.db /tmp/cli-demo-dot.db /tmp/cli-demo-tpl.db

## build: Build the binary
build: web ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	mkdir -p $(BIN_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) main.go

## clean: Remove build artifacts
clean: site-clean ## Remove build artifacts
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BIN_DIR) web/dist $(BINARY_NAME)


## test: Run tests
test: web ## Run tests
	@echo "Running tests..."
	$(GOTEST) -v ./...

## vet: Run go vet
vet: web ## Run go vet
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
