package mcp

import (
	"aws-mcp-server/pkg/aws"
	"context"
	"fmt"
	"strconv"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/mark3labs/mcp-go/mcp"
)

// Interface defines available Security Group tools for AWS infrastructure management.
//
// Available Tools:
//   - listSecurityGroups()           : List all security groups in the region or VPC
//   - createSecurityGroup()          : Create a new security group (aws_security_group equivalent)
//   - addSecurityGroupIngressRule()  : Add an ingress rule to security group (aws_security_group_rule)
//   - addSecurityGroupEgressRule()   : Add an egress rule to security group (aws_security_group_rule)
//   - deleteSecurityGroup()          : Delete a security group
//
// Usage Example (Terraform-like workflow):
//   1. createSecurityGroup(name="web-sg", description="Web server security group", vpcId="vpc-xxx")
//   2. addSecurityGroupIngressRule(groupId="sg-xxx", protocol="tcp", fromPort=80, toPort=80, cidrBlocks=["0.0.0.0/0"])
//   3. addSecurityGroupIngressRule(groupId="sg-xxx", protocol="tcp", fromPort=443, toPort=443, cidrBlocks=["0.0.0.0/0"])
//   4. addSecurityGroupIngressRule(groupId="sg-xxx", protocol="tcp", fromPort=22, toPort=22, cidrBlocks=["10.0.0.0/16"])

func (h *ToolHandler) listSecurityGroups(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	var vpcID string
	if val, exists := arguments["vpcId"]; exists {
		vpcID, _ = val.(string)
	}

	securityGroups, err := h.awsClient.ListSecurityGroups(ctx, vpcID)
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to list security groups: %v", err))
	}

	// Format the response
	var sgList []map[string]interface{}
	for _, sg := range securityGroups {
		sgInfo := map[string]interface{}{
			"groupId":     awssdk.ToString(sg.GroupId),
			"groupName":   awssdk.ToString(sg.GroupName),
			"description": awssdk.ToString(sg.Description),
			"vpcId":       awssdk.ToString(sg.VpcId),
			"tags":        formatTags(sg.Tags),
		}

		// Add ingress rules
		var ingressRules []map[string]interface{}
		for _, rule := range sg.IpPermissions {
			ingressRules = append(ingressRules, formatSecurityGroupRule(rule))
		}
		sgInfo["ingressRules"] = ingressRules

		// Add egress rules
		var egressRules []map[string]interface{}
		for _, rule := range sg.IpPermissionsEgress {
			egressRules = append(egressRules, formatSecurityGroupRule(rule))
		}
		sgInfo["egressRules"] = egressRules

		sgList = append(sgList, sgInfo)
	}

	return h.createSuccessResponse(fmt.Sprintf("Found %d security groups", len(sgList)), map[string]interface{}{
		"securityGroups": sgList,
	})
}

func (h *ToolHandler) createSecurityGroup(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// Extract required parameters
	groupName, ok := arguments["groupName"].(string)
	if !ok || groupName == "" {
		return h.createErrorResponse("groupName is required")
	}

	description, ok := arguments["description"].(string)
	if !ok || description == "" {
		return h.createErrorResponse("description is required")
	}

	// Extract optional parameters
	var vpcID string
	if val, exists := arguments["vpcId"]; exists {
		vpcID, _ = val.(string)
	}

	// Extract tags
	tags := make(map[string]string)
	if tagsMap, exists := arguments["tags"].(map[string]interface{}); exists {
		for k, v := range tagsMap {
			if strVal, ok := v.(string); ok {
				tags[k] = strVal
			}
		}
	}

	// Add default Name tag if not provided
	if _, exists := tags["Name"]; !exists {
		tags["Name"] = groupName
	}

	params := aws.SecurityGroupParams{
		GroupName:   groupName,
		Description: description,
		VpcID:       vpcID,
		Tags:        tags,
	}

	result, err := h.awsClient.CreateSecurityGroup(ctx, params)
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to create security group: %v", err))
	}

	return h.createSuccessResponse("Security group created successfully", map[string]interface{}{
		"groupId":   awssdk.ToString(result.GroupId),
		"groupName": groupName,
		"vpcId":     vpcID,
	})
}

func (h *ToolHandler) addSecurityGroupIngressRule(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	return h.addSecurityGroupRule(ctx, arguments, "ingress")
}

func (h *ToolHandler) addSecurityGroupEgressRule(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	return h.addSecurityGroupRule(ctx, arguments, "egress")
}

