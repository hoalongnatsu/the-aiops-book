package mcp

import (
	"aws-mcp-server/pkg/aws"
	"context"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// ========== Interface defines ==========

// EC2ToolsInterface defines all available EC2 compute tools
// Following Single Responsibility Principle - each tool manages one specific EC2 resource type
//
// Available Tools:
//   - listEC2Instances()          : List all EC2 instances in the region
//   - listAMIs()                  : List all AMIs owned by the account
//   - createEC2Instance()         : Create a new EC2 instance (aws_instance equivalent)
//   - startEC2Instance()          : Start a stopped EC2 instance
//   - stopEC2Instance()           : Stop a running EC2 instance
//   - terminateEC2Instance()      : Terminate an EC2 instance (permanent deletion)
//   - createAMIFromInstance()     : Create an AMI from an existing EC2 instance
//
// Usage Example (Terraform-like workflow):
//   1. createEC2Instance(imageId="ami-xxx", instanceType="t3.micro", keyName="my-key")
//   2. createAMIFromInstance(instanceId="i-xxx", name="my-custom-ami")
//   3. stopEC2Instance(instanceId="i-xxx")

// ========== EC2 Tools ==========

func (h *ToolHandler) listEC2Instances(ctx context.Context) (*mcp.CallToolResult, error) {
	result, err := h.resourceHandler.ReadResource(ctx, "aws://ec2/instances")
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to list EC2 instances: %s", err.Error()))
	}

	// Extract the text content from the resource result
	if len(result.Contents) > 0 {
		if textContent, ok := result.Contents[0].(*mcp.TextResourceContents); ok {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Type: "text",
						Text: textContent.Text,
					},
				},
			}, nil
		}
	}

	return h.createErrorResponse("No data returned from resource")
}

func (h *ToolHandler) listAMIs(ctx context.Context) (*mcp.CallToolResult, error) {
	result, err := h.resourceHandler.ReadResource(ctx, "aws://ec2/images")
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to list AMIs: %s", err.Error()))
	}

	if len(result.Contents) > 0 {
		if textContent, ok := result.Contents[0].(*mcp.TextResourceContents); ok {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Type: "text",
						Text: textContent.Text,
					},
				},
			}, nil
		}
	}

	return h.createErrorResponse("No data returned from resource")
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

// createAMIFromInstance creates an AMI from an existing EC2 instance
func (h *ToolHandler) createAMIFromInstance(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	instanceID, ok := arguments["instanceId"].(string)
	if !ok || instanceID == "" {
		return h.createErrorResponse("instanceId is required")
	}

	amiName, ok := arguments["amiName"].(string)
	if !ok || amiName == "" {
		// Generate default name
		amiName = fmt.Sprintf("ami-%s-%d", instanceID, time.Now().Unix())
	}

	description := "AMI created for production deployment"
	if val, exists := arguments["description"]; exists {
		description, _ = val.(string)
	}

	resource, err := h.awsClient.CreateAMI(ctx, instanceID, amiName, description)
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to create AMI: %s", err.Error()))
	}

	return h.createSuccessResponse("AMI creation initiated", map[string]interface{}{
		"amiId":            resource.ID,
		"amiName":          amiName,
		"description":      description,
		"sourceInstanceId": instanceID,
		"state":            resource.State,
		"message":          "AMI creation started. This process may take several minutes.",
	})
}
