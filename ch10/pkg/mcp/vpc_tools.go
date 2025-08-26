package mcp

import (
	"aws-mcp-server/pkg/aws"
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// VPCToolsInterface defines all available VPC and networking tools
// Following Single Responsibility Principle - each tool creates one specific resource type
//
// Available Tools:
//   - listVPCs()                    : List all VPCs in the region
//   - listSubnets()                 : List all subnets in the region
//   - createVPC()                   : Create a VPC (aws_vpc equivalent)
//   - createPrivateSubnet()         : Create a private subnet (aws_subnet with private config)
//   - createPublicSubnet()          : Create a public subnet (aws_subnet with public config)
//   - createInternetGateway()       : Create an Internet Gateway (aws_internet_gateway)
//   - createPublicRouteTable()      : Create a public route table (aws_route_table for public)
//   - createPrivateRouteTable()     : Create a private route table (aws_route_table for private)
//   - associateRouteTable()         : Associate route table with subnet (aws_route_table_association)
//   - createNATGateway()            : Create a NAT Gateway with EIP (aws_eip + aws_nat_gateway)
//   - createSubnet()                : Legacy method - creates public/private based on mapPublicIpOnLaunch
//
// Usage Example (Terraform-like workflow):
//   1. createVPC(name="my-vpc", cidrBlock="10.0.0.0/16")
//   2. createPublicSubnet(vpcId="vpc-xxx", cidrBlock="10.0.1.0/24", availabilityZone="us-west-2a")
//   3. createPrivateSubnet(vpcId="vpc-xxx", cidrBlock="10.0.10.0/24", availabilityZone="us-west-2a")
//   4. createInternetGateway(vpcId="vpc-xxx", name="my-igw")
//   5. createNATGateway(subnetId="subnet-public", name="my-nat")
//   6. createPublicRouteTable(vpcId="vpc-xxx", internetGatewayId="igw-xxx")
//   7. createPrivateRouteTable(vpcId="vpc-xxx", natGatewayId="nat-xxx")
//   8. associateRouteTable(routeTableId="rtb-xxx", subnetId="subnet-xxx")

func (h *ToolHandler) listVPCs(ctx context.Context) (*mcp.CallToolResult, error) {
	result, err := h.resourceHandler.ReadResource(ctx, "aws://vpc/vpcs")
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to list VPCs: %s", err.Error()))
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

func (h *ToolHandler) listSubnets(ctx context.Context) (*mcp.CallToolResult, error) {
	result, err := h.resourceHandler.ReadResource(ctx, "aws://vpc/subnets")
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to list subnets: %s", err.Error()))
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

// ========== Single Responsibility Functions for VPC Resources ==========
// Each function creates exactly one type of AWS resource, following Terraform structure

// createVPC - Creates only a VPC resource (equivalent to aws_vpc in Terraform)
func (h *ToolHandler) createVPC(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	name, ok := arguments["name"].(string)
	if !ok || name == "" {
		return h.createErrorResponse("name is required")
	}

	cidrBlock, ok := arguments["cidrBlock"].(string)
	if !ok || cidrBlock == "" {
		cidrBlock = "10.0.0.0/16" // Default CIDR
	}

	enableDnsHostnames := true
	if val, exists := arguments["enableDnsHostnames"]; exists {
		enableDnsHostnames, _ = val.(bool)
	}

	enableDnsSupport := true
	if val, exists := arguments["enableDnsSupport"]; exists {
		enableDnsSupport, _ = val.(bool)
	}

	params := aws.CreateVPCParams{
		Name:               name,
		CidrBlock:          cidrBlock,
		EnableDnsHostnames: enableDnsHostnames,
		EnableDnsSupport:   enableDnsSupport,
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

	vpcResource, err := h.awsClient.CreateVPC(ctx, params)
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to create VPC: %s", err.Error()))
	}

	return h.createSuccessResponse("VPC created successfully", map[string]interface{}{
		"vpcId":              vpcResource.ID,
		"name":               name,
		"cidrBlock":          cidrBlock,
		"enableDnsHostnames": enableDnsHostnames,
		"enableDnsSupport":   enableDnsSupport,
		"tags":               vpcResource.Tags,
		"state":              vpcResource.State,
	})
}

// createPrivateSubnet - Creates a private subnet (equivalent to aws_subnet with private config)
func (h *ToolHandler) createPrivateSubnet(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	vpcID, ok := arguments["vpcId"].(string)
	if !ok || vpcID == "" {
		return h.createErrorResponse("vpcId is required")
	}

	cidrBlock, ok := arguments["cidrBlock"].(string)
	if !ok || cidrBlock == "" {
		return h.createErrorResponse("cidrBlock is required")
	}

	availabilityZone, ok := arguments["availabilityZone"].(string)
	if !ok || availabilityZone == "" {
		return h.createErrorResponse("availabilityZone is required")
	}

	name, _ := arguments["name"].(string)
	if name == "" {
		name = "private-subnet"
	}

	params := aws.CreateSubnetParams{
		VpcID:               vpcID,
		CidrBlock:           cidrBlock,
		AvailabilityZone:    availabilityZone,
		MapPublicIpOnLaunch: false, // Private subnet - no public IP assignment
		Name:                name,
		Tags: map[string]string{
			"Name": name,
			"Type": "private",
		},
	}

	// Add optional tags
	if tags, exists := arguments["tags"].(map[string]interface{}); exists {
		for k, v := range tags {
			if strVal, ok := v.(string); ok {
				params.Tags[k] = strVal
			}
		}
	}

	resource, err := h.awsClient.CreateSubnet(ctx, params)
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to create private subnet: %s", err.Error()))
	}

	return h.createSuccessResponse("Private subnet created successfully", map[string]interface{}{
		"subnetId":            resource.ID,
		"name":                name,
		"vpcId":               vpcID,
		"cidrBlock":           cidrBlock,
		"availabilityZone":    availabilityZone,
		"mapPublicIpOnLaunch": false,
		"type":                "private",
		"state":               resource.State,
		"tags":                resource.Tags,
	})
}

// createPublicSubnet - Creates a public subnet (equivalent to aws_subnet with public config)
func (h *ToolHandler) createPublicSubnet(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	vpcID, ok := arguments["vpcId"].(string)
	if !ok || vpcID == "" {
		return h.createErrorResponse("vpcId is required")
	}

	cidrBlock, ok := arguments["cidrBlock"].(string)
	if !ok || cidrBlock == "" {
		return h.createErrorResponse("cidrBlock is required")
	}

	availabilityZone, ok := arguments["availabilityZone"].(string)
	if !ok || availabilityZone == "" {
		return h.createErrorResponse("availabilityZone is required")
	}

	name, _ := arguments["name"].(string)
	if name == "" {
		name = "public-subnet"
	}

	params := aws.CreateSubnetParams{
		VpcID:               vpcID,
		CidrBlock:           cidrBlock,
		AvailabilityZone:    availabilityZone,
		MapPublicIpOnLaunch: true, // Public subnet - auto-assign public IP
		Name:                name,
		Tags: map[string]string{
			"Name": name,
			"Type": "public",
		},
	}

	// Add optional tags
	if tags, exists := arguments["tags"].(map[string]interface{}); exists {
		for k, v := range tags {
			if strVal, ok := v.(string); ok {
				params.Tags[k] = strVal
			}
		}
	}

	resource, err := h.awsClient.CreateSubnet(ctx, params)
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to create public subnet: %s", err.Error()))
	}

	return h.createSuccessResponse("Public subnet created successfully", map[string]interface{}{
		"subnetId":            resource.ID,
		"name":                name,
		"vpcId":               vpcID,
		"cidrBlock":           cidrBlock,
		"availabilityZone":    availabilityZone,
		"mapPublicIpOnLaunch": true,
		"type":                "public",
		"state":               resource.State,
		"tags":                resource.Tags,
	})
}

// createInternetGateway - Creates an Internet Gateway (equivalent to aws_internet_gateway)
func (h *ToolHandler) createInternetGateway(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	name, _ := arguments["name"].(string)
	if name == "" {
		name = "custom-igw"
	}

	vpcID, ok := arguments["vpcId"].(string)
	if !ok || vpcID == "" {
		return h.createErrorResponse("vpcId is required")
	}

	params := aws.CreateInternetGatewayParams{
		Name: name,
		Tags: map[string]string{
			"Name": name,
		},
	}

	// Add optional tags
	if tags, exists := arguments["tags"].(map[string]interface{}); exists {
		for k, v := range tags {
			if strVal, ok := v.(string); ok {
				params.Tags[k] = strVal
			}
		}
	}

	resource, err := h.awsClient.CreateInternetGateway(ctx, params, vpcID)
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to create internet gateway: %s", err.Error()))
	}

	return h.createSuccessResponse("Internet Gateway created successfully", map[string]interface{}{
		"internetGatewayId": resource.ID,
		"name":              name,
		"vpcId":             vpcID,
		"state":             resource.State,
		"tags":              resource.Tags,
	})
}

