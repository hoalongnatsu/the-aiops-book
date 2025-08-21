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
	// EC2 Instances
	case uri == "aws://ec2/instances":
		return h.readEC2InstancesList(ctx)
	case strings.HasPrefix(uri, "aws://ec2/instances/"):
		instanceID := strings.TrimPrefix(uri, "aws://ec2/instances/")
		return h.readEC2Instance(ctx, instanceID)

	// VPCs
	case uri == "aws://vpc/vpcs":
		return h.readVPCsList(ctx)
	case strings.HasPrefix(uri, "aws://vpc/vpcs/"):
		vpcID := strings.TrimPrefix(uri, "aws://vpc/vpcs/")
		return h.readVPC(ctx, vpcID)

	// Subnets
	case uri == "aws://vpc/subnets":
		return h.readSubnetsList(ctx)
	case strings.HasPrefix(uri, "aws://vpc/subnets/"):
		subnetID := strings.TrimPrefix(uri, "aws://vpc/subnets/")
		return h.readSubnet(ctx, subnetID)

	// Auto Scaling Groups
	case uri == "aws://autoscaling/groups":
		return h.readAutoScalingGroupsList(ctx)
	case strings.HasPrefix(uri, "aws://autoscaling/groups/"):
		groupName := strings.TrimPrefix(uri, "aws://autoscaling/groups/")
		return h.readAutoScalingGroup(ctx, groupName)

	// Load Balancers
	case uri == "aws://elbv2/loadbalancers":
		return h.readLoadBalancersList(ctx)
	case strings.HasPrefix(uri, "aws://elbv2/loadbalancers/"):
		loadBalancerArn := strings.TrimPrefix(uri, "aws://elbv2/loadbalancers/")
		return h.readLoadBalancer(ctx, loadBalancerArn)

	// Target Groups
	case uri == "aws://elbv2/targetgroups":
		return h.readTargetGroupsList(ctx)
	case strings.HasPrefix(uri, "aws://elbv2/targetgroups/"):
		targetGroupArn := strings.TrimPrefix(uri, "aws://elbv2/targetgroups/")
		return h.readTargetGroup(ctx, targetGroupArn)

	// Launch Templates
	case uri == "aws://ec2/launchtemplates":
		return h.readLaunchTemplatesList(ctx)
	case strings.HasPrefix(uri, "aws://ec2/launchtemplates/"):
		templateID := strings.TrimPrefix(uri, "aws://ec2/launchtemplates/")
		return h.readLaunchTemplate(ctx, templateID)

	// AMIs
	case uri == "aws://ec2/images":
		return h.readAMIsList(ctx)
	case strings.HasPrefix(uri, "aws://ec2/images/"):
		imageID := strings.TrimPrefix(uri, "aws://ec2/images/")
		return h.readAMI(ctx, imageID)

	// RDS Instances
	case uri == "aws://rds/instances":
		return h.readRDSInstancesList(ctx)
	case strings.HasPrefix(uri, "aws://rds/instances/"):
		dbInstanceIdentifier := strings.TrimPrefix(uri, "aws://rds/instances/")
		return h.readRDSInstance(ctx, dbInstanceIdentifier)

	// RDS Snapshots
	case uri == "aws://rds/snapshots":
		return h.readRDSSnapshotsList(ctx)
	case strings.HasPrefix(uri, "aws://rds/snapshots/"):
		snapshotID := strings.TrimPrefix(uri, "aws://rds/snapshots/")
		return h.readRDSSnapshot(ctx, snapshotID)

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
func (h *ResourceHandler) formatInstancesForAI(instances []*types.AWSResource) map[string]interface{} {
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

// VPC Resource Handlers

// readVPCsList returns a formatted list of all VPCs
func (h *ResourceHandler) readVPCsList(ctx context.Context) (*mcp.ReadResourceResult, error) {
	vpcs, err := h.awsClient.DescribeVPCs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to describe VPCs: %w", err)
	}

	// Format for AI consumption
	formatted := h.formatVPCsForAI(vpcs)

	jsonData, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal VPCs data: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{
			&mcp.TextResourceContents{
				URI:      "aws://vpc/vpcs",
				MIMEType: "application/json",
				Text:     string(jsonData),
			},
		},
	}, nil
}

