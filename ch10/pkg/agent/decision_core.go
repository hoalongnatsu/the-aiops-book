package agent

import (
	"context"
	"fmt"
	"strings"

	"aws-mcp-server/pkg/types"

	"github.com/google/uuid"
	"github.com/tmc/langchaingo/llms"
)

// ========== Interface defines ==========

// DecisionCoreInterface defines core decision processing functionality for infrastructure automation
//
// Available Functions:
//   - ProcessRequest()              : Process natural language infrastructure request and generate execution plan
//   - ExecuteDecision()             : Execute AI-generated infrastructure decision with step-by-step execution
//   - buildContextForDecision()     : Build comprehensive decision context with state analysis and correlation
//   - executeSteps()                : Execute individual steps in infrastructure deployment plan
//
// Usage Example:
//   1. decision, err := agent.ProcessRequest(ctx, "Create web server infrastructure")
//   2. result, err := agent.ExecuteDecision(ctx, decision)
//   3. Monitor execution progress and handle errors

// ========== Core Decision Processing Functions ==========

// ProcessRequest processes a natural language infrastructure request and generates a plan
func (a *StateAwareAgent) ProcessRequest(ctx context.Context, request string) (*types.AgentDecision, error) {
	a.Logger.WithField("request", request).Info("Processing infrastructure request")

	// Create decision ID
	decisionID := uuid.New().String()

	// Gather context
	decisionContext, err := a.gatherDecisionContext(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to gather decision context: %w", err)
	}

	// Generate AI decision with detailed execution plan
	decision, err := a.generateDecisionWithPlan(ctx, decisionID, request, decisionContext)
	if err != nil {
		return nil, fmt.Errorf("failed to generate decision: %w", err)
	}

	// Validate decision
	if err := a.validateDecision(decision, decisionContext); err != nil {
		return nil, fmt.Errorf("decision validation failed: %w", err)
	}

	a.Logger.WithFields(map[string]interface{}{
		"decision_id": decision.ID,
		"action":      decision.Action,
		"confidence":  decision.Confidence,
		"plan_steps":  len(decision.ExecutionPlan),
	}).Info("Infrastructure request processed successfully")

	return decision, nil
}

// gatherDecisionContext gathers context for decision-making
func (a *StateAwareAgent) gatherDecisionContext(ctx context.Context, request string) (*DecisionContext, error) {
	a.Logger.Debug("Gathering decision context")

	// Use MCP server to analyze infrastructure state
	currentState, discoveredResources, _, err := a.AnalyzeInfrastructureState(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze infrastructure state: %w", err)
	}

	// Use MCP server to detect conflicts
	conflicts, err := a.DetectInfrastructureConflicts(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("failed to detect conflicts: %w", err)
	}

	// Use MCP server to get deployment order
	deploymentOrder, _, err := a.PlanInfrastructureDeployment(ctx, nil, false)
	if err != nil {
		// Non-fatal error - continue without deployment order
		a.Logger.WithError(err).Warn("Failed to calculate deployment order")
		deploymentOrder = []string{}
	}

	// Analyze resource correlation for better decision making
	resourceCorrelation := a.analyzeResourceCorrelation(currentState, discoveredResources)

	return &DecisionContext{
		Request:             request,
		CurrentState:        currentState,
		DiscoveredState:     discoveredResources,
		Conflicts:           conflicts,
		DependencyGraph:     nil, // Will be handled by MCP server
		DeploymentOrder:     deploymentOrder,
		ResourceCorrelation: resourceCorrelation,
	}, nil
}

// generateDecisionWithPlan uses AI to generate a decision with detailed execution plan
func (a *StateAwareAgent) generateDecisionWithPlan(ctx context.Context, decisionID, request string, context *DecisionContext) (*types.AgentDecision, error) {
	a.Logger.Debug("Generating AI decision with execution plan")

	// Create prompt for the AI that includes plan generation
	prompt := a.buildDecisionWithPlanPrompt(request, context)

	// Log prompt details for debugging
	a.Logger.WithFields(map[string]interface{}{
		"prompt_length":  len(prompt),
		"max_tokens":     a.config.MaxTokens,
		"temperature":    a.config.Temperature,
		"provider":       a.config.Provider,
		"model":          a.config.Model,
		"prompt_preview": prompt[:min(500, len(prompt))],
	}).Info("Calling LLM with prompt")

	// Call the LLM using the new recommended method
	response, err := llms.GenerateFromSinglePrompt(ctx, a.llm, prompt,
		llms.WithTemperature(a.config.Temperature),
		llms.WithMaxTokens(a.config.MaxTokens))

	// Enhanced error handling
	if err != nil {
		a.Logger.WithError(err).WithFields(map[string]interface{}{
			"provider":      a.config.Provider,
			"model":         a.config.Model,
			"prompt_length": len(prompt),
		}).Error("LLM call failed")
		return nil, fmt.Errorf("failed to generate AI response: %w", err)
	}

	// Comprehensive response logging
	a.Logger.WithFields(map[string]interface{}{
		"response_length":  len(response),
		"response_empty":   len(response) == 0,
		"response_content": response, // Log full response for debugging
	}).Info("LLM Response received")

	// Handle empty response immediately
	if len(response) == 0 {
		a.Logger.Error("LLM returned empty response - check API key, model availability, and prompt")
		return nil, fmt.Errorf("LLM returned empty response - possible API key, model, or prompt issue")
	}

	// Log response characteristics for debugging
	a.Logger.WithFields(map[string]interface{}{
		"response_length":     len(response),
		"max_tokens_config":   a.config.MaxTokens,
		"starts_with_brace":   strings.HasPrefix(response, "{"),
		"ends_with_brace":     strings.HasSuffix(response, "}"),
		"probable_truncation": strings.HasPrefix(response, "{") && !strings.HasSuffix(response, "}"),
	}).Debug("LLM Response Analysis")

	// Check for potential token limit issues
	if len(response) > 0 && strings.HasPrefix(response, "{") && !strings.HasSuffix(response, "}") {
		a.Logger.WithFields(map[string]interface{}{
			"response_length": len(response),
			"max_tokens":      a.config.MaxTokens,
			"last_100_chars":  response[max(0, len(response)-100):],
		}).Warn("Response appears truncated - consider increasing max_tokens in config")
	}

	// Parse the AI response with execution plan
	decision, err := a.parseAIResponseWithPlan(decisionID, request, response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	return decision, nil
}

// validateDecision validates an agent decision
func (a *StateAwareAgent) validateDecision(decision *types.AgentDecision, context *DecisionContext) error {
	a.Logger.Debug("Validating agent decision")

	// Check confidence threshold
	if decision.Confidence < 0.7 {
		return fmt.Errorf("decision confidence too low: %f", decision.Confidence)
	}

	// Validate action
	validActions := map[string]bool{
		"create_infrastructure": true,
		"update_infrastructure": true,
		"delete_infrastructure": true,
		"resolve_conflicts":     true,
		"no_action":             true,
	}

	if !validActions[decision.Action] {
		return fmt.Errorf("invalid action: %s", decision.Action)
	}

	// Check for critical conflicts if auto-resolve is disabled
	if !a.config.AutoResolveConflicts && len(context.Conflicts) > 0 {
		for _, conflict := range context.Conflicts {
			if conflict.ConflictType == "dependency" {
				return fmt.Errorf("critical dependency conflict detected, manual resolution required")
			}
		}
	}

	return nil
}
