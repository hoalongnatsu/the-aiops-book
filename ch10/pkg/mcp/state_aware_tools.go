package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aws-mcp-server/pkg/agent"
	"aws-mcp-server/pkg/types"

	"github.com/mark3labs/mcp-go/mcp"
)

// registerStateAwareTools registers state-aware infrastructure tools
func (s *Server) registerStateAwareTools() {
	// State management tools
	s.mcpServer.AddTool(
		mcp.NewTool("analyze-infrastructure-state",
			mcp.WithDescription("Analyze current infrastructure state and detect drift"),
			mcp.WithBoolean("scan_live", mcp.Description("Whether to scan live infrastructure")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.handleAnalyzeInfrastructureState(ctx, arguments)
		},
	)

	// Dependency graph tools
	s.mcpServer.AddTool(
		mcp.NewTool("visualize-dependency-graph",
			mcp.WithDescription("Generate dependency graph visualization"),
			mcp.WithString("format", mcp.Description("Output format: text, mermaid")),
			mcp.WithBoolean("include_bottlenecks", mcp.Description("Include bottleneck analysis")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.handleVisualizeDependencyGraph(ctx, arguments)
		},
	)

	// Conflict resolution tools
	s.mcpServer.AddTool(
		mcp.NewTool("detect-infrastructure-conflicts",
			mcp.WithDescription("Detect conflicts in infrastructure configuration"),
			mcp.WithBoolean("auto_resolve", mcp.Description("Automatically resolve conflicts where possible")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.handleDetectInfrastructureConflicts(ctx, arguments)
		},
	)

	// AI agent tools
	s.mcpServer.AddTool(
		mcp.NewTool("process-infrastructure-request",
			mcp.WithDescription("Process natural language infrastructure request using AI agent"),
			mcp.WithString("request", mcp.Description("Natural language infrastructure request"), mcp.Required()),
			mcp.WithBoolean("dry_run", mcp.Description("Execute in dry run mode")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.handleProcessInfrastructureRequest(ctx, arguments)
		},
	)

	// State export/import tools
	s.mcpServer.AddTool(
		mcp.NewTool("export-infrastructure-state",
			mcp.WithDescription("Export current infrastructure state to JSON"),
			mcp.WithBoolean("include_discovered", mcp.Description("Include discovered (unmanaged) resources")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.handleExportInfrastructureState(ctx, arguments)
		},
	)

	// State persistence tools
	s.mcpServer.AddTool(
		mcp.NewTool("save-state",
			mcp.WithDescription("Save current infrastructure state to persistent storage"),
			mcp.WithBoolean("force", mcp.Description("Force save even if state hasn't changed")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.handleSaveState(ctx, arguments)
		},
	)

	s.mcpServer.AddTool(
		mcp.NewTool("add-resource-to-state",
			mcp.WithDescription("Add a resource to the managed infrastructure state"),
			mcp.WithString("resource_id", mcp.Description("Resource ID"), mcp.Required()),
			mcp.WithString("resource_name", mcp.Description("Resource name"), mcp.Required()),
			mcp.WithString("resource_type", mcp.Description("Resource type"), mcp.Required()),
			mcp.WithString("status", mcp.Description("Resource status")),
			mcp.WithObject("properties", mcp.Description("Resource properties")),
			mcp.WithArray("dependencies", mcp.Description("Resource dependencies")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.handleAddResourceToState(ctx, arguments)
		},
	)

	// Deployment planning tools
	s.mcpServer.AddTool(
		mcp.NewTool("plan-infrastructure-deployment",
			mcp.WithDescription("Generate deployment plan with dependency ordering"),
			mcp.WithBoolean("include_levels", mcp.Description("Include deployment levels for parallel execution")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments, ok := request.Params.Arguments.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid arguments format")
			}
			return s.handlePlanInfrastructureDeployment(ctx, arguments)
		},
	)
}

// Tool handlers

func (s *Server) handleAnalyzeInfrastructureState(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// Load state from file on every API call
	if err := s.StateManager.LoadState(ctx); err != nil {
		s.Logger.WithError(err).Warn("Failed to load state from file, continuing with current state")
	}

	scanLive := true
	if val, ok := arguments["scan_live"].(bool); ok {
		scanLive = val
	}

	s.Logger.WithField("scan_live", scanLive).Info("Analyzing infrastructure state")

	result := map[string]interface{}{
		"managed_resources": s.StateManager.GetState().Resources,
	}

	if scanLive {
		// Discover live infrastructure
		discoveredResources, err := s.DiscoveryScanner.DiscoverInfrastructure(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to discover infrastructure: %w", err)
		}

		result["discovered_resources"] = discoveredResources

		// Detect drift
		var driftDetections []*types.ChangeDetection
		for _, resource := range discoveredResources {
			if _, exists := s.StateManager.GetResource(resource.ID); exists {
				drift, err := s.StateManager.DetectDrift(ctx, resource.Properties, resource.ID)
				if err != nil {
					s.Logger.WithError(err).WithField("resource_id", resource.ID).Warn("Failed to detect drift")
					continue
				}
				if drift != nil {
					driftDetections = append(driftDetections, drift)
				}
			}
		}

		result["drift_detections"] = driftDetections
	}

	// Marshal the structured data as JSON for the agent to parse
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal analysis result: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(jsonData),
			},
		},
	}, nil
}

func (s *Server) handleVisualizeDependencyGraph(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	format := "text"
	if val, ok := arguments["format"].(string); ok {
		format = val
	}

	includeBottlenecks := true
	if val, ok := arguments["include_bottlenecks"].(bool); ok {
		includeBottlenecks = val
	}

	s.Logger.WithFields(map[string]interface{}{
		"format":              format,
		"include_bottlenecks": includeBottlenecks,
	}).Info("Visualizing dependency graph")

	// Discover infrastructure to build graph
	discoveredResources, err := s.DiscoveryScanner.DiscoverInfrastructure(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to discover infrastructure: %w", err)
	}

	// Build dependency graph
	if err := s.GraphManager.BuildGraph(ctx, discoveredResources); err != nil {
		return nil, fmt.Errorf("failed to build dependency graph: %w", err)
	}

	analyzer := s.GraphAnalyzer
	var visualization string

	switch format {
	case "mermaid":
		visualization = analyzer.GenerateMermaidDiagram()
	default:
		visualization = analyzer.GenerateTextualRepresentation()
	}

	result := map[string]interface{}{
		"visualization": visualization,
		"format":        format,
	}

	if includeBottlenecks {
		bottlenecks := analyzer.FindBottlenecks()
		result["bottlenecks"] = bottlenecks
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: visualization,
			},
		},
	}, nil
}

func (s *Server) handleDetectInfrastructureConflicts(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	autoResolve := false
	if val, ok := arguments["auto_resolve"].(bool); ok {
		autoResolve = val
	}

	s.Logger.WithField("auto_resolve", autoResolve).Info("Detecting infrastructure conflicts")

	// Discover infrastructure
	discoveredResources, err := s.DiscoveryScanner.DiscoverInfrastructure(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to discover infrastructure: %w", err)
	}

	// Detect conflicts
	conflicts, err := s.ConflictResolver.DetectConflicts(ctx, discoveredResources)
	if err != nil {
		return nil, fmt.Errorf("failed to detect conflicts: %w", err)
	}

	result := map[string]interface{}{
		"conflicts":    conflicts,
		"auto_resolve": autoResolve,
	}

	if autoResolve && len(conflicts) > 0 {
		var resolutions []map[string]interface{}
		for _, conflict := range conflicts {
			resolution, err := s.ConflictResolver.ResolveConflict(ctx, conflict)
			if err != nil {
				s.Logger.WithError(err).WithField("resource_id", conflict.ResourceID).Warn("Failed to resolve conflict")
				continue
			}
			resolutions = append(resolutions, map[string]interface{}{
				"conflict":   conflict,
				"resolution": resolution,
			})
		}
		result["resolutions"] = resolutions
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Found %d conflicts\n%s", len(conflicts), s.formatConflicts(conflicts)),
			},
		},
	}, nil
}