// readVPC returns detailed information about a specific VPC
func (h *ResourceHandler) readVPC(ctx context.Context, vpcID string) (*mcp.ReadResourceResult, error) {
	vpc, err := h.awsClient.GetVPC(ctx, vpcID)
	if err != nil {
		return nil, fmt.Errorf("failed to get VPC: %w", err)
	}

	// Format for AI consumption
	formatted := h.formatVPCForAI(*vpc)

	jsonData, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal VPC data: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{
			&mcp.TextResourceContents{
				URI:      fmt.Sprintf("aws://vpc/vpcs/%s", vpcID),
				MIMEType: "application/json",
				Text:     string(jsonData),
			},
		},
	}, nil
}

// Subnet Resource Handlers

// readSubnetsList returns a formatted list of all subnets
func (h *ResourceHandler) readSubnetsList(ctx context.Context) (*mcp.ReadResourceResult, error) {
	subnets, err := h.awsClient.DescribeSubnetsAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to describe subnets: %w", err)
	}

	// Format for AI consumption
	formatted := h.formatSubnetsForAI(subnets)

	jsonData, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal subnets data: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{
			&mcp.TextResourceContents{
				URI:      "aws://vpc/subnets",
				MIMEType: "application/json",
				Text:     string(jsonData),
			},
		},
	}, nil
}

// readSubnet returns detailed information about a specific subnet
func (h *ResourceHandler) readSubnet(ctx context.Context, subnetID string) (*mcp.ReadResourceResult, error) {
	subnet, err := h.awsClient.GetSubnet(ctx, subnetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get subnet: %w", err)
	}

	// Format for AI consumption
	formatted := h.formatSubnetForAI(*subnet)

	jsonData, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal subnet data: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{
			&mcp.TextResourceContents{
				URI:      fmt.Sprintf("aws://vpc/subnets/%s", subnetID),
				MIMEType: "application/json",
				Text:     string(jsonData),
			},
		},
	}, nil
}

// Auto Scaling Group Resource Handlers

// readAutoScalingGroupsList returns a formatted list of all Auto Scaling Groups
func (h *ResourceHandler) readAutoScalingGroupsList(ctx context.Context) (*mcp.ReadResourceResult, error) {
	groups, err := h.awsClient.DescribeAutoScalingGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to describe Auto Scaling Groups: %w", err)
	}

	// Format for AI consumption
	formatted := h.formatAutoScalingGroupsForAI(groups)

	jsonData, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Auto Scaling Groups data: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{
			&mcp.TextResourceContents{
				URI:      "aws://autoscaling/groups",
				MIMEType: "application/json",
				Text:     string(jsonData),
			},
		},
	}, nil
}

// readAutoScalingGroup returns detailed information about a specific Auto Scaling Group
func (h *ResourceHandler) readAutoScalingGroup(ctx context.Context, groupName string) (*mcp.ReadResourceResult, error) {
	group, err := h.awsClient.GetAutoScalingGroup(ctx, groupName)
	if err != nil {
		return nil, fmt.Errorf("failed to get Auto Scaling Group: %w", err)
	}

	// Format for AI consumption
	formatted := h.formatAutoScalingGroupForAI(*group)

	jsonData, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Auto Scaling Group data: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{
			&mcp.TextResourceContents{
				URI:      fmt.Sprintf("aws://autoscaling/groups/%s", groupName),
				MIMEType: "application/json",
				Text:     string(jsonData),
			},
		},
	}, nil
}

// Load Balancer Resource Handlers

// readLoadBalancersList returns a formatted list of all Load Balancers
func (h *ResourceHandler) readLoadBalancersList(ctx context.Context) (*mcp.ReadResourceResult, error) {
	loadBalancers, err := h.awsClient.DescribeLoadBalancers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to describe Load Balancers: %w", err)
	}

	// Format for AI consumption
	formatted := h.formatLoadBalancersForAI(loadBalancers)

	jsonData, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Load Balancers data: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{
			&mcp.TextResourceContents{
				URI:      "aws://elbv2/loadbalancers",
				MIMEType: "application/json",
				Text:     string(jsonData),
			},
		},
	}, nil
}

