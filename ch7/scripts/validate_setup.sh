#!/bin/bash

echo "🔍 AWS MCP Server Validation Script"
echo "===================================="

# Check if binary exists and is executable
echo "📁 Checking binary..."
if [ -x "./bin/aws-mcp-server" ]; then
    echo "✅ Binary exists and is executable"
else
    echo "❌ Binary not found or not executable"
    echo "Run: go build -o bin/aws-mcp-server ./cmd/server"
    exit 1
fi

# Check AWS credentials
echo ""
echo "🔐 Checking AWS credentials..."
if aws sts get-caller-identity >/dev/null 2>&1; then
    echo "✅ AWS credentials are configured"
    aws sts get-caller-identity --query 'Account' --output text | sed 's/^/   Account: /'
    aws configure get region | sed 's/^/   Region: /' || echo "   Region: Not set (will use us-east-1)"
else
    echo "⚠️  AWS credentials not configured"
    echo "   This is optional for testing MCP protocol"
    echo "   Configure with: aws configure"
fi

# Check VS Code settings
echo ""
echo "⚙️  Checking VS Code configuration..."
if [ -f ".vscode/settings.json" ]; then
    echo "✅ VS Code settings file exists"
    if grep -q "aws-mcp-server" .vscode/settings.json; then
        echo "✅ MCP server configured in VS Code settings"
    else
        echo "⚠️  MCP server not found in VS Code settings"
    fi
else
    echo "⚠️  VS Code settings file not found"
fi

# Test MCP server startup (quick test)
echo ""
echo "🚀 Testing MCP server startup..."
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | timeout 5s ./bin/aws-mcp-server > /dev/null 2>&1
if [ $? -eq 0 ] || [ $? -eq 124 ]; then  # 0 = success, 124 = timeout (expected)
    echo "✅ MCP server starts successfully"
else
    echo "❌ MCP server failed to start"
    echo "Check logs for details"
fi

echo ""
echo "📋 Setup Summary:"
echo "=================="
echo "✅ Binary: Ready"
echo "$(aws sts get-caller-identity >/dev/null 2>&1 && echo "✅" || echo "⚠️ ") AWS Credentials: $(aws sts get-caller-identity >/dev/null 2>&1 && echo "Configured" || echo "Optional")"
echo "$([ -f ".vscode/settings.json" ] && echo "✅" || echo "⚠️ ") VS Code Settings: $([ -f ".vscode/settings.json" ] && echo "Present" || echo "Missing")"

echo ""
echo "🎯 Next Steps:"
echo "============="
echo "1. Open this project in VS Code: code ."
echo "2. Install GitHub Copilot and Copilot Chat extensions"
echo "3. Open Copilot Chat and try: @aws-mcp-server List my EC2 instances"
echo "4. See docs/VSCODE_SETUP.md for detailed instructions"

echo ""
echo "🔗 Useful Commands:"
echo "=================="
echo "• Test AWS CLI: aws sts get-caller-identity"
echo "• Rebuild server: go build -o bin/aws-mcp-server ./cmd/server"
echo "• View setup guide: cat docs/VSCODE_SETUP.md"