func (s *Server) handleProcessInfrastructureRequest(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	request, ok := arguments["request"].(string)
	if !ok {
		return nil, fmt.Errorf("request parameter is required")
	}

	dryRun := true
	if val, ok := arguments["dry_run"].(bool); ok {
		dryRun = val
	}

	s.Logger.WithFields(map[string]interface{}{
		"request": request,
		"dry_run": dryRun,
	}).Info("Processing infrastructure request with AI agent")

	// Check if we have valid API keys for any provider
	var hasValidProvider bool
	var provider string

	if s.Config.Agent.OpenAIAPIKey != "" {
		hasValidProvider = true
		provider = "openai"
	} else if s.Config.Agent.GeminiAPIKey != "" {
		hasValidProvider = true
		provider = "gemini"
	} else if s.Config.Agent.AnthropicAPIKey != "" {
		hasValidProvider = true
		provider = "anthropic"
	}

	if !hasValidProvider {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("AI Agent Processing: %s\nDry Run: %t\nStatus: No valid API key found for any provider (OpenAI, Gemini, Anthropic)", request, dryRun),
				},
			},
		}, nil
	}

	// Create AI agent instance
	aiAgent, err := agent.NewStateAwareAgent(
		&s.Config.Agent,
		s.AWSClient,
		s.Config.State.FilePath,
		s.Config.AWS.Region,
		s.Logger,
		&s.Config.AWS,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create AI agent: %w", err)
	}
	defer aiAgent.Cleanup()

	// Initialize the agent
	if err := aiAgent.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize AI agent: %w", err)
	}

	// Process the infrastructure request
	decision, err := aiAgent.ProcessRequest(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to process request: %w", err)
	}

	// Execute the plan if not in dry run mode
	var execution *types.PlanExecution
	if !dryRun && decision.Action != "no_action" {
		progressChan := make(chan *types.ExecutionUpdate, 100)
		go func() {
			// Consume progress updates to prevent blocking
			for range progressChan {
				// Progress updates are logged by the agent
			}
		}()

		execution, err = aiAgent.ExecuteConfirmedPlanWithDryRun(ctx, decision, progressChan, false)
		close(progressChan)

		if err != nil {
			return nil, fmt.Errorf("failed to execute plan: %w", err)
		}
	}

	// Format response
	result := map[string]interface{}{
		"request":    request,
		"provider":   provider,
		"dry_run":    dryRun,
		"decision":   decision,
		"plan_steps": len(decision.ExecutionPlan),
	}

	if execution != nil {
		result["execution"] = execution
		result["execution_status"] = execution.Status
		result["total_steps"] = len(execution.Steps)

		// Count completed steps
		completedSteps := 0
		for _, step := range execution.Steps {
			if step.Status == "completed" {
				completedSteps++
			}
		}
		result["completed_steps"] = completedSteps
	}

	responseText := fmt.Sprintf("AI Agent Processing Complete\n"+
		"Provider: %s\n"+
		"Request: %s\n"+
		"Action: %s\n"+
		"Plan Steps: %d\n"+
		"Dry Run: %t\n",
		provider, request, decision.Action, len(decision.ExecutionPlan), dryRun)

	if execution != nil {
		completedSteps := 0
		for _, step := range execution.Steps {
			if step.Status == "completed" {
				completedSteps++
			}
		}
		responseText += fmt.Sprintf("Execution Status: %s\n"+
			"Completed Steps: %d/%d\n",
			execution.Status, completedSteps, len(execution.Steps))
	}

	if decision.Reasoning != "" {
		responseText += fmt.Sprintf("Reasoning: %s\n", decision.Reasoning)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: responseText,
			},
		},
	}, nil
}