// readLoadBalancer returns detailed information about a specific Load Balancer
func (h *ResourceHandler) readLoadBalancer(ctx context.Context, loadBalancerArn string) (*mcp.ReadResourceResult, error) {
	loadBalancer, err := h.awsClient.GetLoadBalancer(ctx, loadBalancerArn)
	if err != nil {
		return nil, fmt.Errorf("failed to get Load Balancer: %w", err)
	}

	// Format for AI consumption
	formatted := h.formatLoadBalancerForAI(*loadBalancer)

	jsonData, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Load Balancer data: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{
			&mcp.TextResourceContents{
				URI:      fmt.Sprintf("aws://elbv2/loadbalancers/%s", loadBalancerArn),
				MIMEType: "application/json",
				Text:     string(jsonData),
			},
		},
	}, nil
}

// Target Group Resource Handlers

// readTargetGroupsList returns a formatted list of all Target Groups
func (h *ResourceHandler) readTargetGroupsList(ctx context.Context) (*mcp.ReadResourceResult, error) {
	targetGroups, err := h.awsClient.DescribeTargetGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to describe Target Groups: %w", err)
	}

	// Format for AI consumption
	formatted := h.formatTargetGroupsForAI(targetGroups)

	jsonData, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Target Groups data: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{
			&mcp.TextResourceContents{
				URI:      "aws://elbv2/targetgroups",
				MIMEType: "application/json",
				Text:     string(jsonData),
			},
		},
	}, nil
}

// readTargetGroup returns detailed information about a specific Target Group
func (h *ResourceHandler) readTargetGroup(ctx context.Context, targetGroupArn string) (*mcp.ReadResourceResult, error) {
	targetGroup, err := h.awsClient.GetTargetGroup(ctx, targetGroupArn)
	if err != nil {
		return nil, fmt.Errorf("failed to get Target Group: %w", err)
	}

	// Format for AI consumption
	formatted := h.formatTargetGroupForAI(*targetGroup)

	jsonData, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Target Group data: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{
			&mcp.TextResourceContents{
				URI:      fmt.Sprintf("aws://elbv2/targetgroups/%s", targetGroupArn),
				MIMEType: "application/json",
				Text:     string(jsonData),
			},
		},
	}, nil
}

// Launch Template Resource Handlers

// readLaunchTemplatesList returns a formatted list of all Launch Templates
func (h *ResourceHandler) readLaunchTemplatesList(ctx context.Context) (*mcp.ReadResourceResult, error) {
	templates, err := h.awsClient.DescribeLaunchTemplates(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to describe Launch Templates: %w", err)
	}

	// Format for AI consumption
	formatted := h.formatLaunchTemplatesForAI(templates)

	jsonData, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Launch Templates data: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{
			&mcp.TextResourceContents{
				URI:      "aws://ec2/launchtemplates",
				MIMEType: "application/json",
				Text:     string(jsonData),
			},
		},
	}, nil
}

// readLaunchTemplate returns detailed information about a specific Launch Template
func (h *ResourceHandler) readLaunchTemplate(ctx context.Context, templateID string) (*mcp.ReadResourceResult, error) {
	template, err := h.awsClient.GetLaunchTemplate(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Launch Template: %w", err)
	}

	// Format for AI consumption
	formatted := h.formatLaunchTemplateForAI(*template)

	jsonData, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Launch Template data: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{
			&mcp.TextResourceContents{
				URI:      fmt.Sprintf("aws://ec2/launchtemplates/%s", templateID),
				MIMEType: "application/json",
				Text:     string(jsonData),
			},
		},
	}, nil
}

// AMI Resource Handlers

// readAMIsList returns a formatted list of all AMIs
func (h *ResourceHandler) readAMIsList(ctx context.Context) (*mcp.ReadResourceResult, error) {
	amis, err := h.awsClient.DescribeAMIs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to describe AMIs: %w", err)
	}

	// Format for AI consumption
	formatted := h.formatAMIsForAI(amis)

	jsonData, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal AMIs data: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{
			&mcp.TextResourceContents{
				URI:      "aws://ec2/images",
				MIMEType: "application/json",
				Text:     string(jsonData),
			},
		},
	}, nil
}

