package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"aws-mcp-server/internal/logging"
	"aws-mcp-server/pkg/aws"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolHandler_CallTool(t *testing.T) {
	// Note: This is a unit test that tests the tool handler structure.
	// It doesn't test actual AWS API calls as that would require AWS credentials
	// and could incur costs. For integration testing, you would need to mock
	// the AWS client or use LocalStack.

	// Create a logger
	logger := logging.NewLogger("info", "text")

	// Create AWS client (this would fail without credentials, but we're just testing structure)
	awsClient, err := aws.NewClient("us-west-2", "", logger)
	if err != nil {
		t.Skip("Skipping test due to AWS configuration requirement")
	}

	// Create tool handler
	toolHandler := NewToolHandler(awsClient, logger)

	ctx := context.Background()

	t.Run("unknown tool", func(t *testing.T) {
		result, err := toolHandler.CallTool(ctx, "unknown-tool", map[string]interface{}{})

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "unknown tool")
	})

	t.Run("create-ec2-instance missing imageId", func(t *testing.T) {
		arguments := map[string]interface{}{
			"instanceType": "t2.micro",
		}

		result, err := toolHandler.CallTool(ctx, "create-ec2-instance", arguments)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Content, 1)

		if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
			assert.Contains(t, textContent.Text, "imageId is required")
			assert.Contains(t, textContent.Text, "\"success\": false")
		}
	})

	t.Run("create-ec2-instance missing instanceType", func(t *testing.T) {
		arguments := map[string]interface{}{
			"imageId": "ami-12345678",
		}

		result, err := toolHandler.CallTool(ctx, "create-ec2-instance", arguments)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Content, 1)

		if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
			assert.Contains(t, textContent.Text, "instanceType is required")
			assert.Contains(t, textContent.Text, "\"success\": false")
		}
	})

	t.Run("start-ec2-instance missing instanceId", func(t *testing.T) {
		arguments := map[string]interface{}{}

		result, err := toolHandler.CallTool(ctx, "start-ec2-instance", arguments)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Content, 1)

		if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
			assert.Contains(t, textContent.Text, "instanceId is required")
			assert.Contains(t, textContent.Text, "\"success\": false")
		}
	})

	t.Run("stop-ec2-instance missing instanceId", func(t *testing.T) {
		arguments := map[string]interface{}{}

		result, err := toolHandler.CallTool(ctx, "stop-ec2-instance", arguments)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Content, 1)

		if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
			assert.Contains(t, textContent.Text, "instanceId is required")
			assert.Contains(t, textContent.Text, "\"success\": false")
		}
	})

	t.Run("terminate-ec2-instance missing instanceId", func(t *testing.T) {
		arguments := map[string]interface{}{}

		result, err := toolHandler.CallTool(ctx, "terminate-ec2-instance", arguments)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Content, 1)

		if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
			assert.Contains(t, textContent.Text, "instanceId is required")
			assert.Contains(t, textContent.Text, "\"success\": false")
		}
	})

	t.Run("valid arguments should pass validation", func(t *testing.T) {
		testCases := []struct {
			name      string
			arguments map[string]interface{}
		}{
			{
				name: "create-ec2-instance",
				arguments: map[string]interface{}{
					"instanceType": "t3.micro",
					"imageId":      "ami-12345678",
					"keyName":      "my-key",
				},
			},
			{
				name: "start-ec2-instance",
				arguments: map[string]interface{}{
					"instanceId": "i-12345678",
				},
			},
			{
				name: "stop-ec2-instance",
				arguments: map[string]interface{}{
					"instanceId": "i-12345678",
				},
			},
			{
				name: "terminate-ec2-instance",
				arguments: map[string]interface{}{
					"instanceId": "i-12345678",
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// This will fail at AWS API level, but argument validation should pass
				result, err := toolHandler.CallTool(ctx, tc.name, tc.arguments)

				// We expect success response (tool validation passed) but with AWS error content
				require.NoError(t, err)
				require.NotNil(t, result)
				require.NotEmpty(t, result.Content)

				// Get the text content from the first content item
				textContent, ok := result.Content[0].(*mcp.TextContent)
				require.True(t, ok)

				// Parse the JSON response
				var response map[string]interface{}
				err = json.Unmarshal([]byte(textContent.Text), &response)
				require.NoError(t, err)

				// Should not contain argument validation errors
				responseStr := fmt.Sprintf("%v", response)
				assert.NotContains(t, responseStr, "is required")
				assert.NotContains(t, responseStr, "unknown tool")
			})
		}
	})
}

func TestNewToolHandler(t *testing.T) {
	logger := logging.NewLogger("info", "text")
	awsClient, err := aws.NewClient("us-west-2", "", logger)
	if err != nil {
		t.Skip("Skipping test due to AWS configuration requirement")
	}

	toolHandler := NewToolHandler(awsClient, logger)

	require.NotNil(t, toolHandler)
	assert.NotNil(t, toolHandler.awsClient)
	assert.NotNil(t, toolHandler.logger)
}
