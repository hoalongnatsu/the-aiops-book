package agent

import (
	"bufio"
	"os/exec"
	"sync"

	"aws-mcp-server/internal/config"
	"aws-mcp-server/internal/logging"
	"aws-mcp-server/pkg/aws"
	"aws-mcp-server/pkg/types"

	"github.com/tmc/langchaingo/llms"
)

// ========== Interface defines ==========
// Type definitions for agent components and shared data structures
// Core structures: Agent, ToolCall, ResponseData, ToolRegistry
// Communication: MCPRequest/Response, stream processing
// Execution: Command processing, state management

// AgentTypesInterface defines all data structures and types used by the AI agent system
//
// Available Types:
//   - StateAwareAgent             : Main agent struct with LLM, config, and MCP capabilities
//   - MCPProcess                  : Running MCP server process representation
//   - MCPToolInfo                 : Information about available MCP tool capabilities
//   - MCPResourceInfo             : Information about available MCP resources
//   - DecisionContext             : Context data for agent decision-making process
//
// Key Features:
//   - MCP Server Integration      : Direct communication with Model Context Protocol servers
//   - Multi-LLM Support          : OpenAI, Google AI, and other LLM providers
//   - Resource Management        : Track mappings between plan steps and actual AWS resource IDs
//   - Thread-Safe Operations     : Mutex protection for concurrent access
//
// Usage Example:
//   agent := &StateAwareAgent{...}  // Main agent instance
//   context := &DecisionContext{...} // For decision making

// ========== Agent Type Definitions ==========

// MCPProcess represents a running MCP server process
type MCPProcess struct {
	cmd    *exec.Cmd
	stdin  *bufio.Writer
	stdout *bufio.Scanner
	mutex  sync.Mutex
	reqID  int64
}

// StateAwareAgent represents an AI agent with state management capabilities
type StateAwareAgent struct {
	llm       llms.Model
	config    *config.AgentConfig
	awsConfig *config.AWSConfig
	awsClient *aws.Client

	mcpProcess       *MCPProcess
	resourceMappings map[string]string // Maps plan step IDs to actual AWS resource IDs
	mappingsMutex    sync.RWMutex

	// MCP Server capabilities discovered at startup
	mcpTools        map[string]MCPToolInfo     // Available tools from MCP server
	mcpResources    map[string]MCPResourceInfo // Available resources from MCP server
	capabilityMutex sync.RWMutex

	Logger *logging.Logger
}

// MCPToolInfo represents information about an available MCP tool
type MCPToolInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// MCPResourceInfo represents information about an available MCP resource
type MCPResourceInfo struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType"`
}

// DecisionContext contains context for agent decision-making
type DecisionContext struct {
	Request             string                      `json:"request"`
	CurrentState        *types.InfrastructureState  `json:"current_state"`
	DiscoveredState     []*types.ResourceState      `json:"discovered_state"`
	Conflicts           []*types.ConflictResolution `json:"conflicts"`
	DependencyGraph     *types.DependencyGraph      `json:"dependency_graph"`
	DeploymentOrder     []string                    `json:"deployment_order"`
	ResourceCorrelation map[string]*ResourceMatch   `json:"resource_correlation"`
}

// ResourceMatch represents correlation between managed and discovered resources
type ResourceMatch struct {
	ManagedResource    *types.ResourceState   `json:"managed_resource"`
	DiscoveredResource *types.ResourceState   `json:"discovered_resource"`
	MatchConfidence    float64                `json:"match_confidence"`
	MatchReason        string                 `json:"match_reason"`
	Capabilities       map[string]interface{} `json:"capabilities"`
}