// readAMI returns detailed information about a specific AMI
func (h *ResourceHandler) readAMI(ctx context.Context, imageID string) (*mcp.ReadResourceResult, error) {
	ami, err := h.awsClient.GetAMI(ctx, imageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get AMI: %w", err)
	}

	// Format for AI consumption
	formatted := h.formatAMIForAI(*ami)

	jsonData, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal AMI data: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{
			&mcp.TextResourceContents{
				URI:      fmt.Sprintf("aws://ec2/images/%s", imageID),
				MIMEType: "application/json",
				Text:     string(jsonData),
			},
		},
	}, nil
}

// ========== Formatting Methods for AI Consumption ==========

// formatVPCsForAI formats VPCs data optimally for AI processing
func (h *ResourceHandler) formatVPCsForAI(vpcs []*types.AWSResource) map[string]interface{} {
	summary := map[string]interface{}{
		"total_vpcs":       len(vpcs),
		"vpcs":             make([]map[string]interface{}, 0, len(vpcs)),
		"summary_by_state": make(map[string]int),
	}

	stateCount := make(map[string]int)

	for _, vpc := range vpcs {
		vpcInfo := map[string]interface{}{
			"id":      vpc.ID,
			"state":   vpc.State,
			"region":  vpc.Region,
			"details": vpc.Details,
			"tags":    vpc.Tags,
		}
		summary["vpcs"] = append(summary["vpcs"].([]map[string]interface{}), vpcInfo)
		stateCount[vpc.State]++
	}

	summary["summary_by_state"] = stateCount
	return summary
}

// formatVPCForAI formats a single VPC for AI processing
func (h *ResourceHandler) formatVPCForAI(vpc types.AWSResource) map[string]interface{} {
	return map[string]interface{}{
		"vpc": map[string]interface{}{
			"id":       vpc.ID,
			"state":    vpc.State,
			"region":   vpc.Region,
			"details":  vpc.Details,
			"tags":     vpc.Tags,
			"lastSeen": vpc.LastSeen,
		},
	}
}

// formatSubnetsForAI formats Subnets data optimally for AI processing
func (h *ResourceHandler) formatSubnetsForAI(subnets []*types.AWSResource) map[string]interface{} {
	summary := map[string]interface{}{
		"total_subnets":    len(subnets),
		"subnets":          make([]map[string]interface{}, 0, len(subnets)),
		"summary_by_state": make(map[string]int),
		"summary_by_vpc":   make(map[string]int),
	}

	stateCount := make(map[string]int)
	vpcCount := make(map[string]int)

	for _, subnet := range subnets {
		subnetInfo := map[string]interface{}{
			"id":      subnet.ID,
			"state":   subnet.State,
			"region":  subnet.Region,
			"details": subnet.Details,
			"tags":    subnet.Tags,
		}
		summary["subnets"] = append(summary["subnets"].([]map[string]interface{}), subnetInfo)
		stateCount[subnet.State]++

		if vpcID, ok := subnet.Details["vpcId"].(string); ok {
			vpcCount[vpcID]++
		}
	}

	summary["summary_by_state"] = stateCount
	summary["summary_by_vpc"] = vpcCount
	return summary
}

// formatSubnetForAI formats a single Subnet for AI processing
func (h *ResourceHandler) formatSubnetForAI(subnet types.AWSResource) map[string]interface{} {
	return map[string]interface{}{
		"subnet": map[string]interface{}{
			"id":       subnet.ID,
			"state":    subnet.State,
			"region":   subnet.Region,
			"details":  subnet.Details,
			"tags":     subnet.Tags,
			"lastSeen": subnet.LastSeen,
		},
	}
}

// formatAutoScalingGroupsForAI formats ASG data optimally for AI processing
func (h *ResourceHandler) formatAutoScalingGroupsForAI(groups []*types.AWSResource) map[string]interface{} {
	summary := map[string]interface{}{
		"total_groups":    len(groups),
		"groups":          make([]map[string]interface{}, 0, len(groups)),
		"total_instances": 0,
	}

	totalInstances := 0

	for _, group := range groups {
		groupInfo := map[string]interface{}{
			"id":      group.ID,
			"state":   group.State,
			"region":  group.Region,
			"details": group.Details,
			"tags":    group.Tags,
		}
		summary["groups"] = append(summary["groups"].([]map[string]interface{}), groupInfo)

		if currentSize, ok := group.Details["desiredCapacity"].(int32); ok {
			totalInstances += int(currentSize)
		}
	}

	summary["total_instances"] = totalInstances
	return summary
}

