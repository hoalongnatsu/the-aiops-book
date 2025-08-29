package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"aws-mcp-server/internal/config"
	"aws-mcp-server/internal/logging"
	"aws-mcp-server/pkg/aws"
	"aws-mcp-server/pkg/conflict"
	"aws-mcp-server/pkg/discovery"
	"aws-mcp-server/pkg/graph"
	"aws-mcp-server/pkg/state"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Server struct {
	Config           *config.Config
	AWSClient        *aws.Client
	Logger           *logging.Logger
	StateManager     *state.Manager
	DiscoveryScanner *discovery.Scanner
	GraphManager     *graph.Manager
	GraphAnalyzer    *graph.Analyzer
	ConflictResolver *conflict.Resolver

	resourceHandler *ResourceHandler
	toolHandler     *ToolHandler
	mcpServer       *server.MCPServer
}

func NewServer(cfg *config.Config, awsClient *aws.Client, logger *logging.Logger) *Server {
	// Initialize individual components
	stateManager := state.NewManager(cfg.State.FilePath, cfg.AWS.Region, logger)
	discoveryScanner := discovery.NewScanner(awsClient, logger)
	graphManager := graph.NewManager(logger)
	graphAnalyzer := graph.NewAnalyzer(graphManager)
	conflictResolver := conflict.NewResolver(logger)

	// Create MCP server
	mcpServer := server.NewMCPServer(
		cfg.MCP.ServerName,
		cfg.MCP.Version,
		server.WithResourceCapabilities(true, true),
		server.WithToolCapabilities(true),
	)

	s := &Server{
		Config:           cfg,
		AWSClient:        awsClient,
		Logger:           logger,
		StateManager:     stateManager,
		DiscoveryScanner: discoveryScanner,
		GraphManager:     graphManager,
		GraphAnalyzer:    graphAnalyzer,
		ConflictResolver: conflictResolver,
		resourceHandler:  NewResourceHandler(awsClient),
		toolHandler:      NewToolHandler(awsClient, logger),
		mcpServer:        mcpServer,
	}

	// Register resources
	s.registerResources()

	// Register tools
	s.registerTools()

	// Register state-aware tools
	s.registerStateAwareTools()

	// Load existing state from file
	if err := s.StateManager.LoadState(context.Background()); err != nil {
		logger.WithError(err).Error("Failed to load infrastructure state, continuing with empty state")
		// Don't fail initialization, just log the error and continue with empty state
	}

	return s
}

