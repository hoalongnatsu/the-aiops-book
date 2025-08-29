package agent

import (
	"fmt"
	"sync"

	"aws-mcp-server/internal/config"
	"aws-mcp-server/internal/logging"
	"aws-mcp-server/pkg/aws"
)

// ========== Interface defines ==========

// AgentCoreInterface defines the core agent functionality for creating and managing AI agents
//
// Available Functions:
//   - NewStateAwareAgent()        : Create a new state-aware AI agent instance
//   - initializeLLM()             : Initialize the Language Model (OpenAI, Gemini, etc.)
//   - Cleanup()                   : Clean up agent resources and connections
//
// Usage Example:
//   1. agent := NewStateAwareAgent(config, awsClient, stateFile, region, logger, awsConfig)
//   2. agent.Initialize(ctx)
//   3. defer agent.Cleanup()

// ========== Agent Core Functions ==========

// NewStateAwareAgent creates a new state-aware AI agent
func NewStateAwareAgent(agentConfig *config.AgentConfig, awsClient *aws.Client, stateFilePath, region string, logger *logging.Logger, awsConfig *config.AWSConfig) (*StateAwareAgent, error) {
	// Initialize LLM based on provider
	llm, err := initializeLLM(agentConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize LLM: %w", err)
	}

	agent := &StateAwareAgent{
		llm:              llm,
		config:           agentConfig,
		awsConfig:        awsConfig,
		awsClient:        awsClient,
		Logger:           logger,
		mcpProcess:       nil, // Will be initialized when needed
		resourceMappings: make(map[string]string),
		mappingsMutex:    sync.RWMutex{},
		mcpTools:         make(map[string]MCPToolInfo),
		mcpResources:     make(map[string]MCPResourceInfo),
		capabilityMutex:  sync.RWMutex{},
	}

	return agent, nil
}

// Cleanup ensures proper cleanup of resources
func (a *StateAwareAgent) Cleanup() {
	a.stopMCPProcess()
}