// formatAutoScalingGroupForAI formats a single ASG for AI processing
func (h *ResourceHandler) formatAutoScalingGroupForAI(group types.AWSResource) map[string]interface{} {
	return map[string]interface{}{
		"group": map[string]interface{}{
			"id":       group.ID,
			"state":    group.State,
			"region":   group.Region,
			"details":  group.Details,
			"tags":     group.Tags,
			"lastSeen": group.LastSeen,
		},
	}
}

// formatLoadBalancersForAI formats Load Balancer data optimally for AI processing
func (h *ResourceHandler) formatLoadBalancersForAI(loadBalancers []*types.AWSResource) map[string]interface{} {
	summary := map[string]interface{}{
		"total_load_balancers": len(loadBalancers),
		"load_balancers":       make([]map[string]interface{}, 0, len(loadBalancers)),
		"summary_by_state":     make(map[string]int),
		"summary_by_type":      make(map[string]int),
	}

	stateCount := make(map[string]int)
	typeCount := make(map[string]int)

	for _, lb := range loadBalancers {
		lbInfo := map[string]interface{}{
			"id":      lb.ID,
			"state":   lb.State,
			"region":  lb.Region,
			"details": lb.Details,
			"tags":    lb.Tags,
		}
		summary["load_balancers"] = append(summary["load_balancers"].([]map[string]interface{}), lbInfo)
		stateCount[lb.State]++

		if lbType, ok := lb.Details["type"].(string); ok {
			typeCount[lbType]++
		}
	}

	summary["summary_by_state"] = stateCount
	summary["summary_by_type"] = typeCount
	return summary
}

// formatLoadBalancerForAI formats a single Load Balancer for AI processing
func (h *ResourceHandler) formatLoadBalancerForAI(loadBalancer types.AWSResource) map[string]interface{} {
	return map[string]interface{}{
		"load_balancer": map[string]interface{}{
			"id":       loadBalancer.ID,
			"state":    loadBalancer.State,
			"region":   loadBalancer.Region,
			"details":  loadBalancer.Details,
			"tags":     loadBalancer.Tags,
			"lastSeen": loadBalancer.LastSeen,
		},
	}
}

// formatTargetGroupsForAI formats Target Group data optimally for AI processing
func (h *ResourceHandler) formatTargetGroupsForAI(targetGroups []*types.AWSResource) map[string]interface{} {
	summary := map[string]interface{}{
		"total_target_groups": len(targetGroups),
		"target_groups":       make([]map[string]interface{}, 0, len(targetGroups)),
		"summary_by_state":    make(map[string]int),
		"summary_by_protocol": make(map[string]int),
	}

	stateCount := make(map[string]int)
	protocolCount := make(map[string]int)

	for _, tg := range targetGroups {
		tgInfo := map[string]interface{}{
			"id":      tg.ID,
			"state":   tg.State,
			"region":  tg.Region,
			"details": tg.Details,
			"tags":    tg.Tags,
		}
		summary["target_groups"] = append(summary["target_groups"].([]map[string]interface{}), tgInfo)
		stateCount[tg.State]++

		if protocol, ok := tg.Details["protocol"].(string); ok {
			protocolCount[protocol]++
		}
	}

	summary["summary_by_state"] = stateCount
	summary["summary_by_protocol"] = protocolCount
	return summary
}

// formatTargetGroupForAI formats a single Target Group for AI processing
func (h *ResourceHandler) formatTargetGroupForAI(targetGroup types.AWSResource) map[string]interface{} {
	return map[string]interface{}{
		"target_group": map[string]interface{}{
			"id":       targetGroup.ID,
			"state":    targetGroup.State,
			"region":   targetGroup.Region,
			"details":  targetGroup.Details,
			"tags":     targetGroup.Tags,
			"lastSeen": targetGroup.LastSeen,
		},
	}
}

