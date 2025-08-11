package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestMCPServer(t *testing.T, mcpServer *server.MCPServer) *MCPTestClient {
	return &MCPTestClient{
		t:      t,
		server: mcpServer,
	}
}

type MCPTestClient struct {
	t      *testing.T
	server *server.MCPServer
}

func (c *MCPTestClient) CallTool(name string, arguments map[string]interface{}) *mcp.CallToolResult {
	result, err := c.CallToolWithError(name, arguments)
	if err != nil {
		c.t.Fatalf("Tool call failed: %v", err)
	}
	return result
}

// CallToolWithError calls a tool and returns both the result and any error
func (c *MCPTestClient) CallToolWithError(name string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// Create the proper request structure
	params := mcp.CallToolParams{
		Name:      name,
		Arguments: arguments,
	}

	// Create a JSON-RPC request
	request := mcp.JSONRPCRequest{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      mcp.NewRequestId(1),
		Params:  params,
	}
	request.Method = "tools/call"

	// Convert to JSON and back to simulate network transmission
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	// Handle the message through the server
	response := c.server.HandleMessage(context.Background(), requestBytes)

	// Check for error in response
	if jsonResponse, ok := response.(mcp.JSONRPCResponse); ok {
		if jsonResponse.Result != nil {
			// Parse the result as raw JSON first
			resultBytes, err := json.Marshal(jsonResponse.Result)
			if err != nil {
				return nil, err
			}

			// Parse using the provided parser function
			resultRaw := json.RawMessage(resultBytes)
			result, err := mcp.ParseCallToolResult(&resultRaw)
			if err != nil {
				return nil, err
			}

			return result, nil
		}
		return &mcp.CallToolResult{}, nil
	} else if jsonError, ok := response.(mcp.JSONRPCError); ok {
		return nil, fmt.Errorf("%v", jsonError.Error.Message)
	}

	return nil, fmt.Errorf("unexpected response type")
}

func (c *MCPTestClient) ReadResource(uri string) *mcp.ReadResourceResult {
	result, err := c.ReadResourceWithError(uri)
	if err != nil {
		c.t.Fatalf("Resource read failed: %v", err)
	}
	return result
}

// ReadResourceWithError reads a resource and returns both the result and any error
func (c *MCPTestClient) ReadResourceWithError(uri string) (*mcp.ReadResourceResult, error) {
	// Create the proper request structure
	params := mcp.ReadResourceParams{
		URI: uri,
	}

	// Create a JSON-RPC request
	request := mcp.JSONRPCRequest{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      mcp.NewRequestId(2),
		Params:  params,
	}
	request.Method = "resources/read"

	// Convert to JSON and back to simulate network transmission
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	// Handle the message through the server
	response := c.server.HandleMessage(context.Background(), requestBytes)

	// Check for error in response
	if jsonResponse, ok := response.(mcp.JSONRPCResponse); ok {
		if jsonResponse.Result != nil {
			// Parse the result as raw JSON first
			resultBytes, err := json.Marshal(jsonResponse.Result)
			if err != nil {
				return nil, err
			}

			// Parse using the provided parser function
			resultRaw := json.RawMessage(resultBytes)
			result, err := mcp.ParseReadResourceResult(&resultRaw)
			if err != nil {
				return nil, err
			}

			return result, nil
		}
		return &mcp.ReadResourceResult{}, nil
	} else if jsonError, ok := response.(mcp.JSONRPCError); ok {
		return nil, fmt.Errorf("%v", jsonError.Error.Message)
	}

	return nil, fmt.Errorf("unexpected response type")
}
