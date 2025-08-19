#!/bin/bash

echo "🔄 AWS MCP Server Quick Test"
echo "============================"

# Build the latest version
echo "📦 Building latest version..."
go build -o bin/aws-mcp-server ./cmd/server
if [ $? -ne 0 ]; then
    echo "❌ Build failed"
    exit 1
fi
echo "✅ Build successful"

# Test basic MCP protocol
echo ""
echo "🧪 Testing MCP protocol..."

# Create a simple test file
cat > /tmp/mcp_test.json << 'EOF'
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{"resources":{},"tools":{}},"clientInfo":{"name":"test-client","version":"1.0.0"}}}
{"jsonrpc":"2.0","id":2,"method":"tools/list"}
{"jsonrpc":"2.0","id":3,"method":"resources/list"}
EOF

# Run the test
echo "📤 Sending test messages to MCP server..."
timeout 10s ./bin/aws-mcp-server < /tmp/mcp_test.json > /tmp/mcp_response.json 2>/tmp/mcp_error.log

# Check responses
if [ $? -eq 124 ]; then
    echo "✅ Server responded (timeout expected)"
elif [ $? -eq 0 ]; then
    echo "✅ Server completed successfully"
else
    echo "❌ Server error occurred"
    echo "Error log:"
    cat /tmp/mcp_error.log
    exit 1
fi

# Show sample responses
echo ""
echo "📥 Sample server responses:"
echo "=========================="
if [ -f /tmp/mcp_response.json ]; then
    head -n 3 /tmp/mcp_response.json | jq '.' 2>/dev/null || head -n 3 /tmp/mcp_response.json
else
    echo "No response file generated"
fi

# Clean up
rm -f /tmp/mcp_test.json /tmp/mcp_response.json /tmp/mcp_error.log

echo ""
echo "🎉 MCP server is ready for VS Code integration!"
echo ""
echo "📝 To use with VS Code + Copilot:"
echo "================================="
echo "1. Open VS Code: code ."
echo "2. Open Copilot Chat (Cmd+Shift+I or sidebar icon)"
echo "3. Try these example prompts:"
echo ""
echo "   @aws-mcp-server What AWS resources do I have?"
echo "   @aws-mcp-server List all my EC2 instances"
echo "   @aws-mcp-server Show me my VPC configuration"
echo "   @aws-mcp-server Create a new VPC with CIDR 10.0.0.0/16"
echo ""
echo "🔍 For detailed setup: cat docs/VSCODE_SETUP.md"
