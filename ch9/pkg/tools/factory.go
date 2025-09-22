package tools

import (
	"fmt"

	"aws-mcp-server/internal/config"
	"aws-mcp-server/internal/logging"
	"aws-mcp-server/pkg/aws"
	"aws-mcp-server/pkg/interfaces"
)

// ToolFactoryImpl implements the ToolFactory interface
type ToolFactoryImpl struct {
	awsClient *aws.Client
	logger    *logging.Logger
}

// ToolDependencies contains all dependencies needed to create tools
type ToolDependencies struct {
	AWSClient *aws.Client
	Config    *config.Config
}

// NewToolFactory creates a new tool factory
func NewToolFactory(awsClient *aws.Client, logger *logging.Logger) interfaces.ToolFactory {
	return &ToolFactoryImpl{
		awsClient: awsClient,
		logger:    logger,
	}
}

// CreateTool creates a tool by type
func (f *ToolFactoryImpl) CreateTool(toolType string, dependencies interface{}) (interfaces.MCPTool, error) {

	// Handle regular tool dependencies
	deps, ok := dependencies.(*ToolDependencies)
	if !ok {
		// Try value type as well
		if depsVal, valOk := dependencies.(ToolDependencies); valOk {
			deps = &depsVal
		} else {
			return nil, fmt.Errorf("invalid dependencies type for tool %s", toolType)
		}
	}

	switch toolType {
	// EC2 Tools
	case "create-ec2-instance":
		return NewCreateEC2InstanceTool(deps.AWSClient, f.logger), nil
	case "list-ec2-instances":
		return NewListEC2InstancesTool(deps.AWSClient, f.logger), nil
	case "start-ec2-instance":
		return NewStartEC2InstanceTool(deps.AWSClient, f.logger), nil
	case "stop-ec2-instance":
		return NewStopEC2InstanceTool(deps.AWSClient, f.logger), nil
	case "terminate-ec2-instance":
		return NewTerminateEC2InstanceTool(deps.AWSClient, f.logger), nil
	case "create-ami-from-instance":
		return NewCreateAMIFromInstanceTool(deps.AWSClient, f.logger), nil
	case "list-amis":
		return NewListAMIsTool(deps.AWSClient, f.logger), nil

	// VPC Tools
	case "create-vpc":
		return NewCreateVPCTool(deps.AWSClient, f.logger), nil
	case "list-vpcs":
		return NewListVPCsTool(deps.AWSClient, f.logger), nil
	case "create-subnet":
		return NewCreateSubnetTool(deps.AWSClient, f.logger), nil
	case "create-private-subnet":
		return NewCreatePrivateSubnetTool(deps.AWSClient, f.logger), nil
	case "create-public-subnet":
		return NewCreatePublicSubnetTool(deps.AWSClient, f.logger), nil
	case "list-subnets":
		return NewListSubnetsTool(deps.AWSClient, f.logger), nil
	case "select-subnets-for-alb":
		return NewSelectSubnetsForALBTool(deps.AWSClient, f.logger), nil
	case "create-internet-gateway":
		return NewCreateInternetGatewayTool(deps.AWSClient, f.logger), nil
	case "create-nat-gateway":
		return NewCreateNATGatewayTool(deps.AWSClient, f.logger), nil
	case "create-public-route-table":
		return NewCreatePublicRouteTableTool(deps.AWSClient, f.logger), nil
	case "create-private-route-table":
		return NewCreatePrivateRouteTableTool(deps.AWSClient, f.logger), nil
	case "associate-route-table":
		return NewAssociateRouteTableTool(deps.AWSClient, f.logger), nil
	case "add-route":
		return NewAddRouteTool(deps.AWSClient, f.logger), nil

	// Security Group Tools
	case "create-security-group":
		return NewCreateSecurityGroupTool(deps.AWSClient, f.logger), nil
	case "list-security-groups":
		return NewListSecurityGroupsTool(deps.AWSClient, f.logger), nil
	case "add-security-group-ingress-rule":
		return NewAddSecurityGroupIngressRuleTool(deps.AWSClient, f.logger), nil
	case "add-security-group-egress-rule":
		return NewAddSecurityGroupEgressRuleTool(deps.AWSClient, f.logger), nil
	case "delete-security-group":
		return NewDeleteSecurityGroupTool(deps.AWSClient, f.logger), nil

	// Auto Scaling Tools
	case "create-launch-template":
		return NewCreateLaunchTemplateTool(deps.AWSClient, f.logger), nil
	case "create-auto-scaling-group":
		return NewCreateAutoScalingGroupTool(deps.AWSClient, f.logger), nil
	case "list-auto-scaling-groups":
		return NewListAutoScalingGroupsTool(deps.AWSClient, f.logger), nil
	case "list-launch-templates":
		return NewListLaunchTemplatesTool(deps.AWSClient, f.logger), nil

	// Load Balancer Tools
	case "create-load-balancer":
		return NewCreateLoadBalancerTool(deps.AWSClient, f.logger), nil
	case "create-target-group":
		return NewCreateTargetGroupTool(deps.AWSClient, f.logger), nil
	case "create-listener":
		return NewCreateListenerTool(deps.AWSClient, f.logger), nil
	case "list-load-balancers":
		return NewListLoadBalancersTool(deps.AWSClient, f.logger), nil
	case "list-target-groups":
		return NewListTargetGroupsTool(deps.AWSClient, f.logger), nil
	case "register-targets":
		return NewRegisterTargetsTool(deps.AWSClient, f.logger), nil
	case "deregister-targets":
		return NewDeregisterTargetsTool(deps.AWSClient, f.logger), nil

	// AMI Tools
	case "get-latest-amazon-linux-ami":
		return NewGetLatestAmazonLinuxAMITool(deps.AWSClient, f.logger), nil
	case "get-latest-ubuntu-ami":
		return NewGetLatestUbuntuAMITool(deps.AWSClient, f.logger), nil
	case "get-latest-windows-ami":
		return NewGetLatestWindowsAMITool(deps.AWSClient, f.logger), nil

	// Zone Tools
	case "get-availability-zones":
		return NewGetAvailabilityZonesTool(deps.AWSClient, f.logger), nil

	// RDS Tools
	case "create-db-subnet-group":
		return NewCreateDBSubnetGroupTool(deps.AWSClient, f.logger), nil
	case "create-db-instance":
		return NewCreateDBInstanceTool(deps.AWSClient, f.logger), nil
	case "start-db-instance":
		return NewStartDBInstanceTool(deps.AWSClient, f.logger), nil
	case "stop-db-instance":
		return NewStopDBInstanceTool(deps.AWSClient, f.logger), nil
	case "delete-db-instance":
		return NewDeleteDBInstanceTool(deps.AWSClient, f.logger), nil
	case "create-db-snapshot":
		return NewCreateDBSnapshotTool(deps.AWSClient, f.logger), nil
	case "list-db-instances":
		return NewListDBInstancesTool(deps.AWSClient, f.logger), nil
	case "list-db-snapshots":
		return NewListDBSnapshotsTool(deps.AWSClient, f.logger), nil

	// State Management Tools
	default:
		return nil, fmt.Errorf("unsupported tool type: %s", toolType)
	}
}

