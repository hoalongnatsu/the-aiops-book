package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"aws-mcp-server/pkg/aws"
	"aws-mcp-server/pkg/types"

	"github.com/mark3labs/mcp-go/mcp"
)

type ResourceHandler struct {
	awsClient *aws.Client
}

func NewResourceHandler(awsClient *aws.Client) *ResourceHandler {
	return &ResourceHandler{
		awsClient: awsClient,
	}
}

// ReadResource handles requests for specific resources
func (h *ResourceHandler) ReadResource(ctx context.Context, uri string) (*mcp.ReadResourceResult, error) {
	switch {
	case uri == "aws://ec2/instances":
		return h.readEC2InstancesList(ctx)
	case strings.HasPrefix(uri, "aws://ec2/instances/"):
		instanceID := strings.TrimPrefix(uri, "aws://ec2/instances/")
		return h.readEC2Instance(ctx, instanceID)
	default:
		return nil, fmt.Errorf("unknown resource URI: %s", uri)
	}
}

// readEC2InstancesList returns a formatted list of all EC2 instances
func (h *ResourceHandler) readEC2InstancesList(ctx context.Context) (*mcp.ReadResourceResult, error) {
	instances, err := h.awsClient.ListEC2Instances(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list EC2 instances: %w", err)
	}

	// Format the data for AI consumption
	formatted := h.formatInstancesForAI(instances)

	jsonData, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal instances data: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{
			&mcp.TextResourceContents{
				URI:      "aws://ec2/instances",
				MIMEType: "application/json",
				Text:     string(jsonData),
			},
		},
	}, nil
}

// readEC2Instance returns detailed information about a specific instance
func (h *ResourceHandler) readEC2Instance(ctx context.Context, instanceID string) (*mcp.ReadResourceResult, error) {
	instance, err := h.awsClient.GetEC2Instance(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get EC2 instance: %w", err)
	}

	// Format for AI consumption
	formatted := h.formatInstanceForAI(*instance)

	jsonData, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal instance data: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{
			&mcp.TextResourceContents{
				URI:      fmt.Sprintf("aws://ec2/instances/%s", instanceID),
				MIMEType: "application/json",
				Text:     string(jsonData),
			},
		},
	}, nil
}

// formatInstancesForAI formats instance data optimally for AI processing
func (h *ResourceHandler) formatInstancesForAI(instances []types.AWSResource) map[string]interface{} {
	summary := map[string]interface{}{
		"total_instances":  len(instances),
		"instances":        make([]map[string]interface{}, 0, len(instances)),
		"summary_by_state": make(map[string]int),
		"summary_by_type":  make(map[string]int),
	}

	stateCount := make(map[string]int)
	typeCount := make(map[string]int)

	for _, instance := range instances {
		formatted := map[string]interface{}{
			"id":     instance.ID,
			"state":  instance.State,
			"type":   instance.Details["instanceType"],
			"region": instance.Region,
		}

		// Add name if available from tags
		if name, exists := instance.Tags["Name"]; exists {
			formatted["name"] = name
		}

		// Add IP addresses if available
		if publicIP := instance.Details["publicIpAddress"]; publicIP != nil {
			formatted["public_ip"] = publicIP
		}

		if privateIP := instance.Details["privateIpAddress"]; privateIP != nil {
			formatted["private_ip"] = privateIP
		}

		summary["instances"] = append(summary["instances"].([]map[string]interface{}), formatted)

		// Update counters
		stateCount[instance.State]++
		if instanceType, ok := instance.Details["instanceType"].(string); ok {
			typeCount[instanceType]++
		}
	}

	summary["summary_by_state"] = stateCount
	summary["summary_by_type"] = typeCount

	return summary
}

// formatInstanceForAI formats a single instance with comprehensive details
func (h *ResourceHandler) formatInstanceForAI(instance types.AWSResource) map[string]interface{} {
	formatted := map[string]interface{}{
		"id":        instance.ID,
		"type":      instance.Type,
		"state":     instance.State,
		"region":    instance.Region,
		"tags":      instance.Tags,
		"details":   instance.Details,
		"last_seen": instance.LastSeen.Format("2006-01-02T15:04:05Z07:00"),
	}

	// Add computed fields that AI systems find useful
	if name, exists := instance.Tags["Name"]; exists {
		formatted["name"] = name
	} else {
		formatted["name"] = instance.ID
	}

	// Add environment classification if available
	if env := instance.Tags["Environment"]; env != "" {
		formatted["environment"] = env
	}

	return formatted
}