func (s *Server) handleExportInfrastructureState(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// Load state from file on every API call
	if err := s.StateManager.LoadState(ctx); err != nil {
		s.Logger.WithError(err).Warn("Failed to load state from file, continuing with current state")
	}

	includeDiscovered := false
	if val, ok := arguments["include_discovered"].(bool); ok {
		includeDiscovered = val
	}

	s.Logger.WithField("include_discovered", includeDiscovered).Info("Exporting infrastructure state")

	state := s.StateManager.GetState()
	result := map[string]interface{}{
		"managed_state": state,
	}

	if includeDiscovered {
		discoveredResources, err := s.DiscoveryScanner.DiscoverInfrastructure(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to discover infrastructure: %w", err)
		}
		result["discovered_resources"] = discoveredResources
	}

	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal state: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(jsonData),
			},
		},
	}, nil
}

func (s *Server) handlePlanInfrastructureDeployment(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	var targetResources []string
	if val, ok := arguments["target_resources"].([]interface{}); ok {
		for _, v := range val {
			if str, ok := v.(string); ok {
				targetResources = append(targetResources, str)
			}
		}
	}

	includeLevels := true
	if val, ok := arguments["include_levels"].(bool); ok {
		includeLevels = val
	}

	s.Logger.WithFields(map[string]interface{}{
		"target_resources": targetResources,
		"include_levels":   includeLevels,
	}).Info("Planning infrastructure deployment")

	// Discover infrastructure to build graph
	discoveredResources, err := s.DiscoveryScanner.DiscoverInfrastructure(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to discover infrastructure: %w", err)
	}

	// Filter target resources if specified
	if len(targetResources) > 0 {
		var filteredResources []*types.ResourceState
		targetMap := make(map[string]bool)
		for _, target := range targetResources {
			targetMap[target] = true
		}

		for _, resource := range discoveredResources {
			if targetMap[resource.ID] {
				filteredResources = append(filteredResources, resource)
			}
		}
		discoveredResources = filteredResources
	}

	// Build dependency graph
	if err := s.GraphManager.BuildGraph(ctx, discoveredResources); err != nil {
		return nil, fmt.Errorf("failed to build dependency graph: %w", err)
	}

	// Get deployment order
	deploymentOrder, err := s.GraphManager.GetDeploymentOrder()
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment order: %w", err)
	}

	result := map[string]interface{}{
		"deployment_order": deploymentOrder,
		"resource_count":   len(discoveredResources),
	}

	if includeLevels {
		deploymentLevels, err := s.GraphManager.CalculateDeploymentLevels()
		if err != nil {
			return nil, fmt.Errorf("failed to calculate deployment levels: %w", err)
		}
		result["deployment_levels"] = deploymentLevels
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Deployment Plan:\n%s", s.formatDeploymentPlan(result)),
			},
		},
	}, nil
}