// registerResources sets up all the MCP resources
func (s *Server) registerResources() {
	// Register EC2 instances list resource
	s.mcpServer.AddResource(
		mcp.NewResource("aws://ec2/instances", "EC2 Instances",
			mcp.WithResourceDescription("List all EC2 instances in the region"),
			mcp.WithMIMEType("application/json"),
		),
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			s.Logger.Info("Received request for EC2 instances list")

			// Use our resource handler to get the instances
			result, err := s.resourceHandler.ReadResource(ctx, "aws://ec2/instances")
			if err != nil {
				s.Logger.WithError(err).Error("Failed to read EC2 instances resource")
				return nil, err
			}

			return result.Contents, nil
		},
	)

	// Register EC2 instance details resource template (supports dynamic instance IDs)
	template := mcp.NewResourceTemplate(
		"aws://ec2/instances/{instanceId}",
		"EC2 Instance Details",
		mcp.WithTemplateDescription("Detailed information about a specific EC2 instance"),
		mcp.WithTemplateMIMEType("application/json"),
	)

	s.mcpServer.AddResourceTemplate(template, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		s.Logger.WithField("uri", request.Params.URI).Info("Received read resource request for specific EC2 instance")

		// The server automatically matches URIs to templates, so we can use the full URI directly
		result, err := s.resourceHandler.ReadResource(ctx, request.Params.URI)
		if err != nil {
			s.Logger.WithError(err).WithField("uri", request.Params.URI).Error("Failed to read resource")
			return nil, err
		}

		return result.Contents, nil
	})

	// Register VPC list resource
	s.mcpServer.AddResource(
		mcp.NewResource("aws://vpc/vpcs", "VPCs",
			mcp.WithResourceDescription("List all VPCs in the region"),
			mcp.WithMIMEType("application/json"),
		),
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			s.Logger.Info("Received request for VPCs list")

			result, err := s.resourceHandler.ReadResource(ctx, "aws://vpc/vpcs")
			if err != nil {
				s.Logger.WithError(err).Error("Failed to read VPCs resource")
				return nil, err
			}

			return result.Contents, nil
		},
	)

	// Register VPC details resource template
	vpcTemplate := mcp.NewResourceTemplate(
		"aws://vpc/vpcs/{vpcId}",
		"VPC Details",
		mcp.WithTemplateDescription("Detailed information about a specific VPC"),
		mcp.WithTemplateMIMEType("application/json"),
	)

	s.mcpServer.AddResourceTemplate(vpcTemplate, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		s.Logger.WithField("uri", request.Params.URI).Info("Received read resource request for specific VPC")

		result, err := s.resourceHandler.ReadResource(ctx, request.Params.URI)
		if err != nil {
			s.Logger.WithError(err).WithField("uri", request.Params.URI).Error("Failed to read VPC resource")
			return nil, err
		}

		return result.Contents, nil
	})

	// Register Subnets list resource
	s.mcpServer.AddResource(
		mcp.NewResource("aws://vpc/subnets", "Subnets",
			mcp.WithResourceDescription("List all subnets in the region"),
			mcp.WithMIMEType("application/json"),
		),
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			s.Logger.Info("Received request for subnets list")

			result, err := s.resourceHandler.ReadResource(ctx, "aws://vpc/subnets")
			if err != nil {
				s.Logger.WithError(err).Error("Failed to read subnets resource")
				return nil, err
			}

			return result.Contents, nil
		},
	)

	// Register Subnet details resource template
	subnetTemplate := mcp.NewResourceTemplate(
		"aws://vpc/subnets/{subnetId}",
		"Subnet Details",
		mcp.WithTemplateDescription("Detailed information about a specific subnet"),
		mcp.WithTemplateMIMEType("application/json"),
	)

	s.mcpServer.AddResourceTemplate(subnetTemplate, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		s.Logger.WithField("uri", request.Params.URI).Info("Received read resource request for specific subnet")

		result, err := s.resourceHandler.ReadResource(ctx, request.Params.URI)
		if err != nil {
			s.Logger.WithError(err).WithField("uri", request.Params.URI).Error("Failed to read subnet resource")
			return nil, err
		}

		return result.Contents, nil
	})

	// Register Auto Scaling Groups list resource
	s.mcpServer.AddResource(
		mcp.NewResource("aws://autoscaling/groups", "Auto Scaling Groups",
			mcp.WithResourceDescription("List all Auto Scaling Groups in the region"),
			mcp.WithMIMEType("application/json"),
		),
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			s.Logger.Info("Received request for Auto Scaling Groups list")

			result, err := s.resourceHandler.ReadResource(ctx, "aws://autoscaling/groups")
			if err != nil {
				s.Logger.WithError(err).Error("Failed to read Auto Scaling Groups resource")
				return nil, err
			}

			return result.Contents, nil
		},
	)

	// Register Auto Scaling Group details resource template
	asgTemplate := mcp.NewResourceTemplate(
		"aws://autoscaling/groups/{groupName}",
		"Auto Scaling Group Details",
		mcp.WithTemplateDescription("Detailed information about a specific Auto Scaling Group"),
		mcp.WithTemplateMIMEType("application/json"),
	)

	s.mcpServer.AddResourceTemplate(asgTemplate, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		s.Logger.WithField("uri", request.Params.URI).Info("Received read resource request for specific Auto Scaling Group")

		result, err := s.resourceHandler.ReadResource(ctx, request.Params.URI)
		if err != nil {
			s.Logger.WithError(err).WithField("uri", request.Params.URI).Error("Failed to read Auto Scaling Group resource")
			return nil, err
		}

		return result.Contents, nil
	})

	// Register Load Balancers list resource
	s.mcpServer.AddResource(
		mcp.NewResource("aws://elbv2/loadbalancers", "Load Balancers",
			mcp.WithResourceDescription("List all Application Load Balancers in the region"),
			mcp.WithMIMEType("application/json"),
		),
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			s.Logger.Info("Received request for Load Balancers list")

			result, err := s.resourceHandler.ReadResource(ctx, "aws://elbv2/loadbalancers")
			if err != nil {
				s.Logger.WithError(err).Error("Failed to read Load Balancers resource")
				return nil, err
			}

			return result.Contents, nil
		},
	)

	// Register Load Balancer details resource template
	albTemplate := mcp.NewResourceTemplate(
		"aws://elbv2/loadbalancers/{loadBalancerArn}",
		"Load Balancer Details",
		mcp.WithTemplateDescription("Detailed information about a specific Load Balancer"),
		mcp.WithTemplateMIMEType("application/json"),
	)

	s.mcpServer.AddResourceTemplate(albTemplate, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		s.Logger.WithField("uri", request.Params.URI).Info("Received read resource request for specific Load Balancer")

		result, err := s.resourceHandler.ReadResource(ctx, request.Params.URI)
		if err != nil {
			s.Logger.WithError(err).WithField("uri", request.Params.URI).Error("Failed to read Load Balancer resource")
			return nil, err
		}

		return result.Contents, nil
	})

	// Register Target Groups list resource
	s.mcpServer.AddResource(
		mcp.NewResource("aws://elbv2/targetgroups", "Target Groups",
			mcp.WithResourceDescription("List all Target Groups in the region"),
			mcp.WithMIMEType("application/json"),
		),
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			s.Logger.Info("Received request for Target Groups list")

			result, err := s.resourceHandler.ReadResource(ctx, "aws://elbv2/targetgroups")
			if err != nil {
				s.Logger.WithError(err).Error("Failed to read Target Groups resource")
				return nil, err
			}

			return result.Contents, nil
		},
	)

	// Register Target Group details resource template
	tgTemplate := mcp.NewResourceTemplate(
		"aws://elbv2/targetgroups/{targetGroupArn}",
		"Target Group Details",
		mcp.WithTemplateDescription("Detailed information about a specific Target Group"),
		mcp.WithTemplateMIMEType("application/json"),
	)

	s.mcpServer.AddResourceTemplate(tgTemplate, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		s.Logger.WithField("uri", request.Params.URI).Info("Received read resource request for specific Target Group")

		result, err := s.resourceHandler.ReadResource(ctx, request.Params.URI)
		if err != nil {
			s.Logger.WithError(err).WithField("uri", request.Params.URI).Error("Failed to read Target Group resource")
			return nil, err
		}

		return result.Contents, nil
	})

	// Register Launch Templates list resource
	s.mcpServer.AddResource(
		mcp.NewResource("aws://ec2/launchtemplates", "Launch Templates",
			mcp.WithResourceDescription("List all Launch Templates in the region"),
			mcp.WithMIMEType("application/json"),
		),
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			s.Logger.Info("Received request for Launch Templates list")

			result, err := s.resourceHandler.ReadResource(ctx, "aws://ec2/launchtemplates")
			if err != nil {
				s.Logger.WithError(err).Error("Failed to read Launch Templates resource")
				return nil, err
			}

			return result.Contents, nil
		},
	)

	// Register Launch Template details resource template
	ltTemplate := mcp.NewResourceTemplate(
		"aws://ec2/launchtemplates/{templateId}",
		"Launch Template Details",
		mcp.WithTemplateDescription("Detailed information about a specific Launch Template"),
		mcp.WithTemplateMIMEType("application/json"),
	)

	s.mcpServer.AddResourceTemplate(ltTemplate, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		s.Logger.WithField("uri", request.Params.URI).Info("Received read resource request for specific Launch Template")

		result, err := s.resourceHandler.ReadResource(ctx, request.Params.URI)
		if err != nil {
			s.Logger.WithError(err).WithField("uri", request.Params.URI).Error("Failed to read Launch Template resource")
			return nil, err
		}

		return result.Contents, nil
	})

	// Register AMIs list resource
	s.mcpServer.AddResource(
		mcp.NewResource("aws://ec2/images", "AMIs",
			mcp.WithResourceDescription("List all AMIs owned by the account in the region"),
			mcp.WithMIMEType("application/json"),
		),
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			s.Logger.Info("Received request for AMIs list")

			result, err := s.resourceHandler.ReadResource(ctx, "aws://ec2/images")
			if err != nil {
				s.Logger.WithError(err).Error("Failed to read AMIs resource")
				return nil, err
			}

			return result.Contents, nil
		},
	)

	// Register AMI details resource template
	amiTemplate := mcp.NewResourceTemplate(
		"aws://ec2/images/{imageId}",
		"AMI Details",
		mcp.WithTemplateDescription("Detailed information about a specific AMI"),
		mcp.WithTemplateMIMEType("application/json"),
	)

	s.mcpServer.AddResourceTemplate(amiTemplate, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		s.Logger.WithField("uri", request.Params.URI).Info("Received read resource request for specific AMI")

		result, err := s.resourceHandler.ReadResource(ctx, request.Params.URI)
		if err != nil {
			s.Logger.WithError(err).WithField("uri", request.Params.URI).Error("Failed to read AMI resource")
			return nil, err
		}

		return result.Contents, nil
	})

	// ========== RDS Resources ==========

	// Register RDS instances list resource
	s.mcpServer.AddResource(
		mcp.NewResource("aws://rds/instances", "RDS Instances",
			mcp.WithResourceDescription("List all RDS database instances in the region"),
			mcp.WithMIMEType("application/json"),
		),
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			s.Logger.Info("Received request for RDS instances list")

			result, err := s.resourceHandler.ReadResource(ctx, "aws://rds/instances")
			if err != nil {
				s.Logger.WithError(err).Error("Failed to read RDS instances resource")
				return nil, err
			}

			return result.Contents, nil
		},
	)

	// Register RDS instance details resource template
	rdsTemplate := mcp.NewResourceTemplate(
		"aws://rds/instances/{dbInstanceIdentifier}",
		"RDS Instance Details",
		mcp.WithTemplateDescription("Detailed information about a specific RDS database instance"),
		mcp.WithTemplateMIMEType("application/json"),
	)

	s.mcpServer.AddResourceTemplate(rdsTemplate, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		s.Logger.WithField("uri", request.Params.URI).Info("Received read resource request for specific RDS instance")

		result, err := s.resourceHandler.ReadResource(ctx, request.Params.URI)
		if err != nil {
			s.Logger.WithError(err).WithField("uri", request.Params.URI).Error("Failed to read RDS instance resource")
			return nil, err
		}

		return result.Contents, nil
	})

	// Register RDS snapshots list resource
	s.mcpServer.AddResource(
		mcp.NewResource("aws://rds/snapshots", "RDS Snapshots",
			mcp.WithResourceDescription("List all RDS database snapshots in the region"),
			mcp.WithMIMEType("application/json"),
		),
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			s.Logger.Info("Received request for RDS snapshots list")

			result, err := s.resourceHandler.ReadResource(ctx, "aws://rds/snapshots")
			if err != nil {
				s.Logger.WithError(err).Error("Failed to read RDS snapshots resource")
				return nil, err
			}

			return result.Contents, nil
		},
	)
}