// createPublicRouteTable - Creates a public route table (equivalent to aws_route_table for public)
func (h *ToolHandler) createPublicRouteTable(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	vpcID, ok := arguments["vpcId"].(string)
	if !ok || vpcID == "" {
		return h.createErrorResponse("vpcId is required")
	}

	internetGatewayID, ok := arguments["internetGatewayId"].(string)
	if !ok || internetGatewayID == "" {
		return h.createErrorResponse("internetGatewayId is required")
	}

	name, _ := arguments["name"].(string)
	if name == "" {
		name = "public-route-table"
	}

	// Create route table
	routeTable, err := h.awsClient.CreateRouteTable(ctx, vpcID, name)
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to create public route table: %s", err.Error()))
	}

	// Add route to Internet Gateway
	if err := h.awsClient.CreateRoute(ctx, routeTable.ID, "0.0.0.0/0", internetGatewayID); err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to create route to Internet Gateway: %s", err.Error()))
	}

	return h.createSuccessResponse("Public route table created successfully", map[string]interface{}{
		"routeTableId":      routeTable.ID,
		"name":              name,
		"vpcId":             vpcID,
		"internetGatewayId": internetGatewayID,
		"routes": []map[string]interface{}{
			{
				"destination": "0.0.0.0/0",
				"target":      internetGatewayID,
				"type":        "internet-gateway",
			},
		},
		"tags": routeTable.Tags,
	})
}

