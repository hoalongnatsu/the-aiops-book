package agent

import (
	"context"
	"fmt"
	"strings"

	"aws-mcp-server/internal/config"
	"aws-mcp-server/internal/logging"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/tmc/langchaingo/llms/openai"
)

// ========== Interface defines ==========

// InitializationInterface defines agent initialization and LLM setup functionality
//
// Available Functions:
//   - Initialize()               : Initialize the agent and load existing state via MCP
//   - testLLMConnectivity()     : Test basic connectivity to the Language Model
//   - initializeLLM()           : Initialize specific LLM provider (OpenAI, Google AI, etc.)
//
// Usage Example:
//   1. agent := NewStateAwareAgent(...)
//   2. err := agent.Initialize(ctx)
//   3. // Agent is ready to process requests

// ========== Initialization Functions ==========

// Initialize initializes the agent and loads existing state
func (a *StateAwareAgent) Initialize(ctx context.Context) error {
	a.Logger.Info("Initializing state-aware AI agent")

	// Test LLM connectivity
	if err := a.testLLMConnectivity(ctx); err != nil {
		a.Logger.WithError(err).Error("LLM connectivity test failed")
		return fmt.Errorf("LLM connectivity test failed: %w", err)
	}

	// Load existing state via MCP server
	if err := a.loadState(ctx); err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Start MCP process and discover capabilities early
	if err := a.startMCPProcess(); err != nil {
		a.Logger.WithError(err).Warn("Failed to start MCP process during initialization, will retry later")
		// Don't fail initialization if MCP process fails, it will be retried when needed
	}

	a.Logger.Info("State-aware AI agent initialized successfully")
	return nil
}

// testLLMConnectivity tests basic connectivity to the LLM
func (a *StateAwareAgent) testLLMConnectivity(ctx context.Context) error {
	a.Logger.Debug("Testing LLM connectivity")

	testPrompt := "Respond with exactly this JSON: {\"status\": \"ok\", \"message\": \"connectivity test successful\"}"

	response, err := llms.GenerateFromSinglePrompt(ctx, a.llm, testPrompt,
		llms.WithTemperature(0.1),
		llms.WithMaxTokens(100))

	if err != nil {
		return fmt.Errorf("LLM test call failed: %w", err)
	}

	if len(response) == 0 {
		return fmt.Errorf("LLM returned empty response during connectivity test")
	}

	a.Logger.WithFields(map[string]interface{}{
		"test_response_length": len(response),
		"test_response":        response,
	}).Info("LLM connectivity test successful")

	return nil
}

// initializeLLM initializes the appropriate LLM based on the provider configuration
func initializeLLM(agentConfig *config.AgentConfig, logger *logging.Logger) (llms.Model, error) {
	provider := strings.ToLower(agentConfig.Provider)

	logger.WithFields(map[string]interface{}{
		"provider": provider,
		"model":    agentConfig.Model,
	}).Info("Initializing LLM")

	switch provider {
	case "openai":
		if agentConfig.OpenAIAPIKey == "" {
			logger.Error("OpenAI API key is missing")
			return nil, fmt.Errorf("OpenAI API key is required for provider 'openai'")
		}
		logger.WithFields(map[string]interface{}{
			"api_key_length": len(agentConfig.OpenAIAPIKey),
			"api_key_prefix": agentConfig.OpenAIAPIKey[:min(8, len(agentConfig.OpenAIAPIKey))],
		}).Debug("OpenAI configuration")

		llm, err := openai.New(
			openai.WithToken(agentConfig.OpenAIAPIKey),
			openai.WithModel(agentConfig.Model),
		)
		if err != nil {
			logger.WithError(err).Error("Failed to initialize OpenAI client")
			return nil, fmt.Errorf("failed to initialize OpenAI client: %w", err)
		}
		logger.Info("OpenAI client initialized successfully")
		return llm, nil

	case "gemini", "googleai":
		if agentConfig.GeminiAPIKey == "" {
			logger.Error("Gemini API key is missing")
			return nil, fmt.Errorf("GeminiAI API key is required for provider 'gemini'")
		}
		logger.WithFields(map[string]interface{}{
			"api_key_length": len(agentConfig.GeminiAPIKey),
			"api_key_prefix": agentConfig.GeminiAPIKey[:min(8, len(agentConfig.GeminiAPIKey))],
		}).Debug("Gemini configuration")

		ctx := context.Background()
		llm, err := googleai.New(
			ctx,
			googleai.WithAPIKey(agentConfig.GeminiAPIKey),
			googleai.WithDefaultModel(agentConfig.Model),
		)
		if err != nil {
			logger.WithError(err).Error("Failed to initialize Gemini client")
			return nil, fmt.Errorf("failed to initialize Gemini client: %w", err)
		}
		logger.Info("Gemini client initialized successfully")
		return llm, nil

	case "anthropic":
		return nil, fmt.Errorf("AnthropicAI provider not yet implemented")

	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s. Supported providers: openai, gemini", provider)
	}
}