// formatLaunchTemplatesForAI formats Launch Template data optimally for AI processing
func (h *ResourceHandler) formatLaunchTemplatesForAI(templates []*types.AWSResource) map[string]interface{} {
	summary := map[string]interface{}{
		"total_templates":  len(templates),
		"templates":        make([]map[string]interface{}, 0, len(templates)),
		"summary_by_state": make(map[string]int),
	}

	stateCount := make(map[string]int)

	for _, template := range templates {
		templateInfo := map[string]interface{}{
			"id":      template.ID,
			"state":   template.State,
			"region":  template.Region,
			"details": template.Details,
			"tags":    template.Tags,
		}
		summary["templates"] = append(summary["templates"].([]map[string]interface{}), templateInfo)
		stateCount[template.State]++
	}

	summary["summary_by_state"] = stateCount
	return summary
}

// formatLaunchTemplateForAI formats a single Launch Template for AI processing
func (h *ResourceHandler) formatLaunchTemplateForAI(template types.AWSResource) map[string]interface{} {
	return map[string]interface{}{
		"template": map[string]interface{}{
			"id":       template.ID,
			"state":    template.State,
			"region":   template.Region,
			"details":  template.Details,
			"tags":     template.Tags,
			"lastSeen": template.LastSeen,
		},
	}
}

// formatAMIsForAI formats AMI data optimally for AI processing
func (h *ResourceHandler) formatAMIsForAI(amis []*types.AWSResource) map[string]interface{} {
	summary := map[string]interface{}{
		"total_amis":          len(amis),
		"amis":                make([]map[string]interface{}, 0, len(amis)),
		"summary_by_state":    make(map[string]int),
		"summary_by_platform": make(map[string]int),
	}

	stateCount := make(map[string]int)
	platformCount := make(map[string]int)

	for _, ami := range amis {
		amiInfo := map[string]interface{}{
			"id":      ami.ID,
			"state":   ami.State,
			"region":  ami.Region,
			"details": ami.Details,
			"tags":    ami.Tags,
		}
		summary["amis"] = append(summary["amis"].([]map[string]interface{}), amiInfo)
		stateCount[ami.State]++

		if platform, ok := ami.Details["platform"].(string); ok && platform != "" {
			platformCount[platform]++
		} else {
			platformCount["linux"]++ // Default assumption
		}
	}

	summary["summary_by_state"] = stateCount
	summary["summary_by_platform"] = platformCount
	return summary
}

// ========== RDS Resource Handlers ==========

// readRDSInstancesList returns a formatted list of all RDS instances
func (h *ResourceHandler) readRDSInstancesList(ctx context.Context) (*mcp.ReadResourceResult, error) {
	instances, err := h.awsClient.ListDBInstances(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list RDS instances: %w", err)
	}

	// Format the data for AI consumption
	formatted := h.formatRDSInstancesForAI(instances)

	jsonData, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal RDS instances: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{
			&mcp.TextResourceContents{
				URI:      "aws://rds/instances",
				MIMEType: "application/json",
				Text:     string(jsonData),
			},
		},
	}, nil
}

// readRDSInstance returns detailed information about a specific RDS instance
func (h *ResourceHandler) readRDSInstance(ctx context.Context, dbInstanceIdentifier string) (*mcp.ReadResourceResult, error) {
	instance, err := h.awsClient.GetDBInstance(ctx, dbInstanceIdentifier)
	if err != nil {
		return nil, fmt.Errorf("failed to get RDS instance %s: %w", dbInstanceIdentifier, err)
	}

	jsonData, err := json.MarshalIndent(instance, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal RDS instance: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{
			&mcp.TextResourceContents{
				URI:      fmt.Sprintf("aws://rds/instances/%s", dbInstanceIdentifier),
				MIMEType: "application/json",
				Text:     string(jsonData),
			},
		},
	}, nil
}

// readRDSSnapshotsList returns a formatted list of all RDS snapshots
func (h *ResourceHandler) readRDSSnapshotsList(ctx context.Context) (*mcp.ReadResourceResult, error) {
	snapshots, err := h.awsClient.ListDBSnapshots(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list RDS snapshots: %w", err)
	}

	// Format the data for AI consumption
	formatted := h.formatRDSSnapshotsForAI(snapshots)

	jsonData, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal RDS snapshots: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{
			&mcp.TextResourceContents{
				URI:      "aws://rds/snapshots",
				MIMEType: "application/json",
				Text:     string(jsonData),
			},
		},
	}, nil
}

