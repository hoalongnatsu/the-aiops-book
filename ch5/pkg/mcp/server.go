package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"os"

	"aws-mcp-server/internal/config"
	"aws-mcp-server/pkg/aws"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/sirupsen/logrus"
)

type Server struct {
	config          *config.Config
	awsClient       *aws.Client
	resourceHandler *ResourceHandler
	logger          *logrus.Logger
	mcpServer       *server.MCPServer
}

func NewServer(cfg *config.Config, awsClient *aws.Client, logger *logrus.Logger) *Server {

	// Create MCP server
	mcpServer := server.NewMCPServer(
		cfg.MCP.ServerName,
		cfg.MCP.Version,
		server.WithResourceCapabilities(true, true),
		// server.WithToolCapabilities(true),
	)

	s := &Server{
		config:          cfg,
		awsClient:       awsClient,
		resourceHandler: NewResourceHandler(awsClient),
		logger:          logger,
		mcpServer:       mcpServer,
	}

	// Register resources
	s.registerResources()

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