// createPrivateRouteTable - Creates a private route table (equivalent to aws_route_table for private)
func (h *ToolHandler) createPrivateRouteTable(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	vpcID, ok := arguments["vpcId"].(string)
	if !ok || vpcID == "" {
		return h.createErrorResponse("vpcId is required")
	}

	natGatewayID, ok := arguments["natGatewayId"].(string)
	if !ok || natGatewayID == "" {
		return h.createErrorResponse("natGatewayId is required")
	}

	name, _ := arguments["name"].(string)
	if name == "" {
		name = "private-route-table"
	}

	// Create route table
	routeTable, err := h.awsClient.CreateRouteTable(ctx, vpcID, name)
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to create private route table: %s", err.Error()))
	}

	// Add route to NAT Gateway
	if err := h.awsClient.CreateRouteForNAT(ctx, routeTable.ID, "0.0.0.0/0", natGatewayID); err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to create route to NAT Gateway: %s", err.Error()))
	}

	return h.createSuccessResponse("Private route table created successfully", map[string]interface{}{
		"routeTableId": routeTable.ID,
		"name":         name,
		"vpcId":        vpcID,
		"natGatewayId": natGatewayID,
		"routes": []map[string]interface{}{
			{
				"destination": "0.0.0.0/0",
				"target":      natGatewayID,
				"type":        "nat-gateway",
			},
		},
		"tags": routeTable.Tags,
	})
}

// associateRouteTable - Associates a route table with a subnet (equivalent to aws_route_table_association)
func (h *ToolHandler) associateRouteTable(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	routeTableID, ok := arguments["routeTableId"].(string)
	if !ok || routeTableID == "" {
		return h.createErrorResponse("routeTableId is required")
	}

	subnetID, ok := arguments["subnetId"].(string)
	if !ok || subnetID == "" {
		return h.createErrorResponse("subnetId is required")
	}

	err := h.awsClient.AssociateRouteTable(ctx, routeTableID, subnetID)
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to associate route table: %s", err.Error()))
	}

	return h.createSuccessResponse("Route table association created successfully", map[string]interface{}{
		"routeTableId": routeTableID,
		"subnetId":     subnetID,
		"associated":   true,
	})
}

// createNATGateway - Creates a NAT Gateway (equivalent to aws_eip + aws_nat_gateway)
func (h *ToolHandler) createNATGateway(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	subnetID, ok := arguments["subnetId"].(string)
	if !ok || subnetID == "" {
		return h.createErrorResponse("subnetId is required (must be a public subnet)")
	}

	name, _ := arguments["name"].(string)
	if name == "" {
		name = "public-nat"
	}

	params := aws.CreateNATGatewayParams{
		SubnetID: subnetID,
		Name:     name,
		Tags: map[string]string{
			"Name": name,
		},
	}

	// Add optional tags
	if tags, exists := arguments["tags"].(map[string]interface{}); exists {
		for k, v := range tags {
			if strVal, ok := v.(string); ok {
				params.Tags[k] = strVal
			}
		}
	}

	natResource, err := h.awsClient.CreateNATGateway(ctx, params)
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to create NAT Gateway: %s", err.Error()))
	}

	// Wait for NAT Gateway to become available
	h.logger.Info("Waiting for NAT Gateway to become available...")
	if err := h.awsClient.WaitForNATGateway(ctx, natResource.ID); err != nil {
		return h.createErrorResponse(fmt.Sprintf("NAT Gateway did not become available: %s", err.Error()))
	}

	return h.createSuccessResponse("NAT Gateway created successfully", map[string]interface{}{
		"natGatewayId": natResource.ID,
		"name":         name,
		"subnetId":     subnetID,
		"eipId":        natResource.Details["eipId"],
		"publicIp":     natResource.Details["publicIp"],
		"privateIp":    natResource.Details["privateIp"],
		"state":        natResource.State,
		"tags":         natResource.Tags,
	})
}

// Legacy method kept for backward compatibility but simplified
func (h *ToolHandler) createSubnet(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// Determine if this should be a public or private subnet based on mapPublicIpOnLaunch
	mapPublicIpOnLaunch := false
	if val, exists := arguments["mapPublicIpOnLaunch"]; exists {
		mapPublicIpOnLaunch, _ = val.(bool)
	}

	if mapPublicIpOnLaunch {
		return h.createPublicSubnet(ctx, arguments)
	} else {
		return h.createPrivateSubnet(ctx, arguments)
	}
}
