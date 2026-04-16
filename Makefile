# Copyright 2024 Wondermove Inc.
# SPDX-License-Identifier: Apache-2.0

BINARY_NAME := k-o11y-otelcol
VERSION := 0.109.0.1
GO := go
GOFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

# Docker settings
DOCKER_REGISTRY ?= ghcr.io/wondermove-inc
DOCKER_IMAGE := $(DOCKER_REGISTRY)/k-o11y-otel-collector-contrib
DOCKER_TAG := $(VERSION)

.PHONY: all build clean test lint docker docker-push

all: build

# Build the binary for current platform
build:
	$(GO) build $(GOFLAGS) -o bin/$(BINARY_NAME) ./cmd/otelcol

# Build for all platforms
build-all:
	@for platform in $(PLATFORMS); do \
		GOOS=$$(echo $$platform | cut -d'/' -f1) \
		GOARCH=$$(echo $$platform | cut -d'/' -f2) \
		$(GO) build $(GOFLAGS) -o bin/$(BINARY_NAME)-$$GOOS-$$GOARCH ./cmd/otelcol; \
	done

# Run tests
test:
	$(GO) test -v -race ./...

# Run tests with coverage
test-coverage:
	$(GO) test -v -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

# Run linter
lint:
	golangci-lint run ./...

# Download dependencies
deps:
	$(GO) mod download
	$(GO) mod tidy

# Verify dependencies
verify:
	$(GO) mod verify

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Build Docker image (multi-arch)
docker:
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		-t $(DOCKER_IMAGE):latest \
		--push \
		.

# Build Docker image for local testing
docker-local:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

# Push Docker image
docker-push:
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_IMAGE):latest

# Run locally with a config file
run:
	$(GO) run ./cmd/otelcol --config=config/config.yaml

# Generate mocks (if needed)
generate:
	$(GO) generate ./...

# Format code
fmt:
	$(GO) fmt ./...
	goimports -w .

# Help
help:
	@echo "Available targets:"
	@echo "  build        - Build binary for current platform"
	@echo "  build-all    - Build binaries for all platforms"
	@echo "  test         - Run tests"
	@echo "  test-coverage- Run tests with coverage"
	@echo "  lint         - Run linter"
	@echo "  deps         - Download and tidy dependencies"
	@echo "  clean        - Clean build artifacts"
	@echo "  docker       - Build and push multi-arch Docker image"
	@echo "  docker-local - Build Docker image locally"
	@echo "  run          - Run locally with config file"
	@echo "  fmt          - Format code"
