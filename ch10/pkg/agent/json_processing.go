package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aws-mcp-server/pkg/types"
)

// ========== Interface defines ==========

// JSONProcessingInterface defines JSON parsing and processing functionality for AI agent responses
//
// Available Functions:
//   - extractJSON()                : Extract complete JSON object from text with nested brace handling
//   - parseAgentDecision()         : Parse AI response into structured AgentDecision
//   - validateAgentDecisionJSON()  : Validate JSON structure and required fields
//   - fixCommonJSONIssues()        : Fix common JSON formatting issues from AI responses
//   - parseExecutionPlan()         : Parse execution plan from AI-generated JSON
//   - validateExecutionStep()      : Validate individual execution step structure
//
// Usage Example:
//   1. jsonStr := agent.extractJSON(aiResponse)
//   2. decision := agent.parseAgentDecision(jsonStr, decisionID, request)
//   3. agent.validateAgentDecisionJSON(decision)

// ========== JSON Processing Functions ==========

// extractJSON extracts a complete JSON object from text, handling nested braces
func (a *StateAwareAgent) extractJSON(text string) string {
	// Find the first opening brace
	start := strings.Index(text, "{")
	if start == -1 {
		return ""
	}

	// Count braces to find the matching closing brace
	braceCount := 0
	inString := false
	escaped := false

	for i := start; i < len(text); i++ {
		char := text[i]

		if escaped {
			escaped = false
			continue
		}

		if char == '\\' {
			escaped = true
			continue
		}

		if char == '"' {
			inString = !inString
			continue
		}

		if !inString {
			switch char {
			case '{':
				braceCount++
			case '}':
				braceCount--
				if braceCount == 0 {
					return text[start : i+1]
				}
			}
		}
	}

	return ""
}

// extractJSONAlternative tries alternative methods to extract JSON from text
func (a *StateAwareAgent) extractJSONAlternative(text string) string {
	// Method 1: Look for JSON in markdown code blocks
	if strings.Contains(text, "```json") {
		start := strings.Index(text, "```json")
		if start != -1 {
			start += 7 // Skip "```json"
			end := strings.Index(text[start:], "```")
			if end != -1 {
				jsonCandidate := strings.TrimSpace(text[start : start+end])
				if a.isValidJSON(jsonCandidate) {
					return jsonCandidate
				}
			}
		}
	}

	// Method 2: Look for JSON in regular code blocks
	if strings.Contains(text, "```") {
		start := strings.Index(text, "```")
		if start != -1 {
			start += 3 // Skip "```"
			// Skip language identifier if present
			if newlinePos := strings.Index(text[start:], "\n"); newlinePos != -1 {
				start += newlinePos + 1
			}
			end := strings.Index(text[start:], "```")
			if end != -1 {
				jsonCandidate := strings.TrimSpace(text[start : start+end])
				if a.isValidJSON(jsonCandidate) {
					return jsonCandidate
				}
			}
		}
	}

	// Method 3: Look for any JSON-like structure (starts with { and reasonable content)
	for i := 0; i < len(text); i++ {
		if text[i] == '{' {
			// Try to extract from this position
			extracted := a.extractJSONFromPosition(text, i)
			if extracted != "" && a.isValidJSON(extracted) {
				return extracted
			}
		}
	}

	return ""
}

// extractJSONFromPosition extracts JSON starting from a specific position
func (a *StateAwareAgent) extractJSONFromPosition(text string, start int) string {
	braceCount := 0
	inString := false
	escaped := false

	for i := start; i < len(text); i++ {
		char := text[i]

		if escaped {
			escaped = false
			continue
		}

		if char == '\\' {
			escaped = true
			continue
		}

		if char == '"' {
			inString = !inString
			continue
		}

		if !inString {
			switch char {
			case '{':
				braceCount++
			case '}':
				braceCount--
				if braceCount == 0 {
					return text[start : i+1]
				}
			}
		}
	}

	return ""
}

