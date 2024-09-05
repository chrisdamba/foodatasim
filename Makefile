# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=bin/foodatasim
BINARY_UNIX=$(BINARY_NAME)_unix

# Main package path
MAIN_PACKAGE=github.com/chrisdamba/foodatasim

# Build the project
all: test build

build:
	$(GOBUILD) -o $(BINARY_NAME) -v $(MAIN_PACKAGE)

test:
	$(GOTEST) -v ./...

# Clean build files
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)

# Run the application
run:
	$(GOBUILD) -o $(BINARY_NAME) -v $(MAIN_PACKAGE)
	./$(BINARY_NAME)

# Cross compilation for Linux
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_UNIX) -v $(MAIN_PACKAGE)

# Download dependencies
deps:
	$(GOGET) -v -t -d ./...
	$(GOMOD) tidy

# Update dependencies
update-deps:
	$(GOGET) -u -v -t -d ./...
	$(GOMOD) tidy

# Format all Go files
fmt:
	$(GOCMD) fmt ./...

# Run linter
lint:
	golangci-lint run

# Generate mocks
mocks:
	mockgen -source=internal/simulator/simulator.go -destination=internal/mocks/mock_simulator.go -package=mocks

# Build and run with race detection
race:
	$(GOBUILD) -race -o $(BINARY_NAME) -v $(MAIN_PACKAGE)
	./$(BINARY_NAME)

.PHONY: all build test clean run build-linux deps update-deps fmt lint mocks race