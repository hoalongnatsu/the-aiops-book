package mcp

import (
	"aws-mcp-server/pkg/aws"
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// ========== Interface defines ==========

// ALBToolsInterface defines all available Application Load Balancer tools
// Following Single Responsibility Principle - each tool manages one specific ALB resource type
//
// Available Tools:
//   - listLoadBalancers()         : List all Application Load Balancers in the region
//   - listTargetGroups()          : List all Target Groups in the region
//   - createApplicationLoadBalancer() : Create an Application Load Balancer (aws_lb equivalent)
//   - createTargetGroup()         : Create a Target Group (aws_lb_target_group equivalent)
//   - createListener()            : Create a Listener for the load balancer (aws_lb_listener equivalent)
//
// Usage Example (Terraform-like workflow):
//   1. createApplicationLoadBalancer(name="my-alb", subnetIds=["subnet-xxx", "subnet-yyy"])
//   2. createTargetGroup(name="my-targets", vpcId="vpc-xxx", port=80)
//   3. createListener(loadBalancerArn="arn:aws:...", targetGroupArn="arn:aws:...")

// ========== ALB Tools ==========

func (h *ToolHandler) listLoadBalancers(ctx context.Context) (*mcp.CallToolResult, error) {
	result, err := h.resourceHandler.ReadResource(ctx, "aws://elbv2/loadbalancers")
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to list Load Balancers: %s", err.Error()))
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

func (h *ToolHandler) listTargetGroups(ctx context.Context) (*mcp.CallToolResult, error) {
	result, err := h.resourceHandler.ReadResource(ctx, "aws://elbv2/targetgroups")
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to list Target Groups: %s", err.Error()))
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

// Load Balancer Methods
func (h *ToolHandler) createApplicationLoadBalancer(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	name, ok := arguments["name"].(string)
	if !ok || name == "" {
		return h.createErrorResponse("name is required")
	}

	scheme := "internet-facing"
	if val, exists := arguments["scheme"]; exists {
		scheme, _ = val.(string)
	}

	loadBalancerType := "application"
	if val, exists := arguments["type"]; exists {
		loadBalancerType, _ = val.(string)
	}

	ipAddressType := "ipv4"
	if val, exists := arguments["ipAddressType"]; exists {
		ipAddressType, _ = val.(string)
	}

	var subnets []string
	if subnetList, exists := arguments["subnets"].([]interface{}); exists {
		for _, subnet := range subnetList {
			if subnetStr, ok := subnet.(string); ok {
				subnets = append(subnets, subnetStr)
			}
		}
	}

	var securityGroups []string
	if sgList, exists := arguments["securityGroups"].([]interface{}); exists {
		for _, sg := range sgList {
			if sgStr, ok := sg.(string); ok {
				securityGroups = append(securityGroups, sgStr)
			}
		}
	}

	params := aws.CreateLoadBalancerParams{
		Name:           name,
		Scheme:         scheme,
		Type:           loadBalancerType,
		IpAddressType:  ipAddressType,
		Subnets:        subnets,
		SecurityGroups: securityGroups,
		Tags:           make(map[string]string),
	}

	// Add optional tags
	if tags, exists := arguments["tags"].(map[string]interface{}); exists {
		for k, v := range tags {
			if strVal, ok := v.(string); ok {
				params.Tags[k] = strVal
			}
		}
	}

	resource, err := h.awsClient.CreateApplicationLoadBalancer(ctx, params)
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to create load balancer: %s", err.Error()))
	}

	return h.createSuccessResponse("Application Load Balancer created successfully", map[string]interface{}{
		"loadBalancerArn":  resource.ID,
		"loadBalancerName": name,
		"scheme":           scheme,
		"type":             loadBalancerType,
		"tags":             resource.Tags,
	})
}

func (h *ToolHandler) createTargetGroup(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	name, ok := arguments["name"].(string)
	if !ok || name == "" {
		return h.createErrorResponse("name is required")
	}

	protocol := "HTTP"
	if val, exists := arguments["protocol"]; exists {
		protocol, _ = val.(string)
	}

	port := int32(80)
	if val, exists := arguments["port"]; exists {
		if f, ok := val.(float64); ok {
			port = int32(f)
		}
	}

	vpcID, ok := arguments["vpcId"].(string)
	if !ok || vpcID == "" {
		return h.createErrorResponse("vpcId is required")
	}

	targetType := "instance"
	if val, exists := arguments["targetType"]; exists {
		targetType, _ = val.(string)
	}

	healthCheckEnabled := true
	if val, exists := arguments["healthCheckEnabled"]; exists {
		healthCheckEnabled, _ = val.(bool)
	}

	healthCheckPath := "/"
	if val, exists := arguments["healthCheckPath"]; exists {
		healthCheckPath, _ = val.(string)
	}

	params := aws.CreateTargetGroupParams{
		Name:                       name,
		Protocol:                   protocol,
		Port:                       port,
		VpcID:                      vpcID,
		TargetType:                 targetType,
		HealthCheckEnabled:         healthCheckEnabled,
		HealthCheckPath:            healthCheckPath,
		HealthCheckProtocol:        protocol,
		HealthCheckIntervalSeconds: 30,
		HealthCheckTimeoutSeconds:  5,
		HealthyThresholdCount:      2,
		UnhealthyThresholdCount:    2,
		Matcher:                    "200",
		Tags:                       make(map[string]string),
	}

	// Add optional tags
	if tags, exists := arguments["tags"].(map[string]interface{}); exists {
		for k, v := range tags {
			if strVal, ok := v.(string); ok {
				params.Tags[k] = strVal
			}
		}
	}

	resource, err := h.awsClient.CreateTargetGroup(ctx, params)
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to create target group: %s", err.Error()))
	}

	return h.createSuccessResponse("Target Group created successfully", map[string]interface{}{
		"targetGroupArn":  resource.ID,
		"targetGroupName": name,
		"protocol":        protocol,
		"port":            port,
		"vpcId":           vpcID,
		"tags":            resource.Tags,
	})
}

func (h *ToolHandler) createListener(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	loadBalancerArn, ok := arguments["loadBalancerArn"].(string)
	if !ok || loadBalancerArn == "" {
		return h.createErrorResponse("loadBalancerArn is required")
	}

	protocol := "HTTP"
	if val, exists := arguments["protocol"]; exists {
		protocol, _ = val.(string)
	}

	port := int32(80)
	if val, exists := arguments["port"]; exists {
		if f, ok := val.(float64); ok {
			port = int32(f)
		}
	}

	defaultTargetGroupArn, ok := arguments["defaultTargetGroupArn"].(string)
	if !ok || defaultTargetGroupArn == "" {
		return h.createErrorResponse("defaultTargetGroupArn is required")
	}

	certificateArn, _ := arguments["certificateArn"].(string)

	params := aws.CreateListenerParams{
		LoadBalancerArn:       loadBalancerArn,
		Protocol:              protocol,
		Port:                  port,
		DefaultTargetGroupArn: defaultTargetGroupArn,
		CertificateArn:        certificateArn,
	}

	resource, err := h.awsClient.CreateListener(ctx, params)
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to create listener: %s", err.Error()))
	}

	return h.createSuccessResponse("Listener created successfully", map[string]interface{}{
		"listenerArn":           resource.ID,
		"loadBalancerArn":       loadBalancerArn,
		"protocol":              protocol,
		"port":                  port,
		"defaultTargetGroupArn": defaultTargetGroupArn,
	})
}
