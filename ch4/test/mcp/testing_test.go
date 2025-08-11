package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPTestClient(t *testing.T) {
	// Create a test MCP server with tool and resource capabilities
	mcpServer := server.NewMCPServer("test-server", "1.0.0",
		server.WithToolCapabilities(false),
		server.WithResourceCapabilities(false, false),
	)

	// Add a simple test tool
	mcpServer.AddTool(
		mcp.NewTool("echo",
			mcp.WithDescription("Echo back the input"),
			mcp.WithString("message", mcp.Description("Message to echo back")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			message := request.GetString("message", "")
			return mcp.NewToolResultText("Echo: " + message), nil
		},
	)

	// Add a simple test resource
	mcpServer.AddResource(
		mcp.NewResource("test://example", "Test Resource",
			mcp.WithResourceDescription("A simple test resource"),
			mcp.WithMIMEType("text/plain"),
		),
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return []mcp.ResourceContents{
				&mcp.TextResourceContents{
					URI:      "test://example",
					Text:     "Hello, World!",
					MIMEType: "text/plain",
				},
			}, nil
		},
	)

	// Create test client
	client := TestMCPServer(t, mcpServer)

	t.Run("CallTool", func(t *testing.T) {
		// Test tool call
		result := client.CallTool("echo", map[string]interface{}{
			"message": "Hello, MCP!",
		})

		require.NotNil(t, result)
		assert.Len(t, result.Content, 1)

		if len(result.Content) > 0 {
			if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
				assert.Equal(t, "Echo: Hello, MCP!", textContent.Text)
			} else {
				t.Error("Expected text content in tool result")
			}
		}
	})

	t.Run("ReadResource", func(t *testing.T) {
		// Test resource read
		result := client.ReadResource("test://example")

		require.NotNil(t, result)
		assert.Len(t, result.Contents, 1)

		if len(result.Contents) > 0 {
			if textContent, ok := mcp.AsTextResourceContents(result.Contents[0]); ok {
				assert.Equal(t, "Hello, World!", textContent.Text)
				assert.Equal(t, "test://example", textContent.URI)
			} else {
				t.Error("Expected text resource content")
			}
		}
	})
}

func TestMCPTestClientErrorHandling(t *testing.T) {
	// Create a test MCP server without any tools but with tool capabilities enabled
	mcpServer := server.NewMCPServer("test-server", "1.0.0",
		server.WithToolCapabilities(false),
	)

	// Create test client
	client := TestMCPServer(t, mcpServer)

	// Test that calling a non-existent tool produces the expected error
	t.Run("NonExistentToolError", func(t *testing.T) {
		// Use the error-returning method to test error handling
		result, err := client.CallToolWithError("non-existent-tool", map[string]interface{}{})

		// We expect an error
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not found")

		t.Logf("Got expected error: %v", err)
	})
}
