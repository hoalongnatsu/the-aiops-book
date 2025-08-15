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

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Server struct {
	config          *config.Config
	awsClient       *aws.Client
	resourceHandler *ResourceHandler
	toolHandler     *ToolHandler
	logger          *logging.Logger
	mcpServer       *server.MCPServer
}

func NewServer(cfg *config.Config, awsClient *aws.Client, logger *logging.Logger) *Server {

	// Create MCP server
	mcpServer := server.NewMCPServer(
		cfg.MCP.ServerName,
		cfg.MCP.Version,
		server.WithResourceCapabilities(true, true),
		server.WithToolCapabilities(true),
	)

	s := &Server{
		config:          cfg,
		awsClient:       awsClient,
		resourceHandler: NewResourceHandler(awsClient),
		toolHandler:     NewToolHandler(awsClient, logger),
		logger:          logger,
		mcpServer:       mcpServer,
	}

	// Register resources
	s.registerResources()

	// Register tools
	s.registerTools()

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
			s.logger.Info("Received request for EC2 instances list")

			// Use our resource handler to get the instances
			result, err := s.resourceHandler.ReadResource(ctx, "aws://ec2/instances")
			if err != nil {
				s.logger.WithError(err).Error("Failed to read EC2 instances resource")
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
		s.logger.WithField("uri", request.Params.URI).Info("Received read resource request for specific EC2 instance")

		// The server automatically matches URIs to templates, so we can use the full URI directly
		result, err := s.resourceHandler.ReadResource(ctx, request.Params.URI)
		if err != nil {
			s.logger.WithError(err).WithField("uri", request.Params.URI).Error("Failed to read resource")
			return nil, err
		}

		return result.Contents, nil
	})
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
}

// Start begins the stdio message loop for the MCP server
func (s *Server) Start(ctx context.Context) error {
	s.logger.Info("Starting MCP server message loop on stdio...")
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			s.logger.Info("Shutdown signal received, stopping server")
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
					s.logger.WithError(err).Error("Failed to marshal response")
					continue
				}

				os.Stdout.Write(responseBytes)
				os.Stdout.Write([]byte("\n"))
			}
		}
	}

	if err := scanner.Err(); err != nil {
		s.logger.WithError(err).Error("Error reading from stdin")
		return err
	}

	return nil
}
