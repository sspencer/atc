BINARY  := atc
BIN_DIR := bin
GO      ?= go

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/sspencer/atc/commands.version=$(VERSION)

.DEFAULT_GOAL := build

.PHONY: help build run install test vet fmt tidy check clean

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

build: ## Build the atc binary into bin/
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) .

run: build ## Build and run atc (pass args with ARGS="...")
	./$(BIN_DIR)/$(BINARY) $(ARGS)

install: ## Install atc into GOBIN
	$(GO) install -ldflags "$(LDFLAGS)" .

test: ## Run tests with the race detector
	$(GO) test -race ./...

vet: ## Run go vet
	$(GO) vet ./...

fmt: ## Format source with gofmt
	gofmt -s -w .

tidy: ## Tidy go.mod and go.sum
	$(GO) mod tidy

check: vet test ## Run vet and tests
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed on:"; gofmt -l .; exit 1)

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR)
