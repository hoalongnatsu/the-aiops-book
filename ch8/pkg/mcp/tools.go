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
	awsClient       *aws.Client
	resourceHandler *ResourceHandler
	logger          *logging.Logger
}

func NewToolHandler(awsClient *aws.Client, logger *logging.Logger) *ToolHandler {
	return &ToolHandler{
		awsClient:       awsClient,
		resourceHandler: NewResourceHandler(awsClient),
		logger:          logger,
	}
}

// CallTool handles requests for specific tools
func (h *ToolHandler) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	h.logger.LogMCPCallTool(name, arguments)

	switch name {
	// EC2 Instance Management
	case "create-ec2-instance":
		return h.createEC2Instance(ctx, arguments)
	case "start-ec2-instance":
		return h.startEC2Instance(ctx, arguments)
	case "stop-ec2-instance":
		return h.stopEC2Instance(ctx, arguments)
	case "terminate-ec2-instance":
		return h.terminateEC2Instance(ctx, arguments)
	case "create-ami-from-instance":
		return h.createAMIFromInstance(ctx, arguments)

	// VPC and Networking - Single Responsibility Functions
	case "create-vpc":
		return h.createVPC(ctx, arguments)
	case "create-subnet":
		return h.createSubnet(ctx, arguments)
	case "create-private-subnet":
		return h.createPrivateSubnet(ctx, arguments)
	case "create-public-subnet":
		return h.createPublicSubnet(ctx, arguments)
	case "create-internet-gateway":
		return h.createInternetGateway(ctx, arguments)
	case "create-nat-gateway":
		return h.createNATGateway(ctx, arguments)
	case "create-public-route-table":
		return h.createPublicRouteTable(ctx, arguments)
	case "create-private-route-table":
		return h.createPrivateRouteTable(ctx, arguments)
	case "associate-route-table":
		return h.associateRouteTable(ctx, arguments)

	// Security Groups - Single Responsibility Functions
	case "list-security-groups":
		return h.listSecurityGroups(ctx, arguments)
	case "create-security-group":
		return h.createSecurityGroup(ctx, arguments)
	case "add-security-group-ingress-rule":
		return h.addSecurityGroupIngressRule(ctx, arguments)
	case "add-security-group-egress-rule":
		return h.addSecurityGroupEgressRule(ctx, arguments)
	case "delete-security-group":
		return h.deleteSecurityGroup(ctx, arguments)

	// Auto Scaling
	case "create-launch-template":
		return h.createLaunchTemplate(ctx, arguments)
	case "create-auto-scaling-group":
		return h.createAutoScalingGroup(ctx, arguments)

	// Load Balancer
	case "create-load-balancer":
		return h.createApplicationLoadBalancer(ctx, arguments)
	case "create-target-group":
		return h.createTargetGroup(ctx, arguments)
	case "create-listener":
		return h.createListener(ctx, arguments)

	// RDS Database
	case "create-db-subnet-group":
		return h.createDBSubnetGroup(ctx, arguments)
	case "create-db-instance":
		return h.createDBInstance(ctx, arguments)
	case "start-db-instance":
		return h.startDBInstance(ctx, arguments)
	case "stop-db-instance":
		return h.stopDBInstance(ctx, arguments)
	case "delete-db-instance":
		return h.deleteDBInstance(ctx, arguments)
	case "create-db-snapshot":
		return h.createDBSnapshot(ctx, arguments)

	// List Tools (for querying resources)
	case "list-ec2-instances":
		return h.listEC2Instances(ctx)
	case "list-vpcs":
		return h.listVPCs(ctx)
	case "list-subnets":
		return h.listSubnets(ctx)
	case "list-auto-scaling-groups":
		return h.listAutoScalingGroups(ctx)
	case "list-load-balancers":
		return h.listLoadBalancers(ctx)
	case "list-target-groups":
		return h.listTargetGroups(ctx)
	case "list-launch-templates":
		return h.listLaunchTemplates(ctx)
	case "list-amis":
		return h.listAMIs(ctx)
	case "list-db-instances":
		return h.listDBInstances(ctx)
	case "list-db-snapshots":
		return h.listDBSnapshots(ctx)

	default:
		return h.createErrorResponse(fmt.Sprintf("unknown tool: %s", name))
	}
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