// GetSupportedToolTypes returns all supported tool types
func (f *ToolFactoryImpl) GetSupportedToolTypes() []string {
	return []string{
		// EC2 Tools
		"create-ec2-instance",
		"list-ec2-instances",
		"start-ec2-instance",
		"stop-ec2-instance",
		"terminate-ec2-instance",
		"create-ami-from-instance",
		"list-amis",

		// VPC Tools
		"create-vpc",
		"list-vpcs",
		"create-subnet",
		"create-private-subnet",
		"create-public-subnet",
		"list-subnets",
		"select-subnets-for-alb",
		"create-internet-gateway",
		"create-nat-gateway",
		"create-public-route-table",
		"create-private-route-table",
		"associate-route-table",
		"add-route",

		// Security Group Tools
		"create-security-group",
		"list-security-groups",
		"add-security-group-ingress-rule",
		"add-security-group-egress-rule",
		"delete-security-group",

		// Auto Scaling Tools
		"create-launch-template",
		"create-auto-scaling-group",
		"list-auto-scaling-groups",
		"list-launch-templates",

		// Load Balancer Tools
		"create-load-balancer",
		"create-target-group",
		"create-listener",
		"list-load-balancers",
		"list-target-groups",
		"register-targets",
		"deregister-targets",

		// AMI Tools
		"get-latest-amazon-linux-ami",
		"get-latest-ubuntu-ami",
		"get-latest-windows-ami",

		// Zone Tools
		"get-availability-zones",

		// RDS Tools
		"create-db-subnet-group",
		"create-db-instance",
		"start-db-instance",
		"stop-db-instance",
		"delete-db-instance",
		"create-db-snapshot",
		"list-db-instances",
		"list-db-snapshots",
	}
}

// ToolRegistrationHelper helps register all standard tools
type ToolRegistrationHelper struct {
	factory  interfaces.ToolFactory
	registry interfaces.ToolRegistry
	logger   *logging.Logger
}

// NewToolRegistrationHelper creates a new registration helper
func NewToolRegistrationHelper(factory interfaces.ToolFactory, registry interfaces.ToolRegistry, logger *logging.Logger) *ToolRegistrationHelper {
	return &ToolRegistrationHelper{
		factory:  factory,
		registry: registry,
		logger:   logger,
	}
}

// RegisterAllTools registers all available tools
func (h *ToolRegistrationHelper) RegisterAllTools(deps ToolDependencies) error {
	supportedTools := h.factory.GetSupportedToolTypes()

	for _, toolType := range supportedTools {
		tool, err := h.factory.CreateTool(toolType, deps)
		if err != nil {
			h.logger.WithError(err).WithField("toolType", toolType).Warn("Skipping tool creation (not implemented)")
			continue
		}

		if err := h.registry.Register(tool); err != nil {
			h.logger.WithError(err).WithField("toolType", toolType).Error("Failed to register tool")
			continue
		}

		h.logger.WithField("toolType", toolType).Info("Successfully registered tool")
	}

	return nil
}
