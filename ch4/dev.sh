#!/bin/bash
set -e

echo "Starting development environment..."

# Start localstack for AWS service mocking (optional)
if command -v docker >/dev/null 2>&1; then
    docker run -d --name localstack -p 4566:4566 localstack/localstack
    echo "✓ LocalStack started for AWS mocking"
fi

# Start the MCP server with live reload
echo "Starting MCP server with live reload..."
air -c .air.toml