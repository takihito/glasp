.PHONY: all test build install clean

# Default target
all: build

# Output directory for the executable
BUILD_DIR := bin
EXEC_NAME := glasp
EXEC_PATH := $(BUILD_DIR)/$(EXEC_NAME)

# Read .env file into make variables (KEY=VALUE format).
ifneq (,$(wildcard .env))
  include .env
  export GLASP_CLIENT_ID GLASP_CLIENT_SECRET
endif

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

LDFLAGS := -X 'main.Version=$(VERSION)' -X 'main.Commit=$(COMMIT)' -X 'main.Date=$(DATE)' \
           -X 'glasp/internal/auth.ldflagsClientID=$(GLASP_CLIENT_ID)' -X 'glasp/internal/auth.ldflagsClientSecret=$(GLASP_CLIENT_SECRET)'

# Test target
test:
	@echo "Running tests..."
	@if [ -f .env ]; then export $$(grep -v '^ # ' .env | xargs) ; fi; go test -v ./...

# Build target
build: $(BUILD_DIR)
	@echo "Building $(EXEC_NAME)..."
	go build -ldflags "$(LDFLAGS)" -o $(EXEC_PATH) ./cmd/glasp

# Create build directory if it doesn't exist
$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

# Install target
install: build
	@echo "Installing $(EXEC_NAME)..."
	go install -ldflags "$(LDFLAGS)" ./cmd/glasp

# Clean target
clean:
	@echo "Cleaning up..."
	rm -f $(EXEC_PATH)
	go clean -testcache
