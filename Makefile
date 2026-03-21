# DevContext AI - Makefile
# Build and manage the DevContext mono-repo

.PHONY: build run-daemon install-local test clean help

# Default target
.DEFAULT_GOAL := help

# Variables
CLI_DIR := cli
DAEMON_DIR := daemon
BIN_DIR := bin
BINARY_NAME := devctx
INSTALL_PATH := /usr/local/bin/$(BINARY_NAME)

# Build the Go CLI binary
build:
	@echo "Building DevContext CLI..."
	@mkdir -p $(BIN_DIR)
	cd $(CLI_DIR) && go build -o ../$(BIN_DIR)/$(BINARY_NAME) .
	@echo "Binary built at $(BIN_DIR)/$(BINARY_NAME)"

# Run the Spring Boot daemon
run-daemon:
	@echo "Starting DevContext daemon..."
	cd $(DAEMON_DIR) && mvn spring-boot:run

# Install the CLI binary to /usr/local/bin
install-local: build
	@echo "Installing DevContext CLI to $(INSTALL_PATH)..."
	@sudo cp $(BIN_DIR)/$(BINARY_NAME) $(INSTALL_PATH)
	@sudo chmod +x $(INSTALL_PATH)
	@echo "Installed successfully! Run 'devctx --help' to get started."

# Run all tests
test:
	@echo "Running CLI tests..."
	cd $(CLI_DIR) && go test -v ./...
	@echo ""
	@echo "Running daemon tests..."
	cd $(DAEMON_DIR) && mvn test

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BIN_DIR)
	cd $(CLI_DIR) && go clean
	cd $(DAEMON_DIR) && mvn clean
	@echo "Clean complete."

# Download dependencies
deps:
	@echo "Downloading CLI dependencies..."
	cd $(CLI_DIR) && go mod download
	@echo ""
	@echo "Downloading daemon dependencies..."
	cd $(DAEMON_DIR) && mvn dependency:resolve
	@echo "Dependencies downloaded."

# Format code
fmt:
	@echo "Formatting Go code..."
	cd $(CLI_DIR) && go fmt ./...

# Lint code
lint:
	@echo "Linting Go code..."
	cd $(CLI_DIR) && go vet ./...

# Show help
help:
	@echo "DevContext AI - Build System"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build         Compile Go CLI binary to bin/devctx"
	@echo "  run-daemon    Start Spring Boot daemon via mvn spring-boot:run"
	@echo "  install-local Copy binary to /usr/local/bin/devctx (requires sudo)"
	@echo "  test          Run all tests (CLI and daemon)"
	@echo "  clean         Remove build artifacts"
	@echo "  deps          Download all dependencies"
	@echo "  fmt           Format Go code"
	@echo "  lint          Lint Go code"
	@echo "  release       Build release artifacts for all platforms"
	@echo "  package-daemon Package Spring Boot daemon JAR"
	@echo "  help          Show this help message"

# Build release for all platforms
release:
	@echo "Building release..."
	./scripts/build-release.sh $(VERSION)

# Package Spring Boot daemon JAR
package-daemon:
	@echo "Packaging daemon..."
	cd $(DAEMON_DIR) && mvn package -DskipTests
	@mkdir -p $(BIN_DIR)
	cp $(DAEMON_DIR)/target/*.jar $(BIN_DIR)/daemon.jar
	@echo "Daemon packaged at $(BIN_DIR)/daemon.jar"

# Full release (CLI + daemon)
release-all: release package-daemon
	@echo "Full release complete!"
