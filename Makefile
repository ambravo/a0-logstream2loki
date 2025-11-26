.PHONY: build run test clean docker-build docker-run help

# Binary name
BINARY_NAME=a0-logstream2loki

# Build variables
GO=go
GOFLAGS=-v
LDFLAGS=-ldflags "-s -w"

## help: Display this help message
help:
	@echo "Available targets:"
	@grep -E '^## ' Makefile | sed 's/## /  /'

## build: Build the binary
build:
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BINARY_NAME)

## run: Run the application
run:
	$(GO) run .

## test: Run tests
test:
	$(GO) test -v ./...

## test-race: Run tests with race detector
test-race:
	$(GO) test -v -race ./...

## clean: Remove build artifacts
clean:
	rm -f $(BINARY_NAME)
	$(GO) clean

## docker-build: Build Docker image
docker-build:
	docker build -t $(BINARY_NAME):latest .

## docker-run: Run Docker container
docker-run:
	docker run -p 8080:8080 \
		-e LOKI_URL=http://loki:3100 \
		-e HMAC_SECRET=your-secret-key \
		$(BINARY_NAME):latest

## install: Install the binary
install:
	$(GO) install $(LDFLAGS)

## fmt: Format code
fmt:
	$(GO) fmt ./...

## vet: Run go vet
vet:
	$(GO) vet ./...

## lint: Run golangci-lint (requires golangci-lint to be installed)
lint:
	golangci-lint run

.DEFAULT_GOAL := help
