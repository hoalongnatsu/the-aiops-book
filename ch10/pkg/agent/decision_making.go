package agent

import (
	"fmt"
	"strings"
)

// ========== Interface defines ==========

// DecisionMakingInterface defines prompt building and decision-making functionality for AI infrastructure automation
//
// Available Functions:
//   - buildDecisionWithPlanPrompt() : Build comprehensive prompt for AI decision-making with execution plan
//                                   : Includes infrastructure state analysis, resource correlation, and reuse policy
//                                   : Returns formatted prompt for LLM with JSON response schema
//
// Usage Example:
//   1. prompt := agent.buildDecisionWithPlanPrompt(userRequest, decisionContext)
//   2. Send prompt to LLM for infrastructure decision analysis
//   3. LLM returns JSON with action, reasoning, and execution plan

// buildDecisionWithPlanPrompt builds a prompt for AI decision-making with execution plan
func (a *StateAwareAgent) buildDecisionWithPlanPrompt(request string, context *DecisionContext) string {
	var prompt strings.Builder

	prompt.WriteString("You are an expert AWS infrastructure automation agent with comprehensive state management capabilities.\n\n")

	// Add available tools context
	prompt.WriteString(a.GetAvailableToolsContext())
	prompt.WriteString("\n")

	prompt.WriteString("USER REQUEST: " + request + "\n\n")

	// === INFRASTRUCTURE STATE OVERVIEW ===
	prompt.WriteString("📊 INFRASTRUCTURE STATE OVERVIEW:\n")
	prompt.WriteString("Analyze ALL available resources from the state file to make informed decisions.\n\n")

	// Show current managed resources from state file
	if len(context.CurrentState.Resources) > 0 {
		prompt.WriteString("🏗️ MANAGED RESOURCES (from state file):\n")
		for resourceID, resource := range context.CurrentState.Resources {
			prompt.WriteString(fmt.Sprintf("- %s (%s): %s", resourceID, resource.Type, resource.Status))

			// Extract and show key properties from state file
			if resource.Properties != nil {
				var properties []string

				// Extract from direct properties
				for key, value := range resource.Properties {
					if key == "mcp_response" {
						// Extract from nested mcp_response
						if mcpMap, ok := value.(map[string]interface{}); ok {
							for mcpKey, mcpValue := range mcpMap {
								if mcpKey != "success" && mcpKey != "timestamp" && mcpKey != "message" {
									properties = append(properties, fmt.Sprintf("%s:%v", mcpKey, mcpValue))
								}
							}
						}
					} else if key != "status" {
						properties = append(properties, fmt.Sprintf("%s:%v", key, value))
					}
				}

				if len(properties) > 0 {
					prompt.WriteString(fmt.Sprintf(" [%s]", strings.Join(properties, ", ")))
				}
			}
			prompt.WriteString("\n")
		}
		prompt.WriteString("\n")
	}

	// Show discovered AWS resources (not in state file)
	if len(context.DiscoveredState) > 0 {
		prompt.WriteString("🔍 DISCOVERED AWS RESOURCES (not managed in state file):\n")
		for _, resource := range context.DiscoveredState {
			prompt.WriteString(fmt.Sprintf("- %s (%s): %s", resource.ID, resource.Type, resource.Status))

			if resource.Properties != nil {
				var properties []string

				// Show most relevant properties for each resource type
				relevantKeys := []string{"vpcId", "groupName", "instanceType", "cidrBlock", "name", "state", "availabilityZone"}
				for _, key := range relevantKeys {
					if value, exists := resource.Properties[key]; exists {
						properties = append(properties, fmt.Sprintf("%s:%v", key, value))
					}
				}

				if len(properties) > 0 {
					prompt.WriteString(fmt.Sprintf(" [%s]", strings.Join(properties, ", ")))
				}
			}
			prompt.WriteString("\n")
		}
		prompt.WriteString("\n")
	}

	// Show resource correlations if any
	if len(context.ResourceCorrelation) > 0 {
		prompt.WriteString("🔗 RESOURCE CORRELATIONS:\n")
		for managedID, correlation := range context.ResourceCorrelation {
			prompt.WriteString(fmt.Sprintf("- State file resource '%s' correlates with AWS resource '%s' (confidence: %.2f)\n",
				managedID, correlation.DiscoveredResource.ID, correlation.MatchConfidence))
		}
		prompt.WriteString("\n")
	}

	// Show any conflicts
	if len(context.Conflicts) > 0 {
		prompt.WriteString("⚠️ DETECTED CONFLICTS:\n")
		for _, conflict := range context.Conflicts {
			prompt.WriteString(fmt.Sprintf("- %s: %s (Resource: %s)\n", conflict.ConflictType, conflict.Details, conflict.ResourceID))
		}
		prompt.WriteString("\n")
	}

	// === DECISION GUIDELINES ===
	prompt.WriteString("🎯 DECISION-MAKING GUIDELINES:\n")
	prompt.WriteString("1. RESOURCE REUSE: Always prefer existing AWS resources over creating new ones\n")
	prompt.WriteString("2. STATE AWARENESS: Consider all resources in the state file for dependencies and conflicts\n")
	prompt.WriteString("3. INTELLIGENT PLANNING: Create execution plans that leverage existing infrastructure\n")
	prompt.WriteString("4. MINIMAL CHANGES: Make only necessary changes to achieve the user's request\n")
	prompt.WriteString("5. DEPENDENCY MANAGEMENT: Ensure proper dependency ordering in execution plans\n\n")

	// === AI DECISION PROMPT ===
	prompt.WriteString("📋 YOUR TASK:\n")
	prompt.WriteString("Based on the user request and ALL infrastructure state information above:\n")
	prompt.WriteString("1. Analyze what already exists in both managed and discovered resources\n")
	prompt.WriteString("2. Determine the minimal set of actions needed to fulfill the request\n")
	prompt.WriteString("3. Create an execution plan using available MCP tools\n")
	prompt.WriteString("4. Provide clear reasoning for your decisions\n\n")

	// === JSON RESPONSE SCHEMA ===
	prompt.WriteString("🔧 REQUIRED JSON RESPONSE FORMAT:\n")
	prompt.WriteString("Respond with ONLY valid JSON in this exact format:\n\n")
	prompt.WriteString("{\n")
	prompt.WriteString("  \"action\": \"create_infrastructure|update_infrastructure|delete_infrastructure|no_action\",\n")
	prompt.WriteString("  \"reasoning\": \"Detailed explanation of your analysis and decision-making process\",\n")
	prompt.WriteString("  \"confidence\": 0.0-1.0,\n")
	prompt.WriteString("  \"resourcesAnalyzed\": {\n")
	prompt.WriteString("    \"managedCount\": 0,\n")
	prompt.WriteString("    \"discoveredCount\": 0,\n")
	prompt.WriteString("    \"reusableResources\": [\"list of resources that can be reused\"]\n")
	prompt.WriteString("  },\n")
	prompt.WriteString("  \"executionPlan\": [\n")
	prompt.WriteString("    {\n")
	prompt.WriteString("      \"id\": \"step-1\",\n")
	prompt.WriteString("      \"name\": \"Step Description\",\n")
	prompt.WriteString("      \"description\": \"Detailed step description\",\n")
	prompt.WriteString("      \"action\": \"create|update|delete|validate\",\n")
	prompt.WriteString("      \"resourceId\": \"logical-resource-id\",\n")
	prompt.WriteString("      \"mcpTool\": \"exact-mcp-tool-name\",\n")
	prompt.WriteString("      \"toolParameters\": {\n")
	prompt.WriteString("        \"parameter\": \"value\"\n")
	prompt.WriteString("      },\n")
	prompt.WriteString("      \"dependsOn\": [\"list-of-step-ids\"],\n")
	prompt.WriteString("      \"estimatedDuration\": \"10s\",\n")
	prompt.WriteString("      \"status\": \"pending\"\n")
	prompt.WriteString("    }\n")
	prompt.WriteString("  ]\n")
	prompt.WriteString("}\n\n")

	// === CRITICAL INSTRUCTIONS ===
	prompt.WriteString("🚨 CRITICAL INSTRUCTIONS:\n")
	prompt.WriteString("1. ANALYZE ALL RESOURCES: Consider every resource shown above before making decisions\n")
	prompt.WriteString("2. REUSE FIRST: Always check if existing resources can fulfill the request\n")
	prompt.WriteString("3. USE EXACT TOOL NAMES: Only use MCP tool names shown in the tools context above\n")
	prompt.WriteString("4. PARAMETER ACCURACY: Use correct parameter names and types for each tool\n")
	prompt.WriteString("5. DEPENDENCY REFERENCES: Use {{step-id.resourceId}} format for dependencies\n")
	prompt.WriteString("6. JSON ONLY: Return only valid JSON - no markdown, no explanations, no extra text\n")
	prompt.WriteString("7. STATE FILE AWARENESS: Remember that managed resources exist in the state file\n\n")

	// === EXAMPLES ===
	prompt.WriteString("💡 DECISION EXAMPLES:\n")
	prompt.WriteString("Example 1 - Resource Reuse: If user wants a web server and you see existing VPC and security groups, reuse them\n")
	prompt.WriteString("Example 2 - Minimal Changes: If user wants to add a database and VPC exists, only create database resources\n")
	prompt.WriteString("Example 3 - No Action: If user requests something that already exists, return action: \"no_action\"\n\n")

	prompt.WriteString("BEGIN YOUR ANALYSIS AND PROVIDE YOUR JSON RESPONSE:\n")

	return prompt.String()
}
