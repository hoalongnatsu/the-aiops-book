package mcp

import (
	"aws-mcp-server/pkg/aws"
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// ========== Interface defines ==========

// ASGToolsInterface defines all available Auto Scaling Group tools
// Following Single Responsibility Principle - each tool manages one specific ASG resource type
//
// Available Tools:
//   - listAutoScalingGroups()     : List all Auto Scaling Groups in the region
//   - listLaunchTemplates()       : List all Launch Templates in the region
//   - createLaunchTemplate()      : Create a Launch Template (aws_launch_template equivalent)
//   - createAutoScalingGroup()    : Create an Auto Scaling Group (aws_autoscaling_group equivalent)
//
// Usage Example (Terraform-like workflow):
//   1. createLaunchTemplate(name="my-template", imageId="ami-xxx", instanceType="t3.micro")
//   2. createAutoScalingGroup(name="my-asg", launchTemplateName="my-template", subnetIds=["subnet-xxx"])

// ========== ASG Tools ==========

func (h *ToolHandler) listAutoScalingGroups(ctx context.Context) (*mcp.CallToolResult, error) {
	result, err := h.resourceHandler.ReadResource(ctx, "aws://autoscaling/groups")
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to list Auto Scaling Groups: %s", err.Error()))
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

func (h *ToolHandler) listLaunchTemplates(ctx context.Context) (*mcp.CallToolResult, error) {
	result, err := h.resourceHandler.ReadResource(ctx, "aws://ec2/launchtemplates")
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to list Launch Templates: %s", err.Error()))
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

// Auto Scaling Methods
func (h *ToolHandler) createLaunchTemplate(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	launchTemplateName, ok := arguments["launchTemplateName"].(string)
	if !ok || launchTemplateName == "" {
		return h.createErrorResponse("launchTemplateName is required")
	}

	imageID, ok := arguments["imageId"].(string)
	if !ok || imageID == "" {
		return h.createErrorResponse("imageId is required")
	}

	instanceType, ok := arguments["instanceType"].(string)
	if !ok || instanceType == "" {
		return h.createErrorResponse("instanceType is required")
	}

	keyName, _ := arguments["keyName"].(string)
	userData, _ := arguments["userData"].(string)
	iamInstanceProfile, _ := arguments["iamInstanceProfile"].(string)

	var securityGroupIDs []string
	if sgs, exists := arguments["securityGroupIds"].([]interface{}); exists {
		for _, sg := range sgs {
			if sgStr, ok := sg.(string); ok {
				securityGroupIDs = append(securityGroupIDs, sgStr)
			}
		}
	}

	params := aws.CreateLaunchTemplateParams{
		LaunchTemplateName: launchTemplateName,
		ImageID:            imageID,
		InstanceType:       instanceType,
		KeyName:            keyName,
		SecurityGroupIDs:   securityGroupIDs,
		UserData:           userData,
		IamInstanceProfile: iamInstanceProfile,
		Tags:               make(map[string]string),
	}

	// Add optional tags
	if tags, exists := arguments["tags"].(map[string]interface{}); exists {
		for k, v := range tags {
			if strVal, ok := v.(string); ok {
				params.Tags[k] = strVal
			}
		}
	}

	resource, err := h.awsClient.CreateLaunchTemplate(ctx, params)
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to create launch template: %s", err.Error()))
	}

	return h.createSuccessResponse("Launch Template created successfully", map[string]interface{}{
		"launchTemplateId":   resource.ID,
		"launchTemplateName": launchTemplateName,
		"imageId":            imageID,
		"instanceType":       instanceType,
		"tags":               resource.Tags,
	})
}

func (h *ToolHandler) createAutoScalingGroup(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	asgName, ok := arguments["autoScalingGroupName"].(string)
	if !ok || asgName == "" {
		return h.createErrorResponse("autoScalingGroupName is required")
	}

	launchTemplateName, ok := arguments["launchTemplateName"].(string)
	if !ok || launchTemplateName == "" {
		return h.createErrorResponse("launchTemplateName is required")
	}

	minSize := int32(1)
	if val, exists := arguments["minSize"]; exists {
		if f, ok := val.(float64); ok {
			minSize = int32(f)
		}
	}

	maxSize := int32(3)
	if val, exists := arguments["maxSize"]; exists {
		if f, ok := val.(float64); ok {
			maxSize = int32(f)
		}
	}

	desiredCapacity := int32(2)
	if val, exists := arguments["desiredCapacity"]; exists {
		if f, ok := val.(float64); ok {
			desiredCapacity = int32(f)
		}
	}

	var vpcZoneIdentifiers []string
	if subnets, exists := arguments["vpcZoneIdentifiers"].([]interface{}); exists {
		for _, subnet := range subnets {
			if subnetStr, ok := subnet.(string); ok {
				vpcZoneIdentifiers = append(vpcZoneIdentifiers, subnetStr)
			}
		}
	}

	var targetGroupARNs []string
	if tgs, exists := arguments["targetGroupArns"].([]interface{}); exists {
		for _, tg := range tgs {
			if tgStr, ok := tg.(string); ok {
				targetGroupARNs = append(targetGroupARNs, tgStr)
			}
		}
	}

	healthCheckType := "EC2"
	if val, exists := arguments["healthCheckType"]; exists {
		healthCheckType, _ = val.(string)
	}

	healthCheckGracePeriod := int32(300)
	if val, exists := arguments["healthCheckGracePeriod"]; exists {
		if f, ok := val.(float64); ok {
			healthCheckGracePeriod = int32(f)
		}
	}

	params := aws.CreateAutoScalingGroupParams{
		AutoScalingGroupName:   asgName,
		LaunchTemplateName:     launchTemplateName,
		MinSize:                minSize,
		MaxSize:                maxSize,
		DesiredCapacity:        desiredCapacity,
		VPCZoneIdentifiers:     vpcZoneIdentifiers,
		TargetGroupARNs:        targetGroupARNs,
		HealthCheckType:        healthCheckType,
		HealthCheckGracePeriod: healthCheckGracePeriod,
		Tags:                   make(map[string]string),
	}

	// Add optional tags
	if tags, exists := arguments["tags"].(map[string]interface{}); exists {
		for k, v := range tags {
			if strVal, ok := v.(string); ok {
				params.Tags[k] = strVal
			}
		}
	}

	resource, err := h.awsClient.CreateAutoScalingGroup(ctx, params)
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to create auto scaling group: %s", err.Error()))
	}

	return h.createSuccessResponse("Auto Scaling Group created successfully", map[string]interface{}{
		"autoScalingGroupArn":  resource.ID,
		"autoScalingGroupName": asgName,
		"minSize":              minSize,
		"maxSize":              maxSize,
		"desiredCapacity":      desiredCapacity,
		"tags":                 resource.Tags,
	})
}