func (s *Server) formatConflicts(conflicts []*types.ConflictResolution) string {
	var output strings.Builder

	for _, conflict := range conflicts {
		output.WriteString(fmt.Sprintf("- %s: %s (Resource: %s)\n",
			conflict.ConflictType, conflict.Details, conflict.ResourceID))
	}

	return output.String()
}

func (s *Server) formatDeploymentPlan(result map[string]interface{}) string {
	var output strings.Builder

	if deploymentOrder, ok := result["deployment_order"].([]string); ok {
		output.WriteString("Deployment Order:\n")
		for i, resourceID := range deploymentOrder {
			output.WriteString(fmt.Sprintf("%d. %s\n", i+1, resourceID))
		}
	}

	if deploymentLevels, ok := result["deployment_levels"].([][]string); ok {
		output.WriteString("\nDeployment Levels (for parallel execution):\n")
		for i, level := range deploymentLevels {
			output.WriteString(fmt.Sprintf("Level %d: %s\n", i+1, strings.Join(level, ", ")))
		}
	}

	return output.String()
}

// handleSaveState saves the current infrastructure state to persistent storage
func (s *Server) handleSaveState(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// Load state from file on every API call
	if err := s.StateManager.LoadState(ctx); err != nil {
		s.Logger.WithError(err).Warn("Failed to load state from file, continuing with current state")
	}

	force := false
	if val, ok := arguments["force"].(bool); ok {
		force = val
	}

	s.Logger.WithField("force", force).Info("Saving infrastructure state")

	// Save state using the state manager
	if err := s.StateManager.SaveState(ctx); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf(`{"success": false, "error": "failed to save state: %v"}`, err),
				},
			},
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Type: "text",
				Text: `{"success": true, "message": "Infrastructure state saved successfully"}`,
			},
		},
	}, nil
}

// handleAddResourceToState adds a resource to the managed infrastructure state
func (s *Server) handleAddResourceToState(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// Load state from file on every API call
	if err := s.StateManager.LoadState(ctx); err != nil {
		s.Logger.WithError(err).Warn("Failed to load state from file, continuing with current state")
	}

	resourceID, ok := arguments["resource_id"].(string)
	if !ok || resourceID == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: `{"success": false, "error": "resource_id is required"}`,
				},
			},
		}, nil
	}

	resourceName, ok := arguments["resource_name"].(string)
	if !ok || resourceName == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: `{"success": false, "error": "resource_name is required"}`,
				},
			},
		}, nil
	}

	resourceType, ok := arguments["resource_type"].(string)
	if !ok || resourceType == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: `{"success": false, "error": "resource_type is required"}`,
				},
			},
		}, nil
	}

	status := "created"
	if val, ok := arguments["status"].(string); ok {
		status = val
	}

	var properties map[string]interface{}
	if val, ok := arguments["properties"].(map[string]interface{}); ok {
		properties = val
	} else {
		properties = make(map[string]interface{})
	}

	var dependencies []string
	if val, ok := arguments["dependencies"].([]interface{}); ok {
		for _, dep := range val {
			if depStr, ok := dep.(string); ok {
				dependencies = append(dependencies, depStr)
			}
		}
	}

	s.Logger.WithFields(map[string]interface{}{
		"resource_id":   resourceID,
		"resource_name": resourceName,
		"resource_type": resourceType,
		"status":        status,
	}).Info("Adding resource to managed state")

	// Create resource state
	resourceState := &types.ResourceState{
		ID:           resourceID,
		Name:         resourceName,
		Type:         resourceType,
		Status:       status,
		Properties:   properties,
		Dependencies: dependencies,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Add to state manager
	if err := s.StateManager.AddResource(ctx, resourceState); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf(`{"success": false, "error": "failed to add resource to state: %v"}`, err),
				},
			},
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf(`{"success": true, "message": "Resource %s added to managed state", "resource_id": "%s"}`, resourceID, resourceID),
			},
		},
	}, nil
}
