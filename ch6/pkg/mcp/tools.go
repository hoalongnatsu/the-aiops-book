package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"aws-mcp-server/internal/logging"
	"aws-mcp-server/pkg/aws"

	"github.com/mark3labs/mcp-go/mcp"
)

type ToolHandler struct {
	awsClient *aws.Client
	logger    *logging.Logger
}

func NewToolHandler(awsClient *aws.Client, logger *logging.Logger) *ToolHandler {
	return &ToolHandler{
		awsClient: awsClient,
		logger:    logger,
	}
}

// CallTool handles requests for specific tools
func (h *ToolHandler) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	h.logger.LogMCPCallTool(name, arguments)

	switch name {
	case "create-ec2-instance":
		return h.createEC2Instance(ctx, arguments)
	case "start-ec2-instance":
		return h.startEC2Instance(ctx, arguments)
	case "stop-ec2-instance":
		return h.stopEC2Instance(ctx, arguments)
	case "terminate-ec2-instance":
		return h.terminateEC2Instance(ctx, arguments)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// createEC2Instance creates a new EC2 instance
// NOTE: In production, parameter validation should be moved to a separate validation function
// for better code organization and reusability. For this chapter, we keep the validation
// inline to make the code easier to understand and follow.
func (h *ToolHandler) createEC2Instance(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// Extract required parameters
	imageID, ok := arguments["imageId"].(string)
	if !ok || imageID == "" {
		return h.createErrorResponse("imageId is required")
	}

	instanceType, ok := arguments["instanceType"].(string)
	if !ok || instanceType == "" {
		return h.createErrorResponse("instanceType is required")
	}

	// Extract optional parameters
	var keyName, securityGroupID, subnetID, name string
	if val, exists := arguments["keyName"]; exists {
		keyName, _ = val.(string)
	}
	if val, exists := arguments["securityGroupId"]; exists {
		securityGroupID, _ = val.(string)
	}
	if val, exists := arguments["subnetId"]; exists {
		subnetID, _ = val.(string)
	}
	if val, exists := arguments["name"]; exists {
		name, _ = val.(string)
	}

	params := aws.CreateInstanceParams{
		ImageID:         imageID,
		InstanceType:    instanceType,
		KeyName:         keyName,
		SecurityGroupID: securityGroupID,
		SubnetID:        subnetID,
		Name:            name,
	}

	resource, err := h.awsClient.CreateEC2Instance(ctx, params)
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("failed to create EC2 instance: %v", err))
	}

	data := map[string]interface{}{
		"instanceId":   resource.ID,
		"state":        resource.State,
		"instanceType": resource.Details["instanceType"],
	}

	return h.createSuccessResponse("EC2 instance created successfully", data)
}

// startEC2Instance starts a stopped EC2 instance
func (h *ToolHandler) startEC2Instance(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	instanceID, ok := arguments["instanceId"].(string)
	if !ok || instanceID == "" {
		return h.createErrorResponse("instanceId is required")
	}

	err := h.awsClient.StartEC2Instance(ctx, instanceID)
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("failed to start EC2 instance: %v", err))
	}

	data := map[string]interface{}{
		"instanceId": instanceID,
		"action":     "start",
	}

	return h.createSuccessResponse("EC2 instance start initiated successfully", data)
}

// stopEC2Instance stops a running EC2 instance
func (h *ToolHandler) stopEC2Instance(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	instanceID, ok := arguments["instanceId"].(string)
	if !ok || instanceID == "" {
		return h.createErrorResponse("instanceId is required")
	}

	err := h.awsClient.StopEC2Instance(ctx, instanceID)
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("failed to stop EC2 instance: %v", err))
	}

	data := map[string]interface{}{
		"instanceId": instanceID,
		"action":     "stop",
	}

	return h.createSuccessResponse("EC2 instance stop initiated successfully", data)
}

// terminateEC2Instance terminates an EC2 instance
func (h *ToolHandler) terminateEC2Instance(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	instanceID, ok := arguments["instanceId"].(string)
	if !ok || instanceID == "" {
		return h.createErrorResponse("instanceId is required")
	}

	err := h.awsClient.TerminateEC2Instance(ctx, instanceID)
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("failed to terminate EC2 instance: %v", err))
	}

	data := map[string]interface{}{
		"instanceId": instanceID,
		"action":     "terminate",
	}

	return h.createSuccessResponse("EC2 instance termination initiated successfully", data)
}

// createErrorResponse creates a standardized error response for tool actions
func (h *ToolHandler) createErrorResponse(message string) (*mcp.CallToolResult, error) {
	errorData := map[string]interface{}{
		"success":   false,
		"error":     message,
		"timestamp": time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}

	jsonData, _ := json.MarshalIndent(errorData, "", "  ")

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Type: "text",
				Text: string(jsonData),
			},
		},
	}, nil
}

// createSuccessResponse creates a standardized success response for tool actions
func (h *ToolHandler) createSuccessResponse(message string, data map[string]interface{}) (*mcp.CallToolResult, error) {
	responseData := map[string]interface{}{
		"success":   true,
		"message":   message,
		"timestamp": time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}

	// Add any additional data
	for key, value := range data {
		responseData[key] = value
	}

	jsonData, _ := json.MarshalIndent(responseData, "", "  ")

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Type: "text",
				Text: string(jsonData),
			},
		},
	}, nil
}
