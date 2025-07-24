# Grafana ArangoDB Datasource Plugin Makefile

.PHONY: all build frontend backend clean dev install test linux linux-amd64 linux-arm64

# Plugin information
PLUGIN_NAME = arangodb-datasource
PLUGIN_ID = arangodb-datasource

# Directories
DIST_DIR = dist
PKG_DIR = pkg
SRC_DIR = src

# Build targets
FRONTEND_TARGET = $(DIST_DIR)/module.js
BACKEND_TARGET = $(DIST_DIR)/gpx_$(PLUGIN_NAME)

# Default target
all: clean build

# Build both frontend and backend
build: frontend backend

# Build frontend
frontend:
	@echo "Building frontend..."
	npm install
	npm run build

# Build backend for current platform
backend:
	@echo "Building backend for current platform..."
	mkdir -p $(DIST_DIR)
	go mod tidy
	cd $(PKG_DIR) && go build -o ../$(BACKEND_TARGET) .

# Build backend for all platforms
backend-all:
	@echo "Building backend for all platforms..."
	mkdir -p $(DIST_DIR)
	go mod tidy
	# Linux AMD64
	cd $(PKG_DIR) && GOOS=linux GOARCH=amd64 go build -o ../$(DIST_DIR)/gpx_arangodb-datasource_linux_amd64 .
	# Linux ARM64
	cd $(PKG_DIR) && GOOS=linux GOARCH=arm64 go build -o ../$(DIST_DIR)/gpx_arangodb-datasource_linux_arm64 .
	# Windows AMD64
	cd $(PKG_DIR) && GOOS=windows GOARCH=amd64 go build -o ../$(DIST_DIR)/gpx_arangodb-datasource_windows_amd64.exe .

# Build for Linux platforms only
linux: linux-amd64 linux-arm64

# Build for Linux x64
linux-amd64:
	@echo "Building for Linux x64..."
	mkdir -p $(DIST_DIR)
	cd $(PKG_DIR) && GOOS=linux GOARCH=amd64 go build -o ../$(DIST_DIR)/gpx_arangodb-datasource_linux_amd64 .

# Build for Linux ARM64
linux-arm64:
	@echo "Building for Linux ARM64..."
	mkdir -p $(DIST_DIR)
	cd $(PKG_DIR) && GOOS=linux GOARCH=arm64 go build -o ../$(DIST_DIR)/gpx_arangodb-datasource_linux_arm64 .
	cd $(PKG_DIR) && GOOS=darwin GOARCH=arm64 go build -o ../$(DIST_DIR)/gpx_$(PLUGIN_NAME)_darwin_arm64 .

# Development mode with file watching
dev:
	@echo "Starting development mode..."
	npm run dev

# Install dependencies
install:
	@echo "Installing dependencies..."
	npm install
	go mod tidy

# Run tests
test:
	@echo "Running tests..."
	npm test
	cd $(PKG_DIR) && go test ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(DIST_DIR)
	rm -rf node_modules/.cache
	go clean

# Package the plugin for distribution
package: clean build
	@echo "Packaging plugin..."
	mkdir -p $(DIST_DIR)
	cp plugin.json $(DIST_DIR)/
	cp README.md $(DIST_DIR)/
	cp -r img $(DIST_DIR)/ || true
	cd $(DIST_DIR) && zip -r $(PLUGIN_NAME).zip .

# Sign the plugin (requires Grafana signing key)
sign:
	@echo "Signing plugin..."
	npx @grafana/sign-plugin@latest

# Install plugin to local Grafana
install-plugin:
	@echo "Installing plugin to local Grafana..."
	@if [ -z "$(GRAFANA_PLUGINS_DIR)" ]; then \
		echo "Error: GRAFANA_PLUGINS_DIR environment variable is not set"; \
		echo "Please set it to your Grafana plugins directory (e.g., /var/lib/grafana/plugins)"; \
		exit 1; \
	fi
	rm -rf $(GRAFANA_PLUGINS_DIR)/$(PLUGIN_NAME)
	cp -r $(DIST_DIR) $(GRAFANA_PLUGINS_DIR)/$(PLUGIN_NAME)
	@echo "Plugin installed. Restart Grafana to load the plugin."

# Help target
help:
	@echo "Available targets:"
	@echo "  all              - Clean and build both frontend and backend"
	@echo "  build            - Build both frontend and backend"
	@echo "  frontend         - Build only the frontend"
	@echo "  backend          - Build backend for current platform"
	@echo "  backend-all      - Build backend for all platforms"
	@echo "  dev              - Start development mode with file watching"
	@echo "  install          - Install dependencies"
	@echo "  test             - Run tests"
	@echo "  clean            - Clean build artifacts"
	@echo "  package          - Package plugin for distribution"
	@echo "  sign             - Sign the plugin"
	@echo "  install-plugin   - Install plugin to local Grafana (requires GRAFANA_PLUGINS_DIR)"
	@echo "  help             - Show this help message"