// registerTools sets up all the MCP tools
// NOTE: In production, it's better to declare tools as an array of structs and use a loop
// to register them. This approach reduces code duplication and makes it easier to manage
// many tools. For this chapter, we write each tool registration separately to make the
// code cleaner and easier to understand.
//
// Production approach would look like:
//
//	type ToolDefinition struct {
//	    Name        string
//	    Description string
//	    Parameters  []mcp.ToolParameter
//	    Handler     string
//	}
//
//	tools := []ToolDefinition{
//	    {Name: "create-ec2-instance", Description: "Create a new EC2 instance", ...},
//	    {Name: "start-ec2-instance", Description: "Start a stopped EC2 instance", ...},
//	    // ... more tools
//	}
//
//	for _, tool := range tools {
//	    s.mcpServer.AddTool(mcp.NewTool(tool.Name, tool.Parameters...), s.getHandlerFunc(tool.Handler))
//	}
func (s *Server) registerTools() {
	// Register create EC2 instance tool
	s.mcpServer.AddTool(
		mcp.NewTool("create-ec2-instance",
			mcp.WithDescription("Create a new EC2 instance"),
			mcp.WithString("imageId", mcp.Description("AMI ID to use for the instance"), mcp.Required()),
			mcp.WithString("instanceType", mcp.Description("EC2 instance type (e.g., t2.micro, t3.small)"), mcp.Required()),
			mcp.WithString("keyName", mcp.Description("Name of the key pair to use for SSH access")),
			mcp.WithString("securityGroupId", mcp.Description("Security group ID to assign to the instance")),
			mcp.WithString("subnetId", mcp.Description("Subnet ID where the instance should be launched")),
			mcp.WithString("name", mcp.Description("Name tag for the instance")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.toolHandler.CallTool(ctx, "create-ec2-instance", arguments)
		},
	)

	// Register start EC2 instance tool
	s.mcpServer.AddTool(
		mcp.NewTool("start-ec2-instance",
			mcp.WithDescription("Start a stopped EC2 instance"),
			mcp.WithString("instanceId", mcp.Description("EC2 instance ID to start"), mcp.Required()),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.toolHandler.CallTool(ctx, "start-ec2-instance", arguments)
		},
	)

	// Register stop EC2 instance tool
	s.mcpServer.AddTool(
		mcp.NewTool("stop-ec2-instance",
			mcp.WithDescription("Stop a running EC2 instance"),
			mcp.WithString("instanceId", mcp.Description("EC2 instance ID to stop"), mcp.Required()),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.toolHandler.CallTool(ctx, "stop-ec2-instance", arguments)
		},
	)

	// Register terminate EC2 instance tool
	s.mcpServer.AddTool(
		mcp.NewTool("terminate-ec2-instance",
			mcp.WithDescription("Terminate an EC2 instance (permanent deletion)"),
			mcp.WithString("instanceId", mcp.Description("EC2 instance ID to terminate"), mcp.Required()),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.toolHandler.CallTool(ctx, "terminate-ec2-instance", arguments)
		},
	)

	// Register VPC creation tool
	s.mcpServer.AddTool(
		mcp.NewTool("create-vpc",
			mcp.WithDescription("Create a new VPC with optional subnets and gateways"),
			mcp.WithString("name", mcp.Description("Name for the VPC"), mcp.Required()),
			mcp.WithString("cidrBlock", mcp.Description("CIDR block for the VPC (e.g., 10.0.0.0/16)")),
			mcp.WithString("setupType", mcp.Description("VPC setup type: vpc-only, public-only, or public-private")),
			mcp.WithBoolean("enableDnsHostnames", mcp.Description("Enable DNS hostnames for the VPC")),
			mcp.WithBoolean("enableDnsSupport", mcp.Description("Enable DNS support for the VPC")),
			mcp.WithObject("tags", mcp.Description("Additional tags for the VPC")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.toolHandler.CallTool(ctx, "create-vpc", arguments)
		},
	)

	// Register subnet creation tool
	s.mcpServer.AddTool(
		mcp.NewTool("create-subnet",
			mcp.WithDescription("Create a subnet within a VPC"),
			mcp.WithString("vpcId", mcp.Description("VPC ID where the subnet will be created"), mcp.Required()),
			mcp.WithString("cidrBlock", mcp.Description("CIDR block for the subnet"), mcp.Required()),
			mcp.WithString("availabilityZone", mcp.Description("Availability zone for the subnet"), mcp.Required()),
			mcp.WithString("name", mcp.Description("Name for the subnet")),
			mcp.WithBoolean("mapPublicIpOnLaunch", mcp.Description("Auto-assign public IP addresses")),
			mcp.WithObject("tags", mcp.Description("Additional tags for the subnet")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.toolHandler.CallTool(ctx, "create-subnet", arguments)
		},
	)

	// Register Internet Gateway creation tool
	s.mcpServer.AddTool(
		mcp.NewTool("create-internet-gateway",
			mcp.WithDescription("Create an Internet Gateway and attach it to a VPC"),
			mcp.WithString("vpcId", mcp.Description("VPC ID to attach the Internet Gateway"), mcp.Required()),
			mcp.WithString("name", mcp.Description("Name for the Internet Gateway")),
			mcp.WithObject("tags", mcp.Description("Additional tags for the Internet Gateway")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.toolHandler.CallTool(ctx, "create-internet-gateway", arguments)
		},
	)

	// Register Private Subnet creation tool (SRP approach)
	s.mcpServer.AddTool(
		mcp.NewTool("create-private-subnet",
			mcp.WithDescription("Create a private subnet (aws_subnet with private config)"),
			mcp.WithString("vpcId", mcp.Description("VPC ID where the subnet will be created"), mcp.Required()),
			mcp.WithString("cidrBlock", mcp.Description("CIDR block for the subnet"), mcp.Required()),
			mcp.WithString("availabilityZone", mcp.Description("Availability zone for the subnet"), mcp.Required()),
			mcp.WithString("name", mcp.Description("Name for the subnet")),
			mcp.WithObject("tags", mcp.Description("Additional tags for the subnet")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.toolHandler.CallTool(ctx, "create-private-subnet", arguments)
		},
	)

	// Register Public Subnet creation tool (SRP approach)
	s.mcpServer.AddTool(
		mcp.NewTool("create-public-subnet",
			mcp.WithDescription("Create a public subnet (aws_subnet with public config)"),
			mcp.WithString("vpcId", mcp.Description("VPC ID where the subnet will be created"), mcp.Required()),
			mcp.WithString("cidrBlock", mcp.Description("CIDR block for the subnet"), mcp.Required()),
			mcp.WithString("availabilityZone", mcp.Description("Availability zone for the subnet"), mcp.Required()),
			mcp.WithString("name", mcp.Description("Name for the subnet")),
			mcp.WithObject("tags", mcp.Description("Additional tags for the subnet")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.toolHandler.CallTool(ctx, "create-public-subnet", arguments)
		},
	)

	// Register NAT Gateway creation tool (SRP approach)
	s.mcpServer.AddTool(
		mcp.NewTool("create-nat-gateway",
			mcp.WithDescription("Create a NAT Gateway with EIP (aws_eip + aws_nat_gateway)"),
			mcp.WithString("subnetId", mcp.Description("Public subnet ID where the NAT Gateway will be created"), mcp.Required()),
			mcp.WithString("name", mcp.Description("Name for the NAT Gateway")),
			mcp.WithObject("tags", mcp.Description("Additional tags for the NAT Gateway")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.toolHandler.CallTool(ctx, "create-nat-gateway", arguments)
		},
	)

	// Register Public Route Table creation tool (SRP approach)
	s.mcpServer.AddTool(
		mcp.NewTool("create-public-route-table",
			mcp.WithDescription("Create a public route table (aws_route_table for public)"),
			mcp.WithString("vpcId", mcp.Description("VPC ID where the route table will be created"), mcp.Required()),
			mcp.WithString("internetGatewayId", mcp.Description("Internet Gateway ID for public route"), mcp.Required()),
			mcp.WithString("name", mcp.Description("Name for the route table")),
			mcp.WithObject("tags", mcp.Description("Additional tags for the route table")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.toolHandler.CallTool(ctx, "create-public-route-table", arguments)
		},
	)

	// Register Private Route Table creation tool (SRP approach)
	s.mcpServer.AddTool(
		mcp.NewTool("create-private-route-table",
			mcp.WithDescription("Create a private route table (aws_route_table for private)"),
			mcp.WithString("vpcId", mcp.Description("VPC ID where the route table will be created"), mcp.Required()),
			mcp.WithString("natGatewayId", mcp.Description("NAT Gateway ID for private route"), mcp.Required()),
			mcp.WithString("name", mcp.Description("Name for the route table")),
			mcp.WithObject("tags", mcp.Description("Additional tags for the route table")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.toolHandler.CallTool(ctx, "create-private-route-table", arguments)
		},
	)

	// Register Route Table Association tool (SRP approach)
	s.mcpServer.AddTool(
		mcp.NewTool("associate-route-table",
			mcp.WithDescription("Associate route table with subnet (aws_route_table_association)"),
			mcp.WithString("routeTableId", mcp.Description("Route table ID to associate"), mcp.Required()),
			mcp.WithString("subnetId", mcp.Description("Subnet ID to associate with"), mcp.Required()),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.toolHandler.CallTool(ctx, "associate-route-table", arguments)
		},
	)

	// Register Security Group tools (SRP approach)
	// Register list security groups tool
	s.mcpServer.AddTool(
		mcp.NewTool("list-security-groups",
			mcp.WithDescription("List all security groups in the region or VPC"),
			mcp.WithString("vpcId", mcp.Description("Filter security groups by VPC ID (optional)")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				arguments = map[string]interface{}{}
			}
			return s.toolHandler.CallTool(ctx, "list-security-groups", arguments)
		},
	)

	// Register create security group tool
	s.mcpServer.AddTool(
		mcp.NewTool("create-security-group",
			mcp.WithDescription("Create a new security group (aws_security_group equivalent)"),
			mcp.WithString("groupName", mcp.Description("Name for the security group"), mcp.Required()),
			mcp.WithString("description", mcp.Description("Description for the security group"), mcp.Required()),
			mcp.WithString("vpcId", mcp.Description("VPC ID where the security group will be created")),
			mcp.WithObject("tags", mcp.Description("Additional tags for the security group")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.toolHandler.CallTool(ctx, "create-security-group", arguments)
		},
	)

	// Register add security group ingress rule tool
	s.mcpServer.AddTool(
		mcp.NewTool("add-security-group-ingress-rule",
			mcp.WithDescription("Add an ingress rule to security group (aws_security_group_rule)"),
			mcp.WithString("groupId", mcp.Description("Security group ID"), mcp.Required()),
			mcp.WithString("protocol", mcp.Description("Protocol: tcp, udp, icmp, or -1 for all"), mcp.Required()),
			mcp.WithNumber("fromPort", mcp.Description("Start port (required for TCP/UDP)")),
			mcp.WithNumber("toPort", mcp.Description("End port (required for TCP/UDP)")),
			mcp.WithArray("cidrBlocks", mcp.Description("CIDR blocks to allow"), mcp.WithStringItems()),
			mcp.WithString("sourceSG", mcp.Description("Source security group ID")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.toolHandler.CallTool(ctx, "add-security-group-ingress-rule", arguments)
		},
	)

	// Register add security group egress rule tool
	s.mcpServer.AddTool(
		mcp.NewTool("add-security-group-egress-rule",
			mcp.WithDescription("Add an egress rule to security group (aws_security_group_rule)"),
			mcp.WithString("groupId", mcp.Description("Security group ID"), mcp.Required()),
			mcp.WithString("protocol", mcp.Description("Protocol: tcp, udp, icmp, or -1 for all"), mcp.Required()),
			mcp.WithNumber("fromPort", mcp.Description("Start port (required for TCP/UDP)")),
			mcp.WithNumber("toPort", mcp.Description("End port (required for TCP/UDP)")),
			mcp.WithArray("cidrBlocks", mcp.Description("CIDR blocks to allow"), mcp.WithStringItems()),
			mcp.WithString("sourceSG", mcp.Description("Source security group ID")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.toolHandler.CallTool(ctx, "add-security-group-egress-rule", arguments)
		},
	)

	// Register delete security group tool
	s.mcpServer.AddTool(
		mcp.NewTool("delete-security-group",
			mcp.WithDescription("Delete a security group"),
			mcp.WithString("groupId", mcp.Description("Security group ID to delete"), mcp.Required()),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.toolHandler.CallTool(ctx, "delete-security-group", arguments)
		},
	)

	// Register Launch Template creation tool
	s.mcpServer.AddTool(
		mcp.NewTool("create-launch-template",
			mcp.WithDescription("Create a launch template for Auto Scaling Groups"),
			mcp.WithString("launchTemplateName", mcp.Description("Name for the launch template"), mcp.Required()),
			mcp.WithString("imageId", mcp.Description("AMI ID to use in the template"), mcp.Required()),
			mcp.WithString("instanceType", mcp.Description("EC2 instance type"), mcp.Required()),
			mcp.WithString("keyName", mcp.Description("Name of the key pair for SSH access")),
			mcp.WithString("userData", mcp.Description("User data script for instance initialization")),
			mcp.WithString("iamInstanceProfile", mcp.Description("IAM instance profile for the instances")),
			mcp.WithArray("securityGroupIds", mcp.Description("Security group IDs to assign"), mcp.WithStringItems()),
			mcp.WithObject("tags", mcp.Description("Additional tags for the launch template")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.toolHandler.CallTool(ctx, "create-launch-template", arguments)
		},
	)

	// Register Auto Scaling Group creation tool
	s.mcpServer.AddTool(
		mcp.NewTool("create-auto-scaling-group",
			mcp.WithDescription("Create an Auto Scaling Group"),
			mcp.WithString("autoScalingGroupName", mcp.Description("Name for the Auto Scaling Group"), mcp.Required()),
			mcp.WithString("launchTemplateName", mcp.Description("Name of the launch template to use"), mcp.Required()),
			mcp.WithNumber("minSize", mcp.Description("Minimum number of instances")),
			mcp.WithNumber("maxSize", mcp.Description("Maximum number of instances")),
			mcp.WithNumber("desiredCapacity", mcp.Description("Desired number of instances")),
			mcp.WithArray("vpcZoneIdentifiers", mcp.Description("Subnet IDs where instances will be launched"), mcp.WithStringItems()),
			mcp.WithArray("targetGroupARNs", mcp.Description("Load balancer target group ARNs"), mcp.WithStringItems()),
			mcp.WithString("healthCheckType", mcp.Description("Health check type: EC2 or ELB")),
			mcp.WithNumber("healthCheckGracePeriod", mcp.Description("Health check grace period in seconds")),
			mcp.WithObject("tags", mcp.Description("Additional tags for the Auto Scaling Group")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.toolHandler.CallTool(ctx, "create-auto-scaling-group", arguments)
		},
	)

	// Register Application Load Balancer creation tool
	s.mcpServer.AddTool(
		mcp.NewTool("create-load-balancer",
			mcp.WithDescription("Create an Application Load Balancer"),
			mcp.WithString("name", mcp.Description("Name for the load balancer"), mcp.Required()),
			mcp.WithArray("subnetIds", mcp.Description("Subnet IDs for the load balancer"), mcp.Required(), mcp.WithStringItems()),
			mcp.WithArray("securityGroupIds", mcp.Description("Security group IDs for the load balancer"), mcp.WithStringItems()),
			mcp.WithString("scheme", mcp.Description("Load balancer scheme: internet-facing or internal")),
			mcp.WithString("ipAddressType", mcp.Description("IP address type: ipv4 or dualstack")),
			mcp.WithObject("tags", mcp.Description("Additional tags for the load balancer")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.toolHandler.CallTool(ctx, "create-load-balancer", arguments)
		},
	)

	// Register Target Group creation tool
	s.mcpServer.AddTool(
		mcp.NewTool("create-target-group",
			mcp.WithDescription("Create a target group for load balancer"),
			mcp.WithString("name", mcp.Description("Name for the target group"), mcp.Required()),
			mcp.WithString("vpcId", mcp.Description("VPC ID for the target group"), mcp.Required()),
			mcp.WithString("protocol", mcp.Description("Protocol for the target group (HTTP, HTTPS)")),
			mcp.WithNumber("port", mcp.Description("Port for the target group")),
			mcp.WithString("targetType", mcp.Description("Target type: instance, ip, or lambda")),
			mcp.WithString("healthCheckPath", mcp.Description("Health check path")),
			mcp.WithString("healthCheckProtocol", mcp.Description("Health check protocol")),
			mcp.WithNumber("healthCheckPort", mcp.Description("Health check port")),
			mcp.WithNumber("healthCheckIntervalSeconds", mcp.Description("Health check interval")),
			mcp.WithNumber("healthCheckTimeoutSeconds", mcp.Description("Health check timeout")),
			mcp.WithNumber("healthyThresholdCount", mcp.Description("Healthy threshold count")),
			mcp.WithNumber("unhealthyThresholdCount", mcp.Description("Unhealthy threshold count")),
			mcp.WithObject("tags", mcp.Description("Additional tags for the target group")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.toolHandler.CallTool(ctx, "create-target-group", arguments)
		},
	)

	// Register Listener creation tool
	s.mcpServer.AddTool(
		mcp.NewTool("create-listener",
			mcp.WithDescription("Create a listener for load balancer"),
			mcp.WithString("loadBalancerArn", mcp.Description("Load balancer ARN"), mcp.Required()),
			mcp.WithString("protocol", mcp.Description("Listener protocol (HTTP, HTTPS)"), mcp.Required()),
			mcp.WithNumber("port", mcp.Description("Listener port"), mcp.Required()),
			mcp.WithString("targetGroupArn", mcp.Description("Target group ARN for default action"), mcp.Required()),
			mcp.WithString("certificateArn", mcp.Description("SSL certificate ARN for HTTPS listeners")),
			mcp.WithObject("tags", mcp.Description("Additional tags for the listener")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.toolHandler.CallTool(ctx, "create-listener", arguments)
		},
	)

	// Register list tools for resources
	// These tools allow LLMs to query and list AWS resources via tool calls

	// Register list EC2 instances tool
	s.mcpServer.AddTool(
		mcp.NewTool("list-ec2-instances",
			mcp.WithDescription("List all EC2 instances in the region"),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.toolHandler.CallTool(ctx, "list-ec2-instances", map[string]interface{}{})
		},
	)

	// Register list VPCs tool
	s.mcpServer.AddTool(
		mcp.NewTool("list-vpcs",
			mcp.WithDescription("List all VPCs in the region"),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.toolHandler.CallTool(ctx, "list-vpcs", map[string]interface{}{})
		},
	)

	// Register list subnets tool
	s.mcpServer.AddTool(
		mcp.NewTool("list-subnets",
			mcp.WithDescription("List all subnets in the region"),
			mcp.WithString("vpcId", mcp.Description("Filter subnets by VPC ID (optional)")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				arguments = map[string]interface{}{}
			}
			return s.toolHandler.CallTool(ctx, "list-subnets", arguments)
		},
	)

	// Register list Auto Scaling Groups tool
	s.mcpServer.AddTool(
		mcp.NewTool("list-auto-scaling-groups",
			mcp.WithDescription("List all Auto Scaling Groups in the region"),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.toolHandler.CallTool(ctx, "list-auto-scaling-groups", map[string]interface{}{})
		},
	)

	// Register list Load Balancers tool
	s.mcpServer.AddTool(
		mcp.NewTool("list-load-balancers",
			mcp.WithDescription("List all Application Load Balancers in the region"),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.toolHandler.CallTool(ctx, "list-load-balancers", map[string]interface{}{})
		},
	)

	// Register list Target Groups tool
	s.mcpServer.AddTool(
		mcp.NewTool("list-target-groups",
			mcp.WithDescription("List all Target Groups in the region"),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.toolHandler.CallTool(ctx, "list-target-groups", map[string]interface{}{})
		},
	)

	// Register list Launch Templates tool
	s.mcpServer.AddTool(
		mcp.NewTool("list-launch-templates",
			mcp.WithDescription("List all Launch Templates in the region"),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.toolHandler.CallTool(ctx, "list-launch-templates", map[string]interface{}{})
		},
	)

	// Register list AMIs tool
	s.mcpServer.AddTool(
		mcp.NewTool("list-amis",
			mcp.WithDescription("List all AMIs owned by the account in the region"),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.toolHandler.CallTool(ctx, "list-amis", map[string]interface{}{})
		},
	)

	// Register AMI creation tool
	s.mcpServer.AddTool(
		mcp.NewTool("create-ami-from-instance",
			mcp.WithDescription("Create an AMI (Amazon Machine Image) from an EC2 instance"),
			mcp.WithString("instanceId", mcp.Description("EC2 instance ID to create AMI from"), mcp.Required()),
			mcp.WithString("name", mcp.Description("Name for the AMI"), mcp.Required()),
			mcp.WithString("description", mcp.Description("Description for the AMI")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.toolHandler.CallTool(ctx, "create-ami-from-instance", arguments)
		},
	)

	// ========== RDS Database Tools ==========

	// Register create DB subnet group tool
	s.mcpServer.AddTool(
		mcp.NewTool("create-db-subnet-group",
			mcp.WithDescription("Create a database subnet group"),
			mcp.WithString("dbSubnetGroupName", mcp.Description("Name for the DB subnet group"), mcp.Required()),
			mcp.WithString("dbSubnetGroupDescription", mcp.Description("Description for the DB subnet group"), mcp.Required()),
			mcp.WithArray("subnetIds", mcp.Description("Array of subnet IDs for the DB subnet group"), mcp.Required()),
			mcp.WithObject("tags", mcp.Description("Tags to apply to the DB subnet group")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.toolHandler.CallTool(ctx, "create-db-subnet-group", arguments)
		},
	)

	// Register create DB instance tool
	s.mcpServer.AddTool(
		mcp.NewTool("create-db-instance",
			mcp.WithDescription("Create a new RDS database instance"),
			mcp.WithString("dbInstanceIdentifier", mcp.Description("Unique identifier for the DB instance"), mcp.Required()),
			mcp.WithString("dbInstanceClass", mcp.Description("DB instance class (e.g., db.t3.micro, db.t3.small)"), mcp.Required()),
			mcp.WithString("engine", mcp.Description("Database engine (mysql, postgres, oracle-ee, etc.)"), mcp.Required()),
			mcp.WithString("masterUsername", mcp.Description("Master username for the database"), mcp.Required()),
			mcp.WithString("masterUserPassword", mcp.Description("Master password for the database"), mcp.Required()),
			mcp.WithNumber("allocatedStorage", mcp.Description("Allocated storage in GB"), mcp.Required()),
			mcp.WithString("engineVersion", mcp.Description("Database engine version")),
			mcp.WithString("storageType", mcp.Description("Storage type (gp2, gp3, io1, etc.)")),
			mcp.WithBoolean("storageEncrypted", mcp.Description("Whether to encrypt storage")),
			mcp.WithString("dbSubnetGroupName", mcp.Description("DB subnet group name")),
			mcp.WithArray("vpcSecurityGroupIds", mcp.Description("VPC security group IDs")),
			mcp.WithNumber("backupRetentionPeriod", mcp.Description("Backup retention period in days")),
			mcp.WithString("preferredBackupWindow", mcp.Description("Preferred backup window")),
			mcp.WithString("preferredMaintenanceWindow", mcp.Description("Preferred maintenance window")),
			mcp.WithBoolean("multiAZ", mcp.Description("Enable Multi-AZ deployment")),
			mcp.WithBoolean("publiclyAccessible", mcp.Description("Whether instance is publicly accessible")),
			mcp.WithObject("tags", mcp.Description("Tags to apply to the DB instance")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.toolHandler.CallTool(ctx, "create-db-instance", arguments)
		},
	)

	// Register start DB instance tool
	s.mcpServer.AddTool(
		mcp.NewTool("start-db-instance",
			mcp.WithDescription("Start a stopped RDS database instance"),
			mcp.WithString("dbInstanceIdentifier", mcp.Description("DB instance identifier to start"), mcp.Required()),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.toolHandler.CallTool(ctx, "start-db-instance", arguments)
		},
	)

	// Register stop DB instance tool
	s.mcpServer.AddTool(
		mcp.NewTool("stop-db-instance",
			mcp.WithDescription("Stop a running RDS database instance"),
			mcp.WithString("dbInstanceIdentifier", mcp.Description("DB instance identifier to stop"), mcp.Required()),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.toolHandler.CallTool(ctx, "stop-db-instance", arguments)
		},
	)

	// Register delete DB instance tool
	s.mcpServer.AddTool(
		mcp.NewTool("delete-db-instance",
			mcp.WithDescription("Delete an RDS database instance"),
			mcp.WithString("dbInstanceIdentifier", mcp.Description("DB instance identifier to delete"), mcp.Required()),
			mcp.WithBoolean("skipFinalSnapshot", mcp.Description("Skip final snapshot before deletion (default: true)")),
			mcp.WithString("finalDBSnapshotIdentifier", mcp.Description("Final snapshot identifier (required if skipFinalSnapshot is false)")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.toolHandler.CallTool(ctx, "delete-db-instance", arguments)
		},
	)

	// Register create DB snapshot tool
	s.mcpServer.AddTool(
		mcp.NewTool("create-db-snapshot",
			mcp.WithDescription("Create a snapshot of an RDS database instance"),
			mcp.WithString("dbInstanceIdentifier", mcp.Description("DB instance identifier to snapshot"), mcp.Required()),
			mcp.WithString("dbSnapshotIdentifier", mcp.Description("Unique identifier for the snapshot"), mcp.Required()),
			mcp.WithObject("tags", mcp.Description("Tags to apply to the snapshot")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.toolHandler.CallTool(ctx, "create-db-snapshot", arguments)
		},
	)

	// Register list DB instances tool
	s.mcpServer.AddTool(
		mcp.NewTool("list-db-instances",
			mcp.WithDescription("List all RDS database instances"),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.toolHandler.CallTool(ctx, "list-db-instances", nil)
		},
	)

	// Register list DB snapshots tool
	s.mcpServer.AddTool(
		mcp.NewTool("list-db-snapshots",
			mcp.WithDescription("List all RDS database snapshots"),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.toolHandler.CallTool(ctx, "list-db-snapshots", nil)
		},
	)
}

// Start begins the stdio message loop for the MCP server
func (s *Server) Start(ctx context.Context) error {
	s.Logger.Info("Starting MCP server message loop on stdio...")
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			s.Logger.Info("Shutdown signal received, stopping server")
			return ctx.Err()
		default:
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			// Handle the JSON-RPC message
			response := s.mcpServer.HandleMessage(ctx, line)

			// Write response to stdout
			if response != nil {
				responseBytes, err := json.Marshal(response)
				if err != nil {
					s.Logger.WithError(err).Error("Failed to marshal response")
					continue
				}

				os.Stdout.Write(responseBytes)
				os.Stdout.Write([]byte("\n"))
			}
		}
	}

	if err := scanner.Err(); err != nil {
		s.Logger.WithError(err).Error("Error reading from stdin")
		return err
	}

	return nil
}