// isValidJSON checks if a string is valid JSON
func (a *StateAwareAgent) isValidJSON(s string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(s), &js) == nil
}

// attemptTruncatedJSONParse attempts to parse potentially truncated JSON responses
func (a *StateAwareAgent) attemptTruncatedJSONParse(response string) string {
	// Check if this looks like the start of a valid JSON but might be incomplete
	if !strings.HasPrefix(response, "{") {
		return ""
	}

	// Try to parse as-is first
	if a.isValidJSON(response) {
		return response
	}

	// If response appears truncated, try to complete it minimally
	// Look for common truncation patterns
	truncatedResponse := strings.TrimSpace(response)

	// If it ends with a quote but no closing brace, it might be truncated mid-string
	if strings.HasSuffix(truncatedResponse, "\"") {
		// Try adding closing quote and braces
		completed := truncatedResponse + "\"}}}"
		if a.isValidJSON(completed) {
			a.Logger.Info("Successfully completed truncated JSON response")
			return completed
		}
	}

	// If it ends without proper closure, try adding closing braces
	if !strings.HasSuffix(truncatedResponse, "}") {
		// Count open braces and try to close them
		openBraces := strings.Count(truncatedResponse, "{") - strings.Count(truncatedResponse, "}")
		if openBraces > 0 {
			completed := truncatedResponse + strings.Repeat("}", openBraces)
			if a.isValidJSON(completed) {
				a.Logger.Info("Successfully completed truncated JSON response by adding closing braces")
				return completed
			}
		}
	}

	// Try to find the last complete key-value pair and truncate there
	lastValidJson := a.findLastValidJSON(truncatedResponse)
	if lastValidJson != "" {
		a.Logger.Info("Found last valid JSON portion from truncated response")
		return lastValidJson
	}

	return ""
}

// findLastValidJSON attempts to find the last valid JSON by progressively removing content
func (a *StateAwareAgent) findLastValidJSON(text string) string {
	// Work backwards to find valid JSON
	for i := len(text) - 1; i >= 0; i-- {
		if text[i] == '}' {
			candidate := text[:i+1]
			if a.isValidJSON(candidate) {
				return candidate
			}
		}
	}
	return ""
}