func (h *ToolHandler) addSecurityGroupRule(ctx context.Context, arguments map[string]interface{}, ruleType string) (*mcp.CallToolResult, error) {
	// Extract required parameters
	groupID, ok := arguments["groupId"].(string)
	if !ok || groupID == "" {
		return h.createErrorResponse("groupId is required")
	}

	protocol, ok := arguments["protocol"].(string)
	if !ok || protocol == "" {
		return h.createErrorResponse("protocol is required (tcp, udp, icmp, or -1 for all)")
	}

	// Extract port parameters (only required for TCP/UDP)
	var fromPort, toPort int32
	if protocol == "tcp" || protocol == "udp" {
		if val, exists := arguments["fromPort"]; exists {
			if portFloat, ok := val.(float64); ok {
				fromPort = int32(portFloat)
			} else if portStr, ok := val.(string); ok {
				if port, err := strconv.Atoi(portStr); err == nil {
					fromPort = int32(port)
				}
			}
		}

		if val, exists := arguments["toPort"]; exists {
			if portFloat, ok := val.(float64); ok {
				toPort = int32(portFloat)
			} else if portStr, ok := val.(string); ok {
				if port, err := strconv.Atoi(portStr); err == nil {
					toPort = int32(port)
				}
			}
		}

		if fromPort == 0 || toPort == 0 {
			return h.createErrorResponse("fromPort and toPort are required for TCP/UDP protocols")
		}
	}

	// Extract CIDR blocks
	var cidrBlocks []string
	if cidrList, exists := arguments["cidrBlocks"].([]interface{}); exists {
		for _, cidr := range cidrList {
			if cidrStr, ok := cidr.(string); ok {
				cidrBlocks = append(cidrBlocks, cidrStr)
			}
		}
	}

	// Extract source security group
	var sourceSG string
	if val, exists := arguments["sourceSG"]; exists {
		sourceSG, _ = val.(string)
	}

	// Validate that either CIDR blocks or source SG is provided
	if len(cidrBlocks) == 0 && sourceSG == "" {
		return h.createErrorResponse("Either cidrBlocks or sourceSG must be provided")
	}

	params := aws.SecurityGroupRuleParams{
		GroupID:    groupID,
		Type:       ruleType,
		Protocol:   protocol,
		FromPort:   fromPort,
		ToPort:     toPort,
		CidrBlocks: cidrBlocks,
		SourceSG:   sourceSG,
	}

	err := h.awsClient.AddSecurityGroupRule(ctx, params)
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to add %s rule: %v", ruleType, err))
	}

	return h.createSuccessResponse(fmt.Sprintf("Security group %s rule added successfully", ruleType), map[string]interface{}{
		"groupId":    groupID,
		"ruleType":   ruleType,
		"protocol":   protocol,
		"fromPort":   fromPort,
		"toPort":     toPort,
		"cidrBlocks": cidrBlocks,
		"sourceSG":   sourceSG,
	})
}

func (h *ToolHandler) deleteSecurityGroup(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	groupID, ok := arguments["groupId"].(string)
	if !ok || groupID == "" {
		return h.createErrorResponse("groupId is required")
	}

	err := h.awsClient.DeleteSecurityGroup(ctx, groupID)
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to delete security group: %v", err))
	}

	return h.createSuccessResponse("Security group deleted successfully", map[string]interface{}{
		"groupId": groupID,
	})
}

// Helper function to format security group rules
func formatSecurityGroupRule(rule types.IpPermission) map[string]interface{} {
	ruleInfo := map[string]interface{}{
		"protocol": awssdk.ToString(rule.IpProtocol),
	}

	if rule.FromPort != nil {
		ruleInfo["fromPort"] = *rule.FromPort
	}
	if rule.ToPort != nil {
		ruleInfo["toPort"] = *rule.ToPort
	}

	// Add CIDR blocks
	var cidrs []string
	for _, ipRange := range rule.IpRanges {
		cidrs = append(cidrs, awssdk.ToString(ipRange.CidrIp))
	}
	if len(cidrs) > 0 {
		ruleInfo["cidrBlocks"] = cidrs
	}

	// Add source security groups
	var sourceSGs []string
	for _, groupPair := range rule.UserIdGroupPairs {
		sourceSGs = append(sourceSGs, awssdk.ToString(groupPair.GroupId))
	}
	if len(sourceSGs) > 0 {
		ruleInfo["sourceSecurityGroups"] = sourceSGs
	}

	return ruleInfo
}

// Helper function to format tags
func formatTags(tags []types.Tag) map[string]string {
	tagMap := make(map[string]string)
	for _, tag := range tags {
		tagMap[awssdk.ToString(tag.Key)] = awssdk.ToString(tag.Value)
	}
	return tagMap
}
