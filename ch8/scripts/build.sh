#!/bin/bash
set -e

echo "Building AWS MCP Server..."

# Clean previous builds
rm -f bin/aws-mcp-server

# Create bin directory
mkdir -p bin

# Build the server
go build -o bin/aws-mcp-server ./cmd/server

echo "✓ Build completed: bin/aws-mcp-server"