// parseAIResponseWithPlan parses the AI response into an AgentDecision with execution plan
func (a *StateAwareAgent) parseAIResponseWithPlan(decisionID, request, response string) (*types.AgentDecision, error) {
	a.Logger.Debug("Parsing AI response for execution plan")

	// Log the raw response for debugging - ALWAYS log this for troubleshooting
	a.Logger.WithFields(map[string]interface{}{
		"raw_response_length": len(response),
		"raw_response":        response,
	}).Info("AI Response received")

	// Check if response appears to be truncated JSON
	if strings.HasPrefix(response, "{") && !strings.HasSuffix(response, "}") {
		a.Logger.WithFields(map[string]interface{}{
			"response_starts_with": response[:min(100, len(response))],
			"response_ends_with":   response[max(0, len(response)-100):],
		}).Warn("Response appears to be truncated JSON")
	}

	// Try multiple JSON extraction methods
	jsonStr := a.extractJSON(response)
	if jsonStr == "" {
		// Try alternative extraction methods
		jsonStr = a.extractJSONAlternative(response)
	}

	// Special handling for potentially truncated responses
	if jsonStr == "" && strings.HasPrefix(response, "{") {
		a.Logger.Warn("Attempting to parse potentially truncated JSON response")
		jsonStr = a.attemptTruncatedJSONParse(response)
	}

	if jsonStr == "" {
		a.Logger.WithFields(map[string]interface{}{
			"response_preview":  response[:min(500, len(response))],
			"response_length":   len(response),
			"starts_with_brace": strings.HasPrefix(response, "{"),
			"ends_with_brace":   strings.HasSuffix(response, "}"),
		}).Error("No valid JSON found in AI response")
		return nil, fmt.Errorf("no valid JSON found in AI response")
	}

	a.Logger.WithFields(map[string]interface{}{
		"extracted_json_length": len(jsonStr),
		"extracted_json":        jsonStr,
	}).Info("Successfully extracted JSON from AI response")

	// Parse JSON with execution plan - updated for native MCP tool support
	var parsed struct {
		Action        string                 `json:"action"`
		Reasoning     string                 `json:"reasoning"`
		Confidence    float64                `json:"confidence"`
		Parameters    map[string]interface{} `json:"parameters"`
		ExecutionPlan []struct {
			ID                string                 `json:"id"`
			Name              string                 `json:"name"`
			Description       string                 `json:"description"`
			Action            string                 `json:"action"`
			ResourceID        string                 `json:"resourceId"`
			MCPTool           string                 `json:"mcpTool"`        // New: Direct MCP tool name
			ToolParameters    map[string]interface{} `json:"toolParameters"` // New: Direct tool parameters
			Parameters        map[string]interface{} `json:"parameters"`     // Legacy fallback
			DependsOn         []string               `json:"dependsOn"`
			EstimatedDuration string                 `json:"estimatedDuration"`
			Status            string                 `json:"status"`
		} `json:"executionPlan"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		a.Logger.WithError(err).WithField("json", jsonStr).Error("Failed to parse AI response JSON")

		// Try fallback parsing without execution plan
		var simpleParsed struct {
			Action     string                 `json:"action"`
			Reasoning  string                 `json:"reasoning"`
			Confidence float64                `json:"confidence"`
			Parameters map[string]interface{} `json:"parameters"`
		}

		if fallbackErr := json.Unmarshal([]byte(jsonStr), &simpleParsed); fallbackErr != nil {
			return nil, fmt.Errorf("failed to parse AI response JSON: %w", err)
		}

		a.Logger.Warn("Using fallback parsing - no execution plan available")
		return &types.AgentDecision{
			ID:            decisionID,
			Action:        simpleParsed.Action,
			Resource:      request,
			Reasoning:     simpleParsed.Reasoning,
			Confidence:    simpleParsed.Confidence,
			Parameters:    simpleParsed.Parameters,
			ExecutionPlan: []*types.ExecutionPlanStep{}, // Empty plan
			Timestamp:     time.Now(),
		}, nil
	}

	// Convert execution plan with native MCP support
	var executionPlan []*types.ExecutionPlanStep
	for _, step := range parsed.ExecutionPlan {
		planStep := &types.ExecutionPlanStep{
			ID:                step.ID,
			Name:              step.Name,
			Description:       step.Description,
			Action:            step.Action,
			ResourceID:        step.ResourceID,
			MCPTool:           step.MCPTool,
			ToolParameters:    step.ToolParameters,
			Parameters:        step.Parameters,
			DependsOn:         step.DependsOn,
			EstimatedDuration: step.EstimatedDuration,
			Status:            step.Status,
		}

		// Ensure we have parameters - use ToolParameters if available, otherwise Parameters
		if len(planStep.ToolParameters) > 0 {
			// Native MCP mode - use ToolParameters as primary
			if planStep.Parameters == nil {
				planStep.Parameters = make(map[string]interface{})
			}
			// Copy tool parameters to legacy parameters for compatibility
			for key, value := range planStep.ToolParameters {
				planStep.Parameters[key] = value
			}
		}

		executionPlan = append(executionPlan, planStep)
	}

	return &types.AgentDecision{
		ID:            decisionID,
		Action:        parsed.Action,
		Resource:      request,
		Reasoning:     parsed.Reasoning,
		Confidence:    parsed.Confidence,
		Parameters:    parsed.Parameters,
		ExecutionPlan: executionPlan,
		Timestamp:     time.Now(),
	}, nil
}