// readRDSSnapshot returns detailed information about a specific RDS snapshot
func (h *ResourceHandler) readRDSSnapshot(ctx context.Context, snapshotID string) (*mcp.ReadResourceResult, error) {
	// For individual snapshot, we can filter the list by ID
	snapshots, err := h.awsClient.ListDBSnapshots(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list RDS snapshots: %w", err)
	}

	var targetSnapshot *types.AWSResource
	for _, snapshot := range snapshots {
		if snapshot.ID == snapshotID {
			targetSnapshot = &snapshot
			break
		}
	}

	if targetSnapshot == nil {
		return nil, fmt.Errorf("RDS snapshot %s not found", snapshotID)
	}

	jsonData, err := json.MarshalIndent(targetSnapshot, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal RDS snapshot: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{
			&mcp.TextResourceContents{
				URI:      fmt.Sprintf("aws://rds/snapshots/%s", snapshotID),
				MIMEType: "application/json",
				Text:     string(jsonData),
			},
		},
	}, nil
}

// formatRDSInstancesForAI formats RDS instances data for AI consumption
func (h *ResourceHandler) formatRDSInstancesForAI(instances []types.AWSResource) map[string]interface{} {
	summary := map[string]interface{}{
		"total_instances": len(instances),
		"instances":       make([]map[string]interface{}, 0),
		"summary":         "RDS Database Instances",
		"description":     "All RDS database instances in the region with their current status, engine types, and configuration",
	}

	// Count by state and engine
	stateCount := make(map[string]int)
	engineCount := make(map[string]int)

	for _, instance := range instances {
		instanceInfo := map[string]interface{}{
			"id":      instance.ID,
			"type":    instance.Type,
			"state":   instance.State,
			"region":  instance.Region,
			"details": instance.Details,
			"tags":    instance.Tags,
		}
		summary["instances"] = append(summary["instances"].([]map[string]interface{}), instanceInfo)
		stateCount[instance.State]++

		if engine, ok := instance.Details["engine"].(string); ok && engine != "" {
			engineCount[engine]++
		}
	}

	summary["summary_by_state"] = stateCount
	summary["summary_by_engine"] = engineCount
	return summary
}

// formatRDSSnapshotsForAI formats RDS snapshots data for AI consumption
func (h *ResourceHandler) formatRDSSnapshotsForAI(snapshots []types.AWSResource) map[string]interface{} {
	summary := map[string]interface{}{
		"total_snapshots": len(snapshots),
		"snapshots":       make([]map[string]interface{}, 0),
		"summary":         "RDS Database Snapshots",
		"description":     "All RDS database snapshots in the region with their status and source instances",
	}

	// Count by state and engine
	stateCount := make(map[string]int)
	engineCount := make(map[string]int)

	for _, snapshot := range snapshots {
		snapshotInfo := map[string]interface{}{
			"id":      snapshot.ID,
			"type":    snapshot.Type,
			"state":   snapshot.State,
			"region":  snapshot.Region,
			"details": snapshot.Details,
			"tags":    snapshot.Tags,
		}
		summary["snapshots"] = append(summary["snapshots"].([]map[string]interface{}), snapshotInfo)
		stateCount[snapshot.State]++

		if engine, ok := snapshot.Details["engine"].(string); ok && engine != "" {
			engineCount[engine]++
		}
	}

	summary["summary_by_state"] = stateCount
	summary["summary_by_engine"] = engineCount
	return summary
}

// formatAMIForAI formats a single AMI for AI processing
func (h *ResourceHandler) formatAMIForAI(ami types.AWSResource) map[string]interface{} {
	return map[string]interface{}{
		"ami": map[string]interface{}{
			"id":       ami.ID,
			"state":    ami.State,
			"region":   ami.Region,
			"details":  ami.Details,
			"tags":     ami.Tags,
			"lastSeen": ami.LastSeen,
		},
	}
}
