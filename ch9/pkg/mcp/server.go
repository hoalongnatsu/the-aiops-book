package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"os"

	"aws-mcp-server/internal/config"
	"aws-mcp-server/internal/logging"
	"aws-mcp-server/pkg/aws"

	"github.com/mark3labs/mcp-go/server"
)

type Server struct {
	mcpServer *server.MCPServer

	Config      *config.Config
	AWSClient   *aws.Client
	Logger      *logging.Logger
	ToolManager *ToolManager
}

func NewServer(cfg *config.Config, awsClient *aws.Client, logger *logging.Logger) *Server {
	// Initialize tool manager
	toolManager := NewToolManager(logger)

	// Create MCP server
	mcpServer := server.NewMCPServer(
		cfg.MCP.ServerName,
		cfg.MCP.Version,
		server.WithResourceCapabilities(true, true),
		server.WithToolCapabilities(true),
	)

	s := &Server{
		mcpServer: mcpServer,

		Config:      cfg,
		AWSClient:   awsClient,
		Logger:      logger,
		ToolManager: toolManager,
	}

	// Register resources using the new registry-based approach
	s.registerResources()

	// Register modern adapter-based tools (replaces legacy registerTools)
	s.registerServerTools()

	return s
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
