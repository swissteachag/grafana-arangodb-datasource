#!/usr/bin/env pwsh

# Grafana ArangoDB Datasource Setup Script for Windows
Write-Host "Setting up Grafana ArangoDB Datasource..." -ForegroundColor Green

# Check if Node.js is installed
try {
    $nodeVersion = node --version
    Write-Host "Node.js version: $nodeVersion" -ForegroundColor Green
} catch {
    Write-Host "Error: Node.js is not installed or not in PATH" -ForegroundColor Red
    Write-Host "Please install Node.js from https://nodejs.org/" -ForegroundColor Yellow
    exit 1
}

# Check if Go is installed
try {
    $goVersion = go version
    Write-Host "Go version: $goVersion" -ForegroundColor Green
} catch {
    Write-Host "Error: Go is not installed or not in PATH" -ForegroundColor Red
    Write-Host "Please install Go from https://golang.org/dl/" -ForegroundColor Yellow
    exit 1
}

# Install Node.js dependencies
Write-Host "Installing Node.js dependencies..." -ForegroundColor Blue
npm install

if ($LASTEXITCODE -ne 0) {
    Write-Host "Failed to install Node.js dependencies" -ForegroundColor Red
    exit 1
}

# Initialize Go module and download dependencies
Write-Host "Setting up Go dependencies..." -ForegroundColor Blue
go mod tidy

if ($LASTEXITCODE -ne 0) {
    Write-Host "Failed to set up Go dependencies" -ForegroundColor Red
    exit 1
}

# Build the plugin
Write-Host "Building the plugin..." -ForegroundColor Blue

# Build frontend
Write-Host "Building frontend..." -ForegroundColor Cyan
npm run build

if ($LASTEXITCODE -ne 0) {
    Write-Host "Failed to build frontend" -ForegroundColor Red
    exit 1
}

# Build backend
Write-Host "Building backend for multiple platforms..." -ForegroundColor Cyan

# Ask user which platforms to build for
Write-Host ""
Write-Host "Select target platforms:" -ForegroundColor Yellow
Write-Host "1. Linux x64 only"
Write-Host "2. Linux ARM64 only"
Write-Host "3. Both Linux x64 and ARM64 (default)"
Write-Host "4. Windows x64 (for development)"
Write-Host "5. All platforms"
$platformChoice = Read-Host "Enter choice (1-5, default: 3)"

if ([string]::IsNullOrEmpty($platformChoice)) {
    $platformChoice = "3"
}

if (!(Test-Path "dist")) {
    New-Item -ItemType Directory -Path "dist"
}

Set-Location pkg

$buildSuccess = $true

switch ($platformChoice) {
    "1" {
        Write-Host "Building for Linux x64..." -ForegroundColor Yellow
        $env:GOOS = "linux"
        $env:GOARCH = "amd64"
        go build -o "../dist/gpx_arangodb-datasource_linux_amd64" .
        if ($LASTEXITCODE -ne 0) { $buildSuccess = $false }
    }
    "2" {
        Write-Host "Building for Linux ARM64..." -ForegroundColor Yellow
        $env:GOOS = "linux"
        $env:GOARCH = "arm64"
        go build -o "../dist/gpx_arangodb-datasource_linux_arm64" .
        if ($LASTEXITCODE -ne 0) { $buildSuccess = $false }
    }
    "3" {
        Write-Host "Building for Linux x64..." -ForegroundColor Yellow
        $env:GOOS = "linux"
        $env:GOARCH = "amd64"
        go build -o "../dist/gpx_arangodb-datasource_linux_amd64" .
        if ($LASTEXITCODE -ne 0) { $buildSuccess = $false }
        
        if ($buildSuccess) {
            Write-Host "Building for Linux ARM64..." -ForegroundColor Yellow
            $env:GOOS = "linux"
            $env:GOARCH = "arm64"
            go build -o "../dist/gpx_arangodb-datasource_linux_arm64" .
            if ($LASTEXITCODE -ne 0) { $buildSuccess = $false }
        }
    }
    "4" {
        Write-Host "Building for Windows x64..." -ForegroundColor Yellow
        $env:GOOS = "windows"
        $env:GOARCH = "amd64"
        go build -o "../dist/gpx_arangodb-datasource_windows_amd64.exe" .
        if ($LASTEXITCODE -ne 0) { $buildSuccess = $false }
    }
    "5" {
        Write-Host "Building for Linux x64..." -ForegroundColor Yellow
        $env:GOOS = "linux"
        $env:GOARCH = "amd64"
        go build -o "../dist/gpx_arangodb-datasource_linux_amd64" .
        if ($LASTEXITCODE -ne 0) { $buildSuccess = $false }
        
        if ($buildSuccess) {
            Write-Host "Building for Linux ARM64..." -ForegroundColor Yellow
            $env:GOOS = "linux"
            $env:GOARCH = "arm64"
            go build -o "../dist/gpx_arangodb-datasource_linux_arm64" .
            if ($LASTEXITCODE -ne 0) { $buildSuccess = $false }
        }
        
        if ($buildSuccess) {
            Write-Host "Building for Windows x64..." -ForegroundColor Yellow
            $env:GOOS = "windows"
            $env:GOARCH = "amd64"
            go build -o "../dist/gpx_arangodb-datasource_windows_amd64.exe" .
            if ($LASTEXITCODE -ne 0) { $buildSuccess = $false }
        }
    }
    default {
        Write-Host "Invalid choice, building for both Linux platforms..." -ForegroundColor Yellow
        $env:GOOS = "linux"
        $env:GOARCH = "amd64"
        go build -o "../dist/gpx_arangodb-datasource_linux_amd64" .
        if ($LASTEXITCODE -ne 0) { $buildSuccess = $false }
        
        if ($buildSuccess) {
            $env:GOOS = "linux"
            $env:GOARCH = "arm64"
            go build -o "../dist/gpx_arangodb-datasource_linux_arm64" .
            if ($LASTEXITCODE -ne 0) { $buildSuccess = $false }
        }
    }
}

# Reset environment variables
Remove-Item Env:GOOS -ErrorAction SilentlyContinue
Remove-Item Env:GOARCH -ErrorAction SilentlyContinue

Set-Location ..

if (-not $buildSuccess) {
    Write-Host "Failed to build backend" -ForegroundColor Red
    exit 1
}

# Copy plugin.json and troubleshooting guide to dist
Copy-Item "plugin.json" "dist/"

Write-Host "Plugin built successfully!" -ForegroundColor Green
Write-Host ""
Write-Host "Built executables:" -ForegroundColor Yellow
Get-ChildItem "dist/gpx_arangodb-datasource_*" | ForEach-Object {
    Write-Host "- $($_.Name)" -ForegroundColor White
}
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Yellow
Write-Host "1. Copy the 'dist' folder to your Grafana plugins directory" -ForegroundColor White
Write-Host "   Linux example: /var/lib/grafana/plugins/arangodb-datasource/" -ForegroundColor Gray
Write-Host "   Windows example: C:\Program Files\GrafanaLabs\grafana\data\plugins\arangodb-datasource\" -ForegroundColor Gray
Write-Host "2. Restart Grafana" -ForegroundColor White
Write-Host "3. Go to Configuration -> Data Sources -> Add data source" -ForegroundColor White
Write-Host "4. Select 'ArangoDB' from the list" -ForegroundColor White
Write-Host ""
Write-Host "For development, you can run: npm run dev" -ForegroundColor Cyan
