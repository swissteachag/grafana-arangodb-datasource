#!/bin/bash

# Grafana ArangoDB Datasource Build Script for Linux
echo "Setting up Grafana ArangoDB Datasource..."

# Check if Node.js is installed
if ! command -v node &> /dev/null; then
    echo "Error: Node.js is not installed"
    echo "Please install Node.js from https://nodejs.org/"
    exit 1
fi

echo "Node.js version: $(node --version)"

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed"
    echo "Please install Go from https://golang.org/dl/"
    exit 1
fi

echo "Go version: $(go version)"

# Install Node.js dependencies
echo "Installing Node.js dependencies..."
npm install

if [ $? -ne 0 ]; then
    echo "Failed to install Node.js dependencies"
    exit 1
fi

# Initialize Go module and download dependencies
echo "Setting up Go dependencies..."
go mod tidy

if [ $? -ne 0 ]; then
    echo "Failed to set up Go dependencies"
    exit 1
fi

# Build the plugin
echo "Building the plugin..."

# Build frontend
echo "Building frontend..."
npm run build

if [ $? -ne 0 ]; then
    echo "Failed to build frontend"
    exit 1
fi

# Build backend for multiple platforms
echo "Building backend for multiple platforms..."
mkdir -p dist

cd pkg

# Build for Linux x64
echo "Building for Linux x64..."
GOOS=linux GOARCH=amd64 go build -o "../dist/gpx_arangodb-datasource_linux_amd64" .

if [ $? -ne 0 ]; then
    echo "Failed to build for Linux x64"
    cd ..
    exit 1
fi

# Build for Linux ARM64
echo "Building for Linux ARM64..."
GOOS=linux GOARCH=arm64 go build -o "../dist/gpx_arangodb-datasource_linux_arm64" .

if [ $? -ne 0 ]; then
    echo "Failed to build for Linux ARM64"
    cd ..
    exit 1
fi

cd ..

# Copy plugin.json to dist
cp plugin.json dist/
cp TROUBLESHOOTING.md dist/

echo "Plugin built successfully!"
echo ""
echo "Built executables:"
echo "- dist/gpx_arangodb-datasource_linux_amd64 (Linux x64)"
echo "- dist/gpx_arangodb-datasource_linux_arm64 (Linux ARM64)"
echo ""
echo "Next steps:"
echo "1. Copy the 'dist' folder to your Grafana plugins directory"
echo "   Example: /var/lib/grafana/plugins/arangodb-datasource/"
echo "2. Restart Grafana"
echo "3. Go to Configuration -> Data Sources -> Add data source"
echo "4. Select 'ArangoDB' from the list"
echo ""

# Check if GRAFANA_PLUGINS_DIR is set
if [ ! -z "$GRAFANA_PLUGINS_DIR" ]; then
    echo ""
    read -p "Install plugin to Grafana plugins directory? (y/N): " install_choice
    if [[ "$install_choice" == "y" || "$install_choice" == "Y" ]]; then
        plugin_path="$GRAFANA_PLUGINS_DIR/arangodb-datasource"
        
        if [ -d "$plugin_path" ]; then
            rm -rf "$plugin_path"
        fi
        
        cp -r dist "$plugin_path"
        echo "Plugin installed to: $plugin_path"
        echo "Please restart Grafana to load the plugin."
    fi
else
    echo ""
    echo "Tip: Set GRAFANA_PLUGINS_DIR environment variable to automatically install the plugin"
    echo "Example: export GRAFANA_PLUGINS_DIR='/var/lib/grafana/plugins'"
fi
