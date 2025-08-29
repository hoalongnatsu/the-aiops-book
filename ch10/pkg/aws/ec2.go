package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"aws-mcp-server/pkg/types"

	"github.com/sirupsen/logrus"
)

// ========== EC2 Instance Management Methods ==========

// CreateEC2Instance creates a new EC2 instance
func (c *Client) CreateEC2Instance(ctx context.Context, params CreateInstanceParams) (*types.AWSResource, error) {
	input := &ec2.RunInstancesInput{
		ImageId:      aws.String(params.ImageID),
		InstanceType: ec2types.InstanceType(params.InstanceType),
		MinCount:     aws.Int32(1),
		MaxCount:     aws.Int32(1),
	}

	if params.KeyName != "" {
		input.KeyName = aws.String(params.KeyName)
	}

	if params.SecurityGroupID != "" {
		input.SecurityGroupIds = []string{params.SecurityGroupID}
	}

	if params.SubnetID != "" {
		input.SubnetId = &params.SubnetID
	} else {
		// If no subnet is specified, try to find a default subnet
		defaultSubnetID, err := c.findDefaultSubnet(ctx)
		if err != nil {
			c.logger.WithError(err).Warn("No default subnet found, instance will be created without VPC specification")
		} else {
			input.SubnetId = &defaultSubnetID
			c.logger.WithField("subnetId", defaultSubnetID).Info("Using default subnet")
		}
	}

	// Add tag specifications during creation if name is provided
	if params.Name != "" {
		input.TagSpecifications = []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeInstance,
				Tags: []ec2types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(params.Name),
					},
				},
			},
		}
	}

	result, err := c.ec2.RunInstances(ctx, input)
	if err != nil {
		c.logger.WithError(err).Error("Failed to create EC2 instance")
		return nil, fmt.Errorf("failed to create instance: %w", err)
	}

	if len(result.Instances) == 0 {
		return nil, fmt.Errorf("no instances created")
	}

	instance := result.Instances[0]
	resource := c.convertEC2Instance(instance)

	c.logger.WithField("instanceId", *instance.InstanceId).Info("EC2 instance created successfully")
	return resource, nil
}

// StartEC2Instance starts a stopped EC2 instance
func (c *Client) StartEC2Instance(ctx context.Context, instanceID string) error {
	input := &ec2.StartInstancesInput{
		InstanceIds: []string{instanceID},
	}

	_, err := c.ec2.StartInstances(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to start instance %s: %w", instanceID, err)
	}

	c.logger.WithField("instanceId", instanceID).Info("EC2 instance start initiated")
	return nil
}

// StopEC2Instance stops a running EC2 instance
func (c *Client) StopEC2Instance(ctx context.Context, instanceID string) error {
	input := &ec2.StopInstancesInput{
		InstanceIds: []string{instanceID},
	}

	_, err := c.ec2.StopInstances(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to stop instance %s: %w", instanceID, err)
	}

	c.logger.WithField("instanceId", instanceID).Info("EC2 instance stop initiated")
	return nil
}

// TerminateEC2Instance terminates an EC2 instance
func (c *Client) TerminateEC2Instance(ctx context.Context, instanceID string) error {
	input := &ec2.TerminateInstancesInput{
		InstanceIds: []string{instanceID},
	}

	_, err := c.ec2.TerminateInstances(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to terminate instance %s: %w", instanceID, err)
	}

	c.logger.WithField("instanceId", instanceID).Info("EC2 instance termination initiated")
	return nil
}

// convertEC2Instance converts an EC2 instance to our internal resource representation
func (c *Client) convertEC2Instance(instance ec2types.Instance) *types.AWSResource {
	tags := make(map[string]string)
	for _, tag := range instance.Tags {
		if tag.Key != nil && tag.Value != nil {
			tags[*tag.Key] = *tag.Value
		}
	}

	details := map[string]interface{}{
		"instanceType":     string(instance.InstanceType),
		"imageId":          aws.ToString(instance.ImageId),
		"launchTime":       instance.LaunchTime,
		"privateIpAddress": aws.ToString(instance.PrivateIpAddress),
		"publicIpAddress":  aws.ToString(instance.PublicIpAddress),
		"subnetId":         aws.ToString(instance.SubnetId),
		"vpcId":            aws.ToString(instance.VpcId),
	}

	if instance.Placement != nil {
		details["availabilityZone"] = aws.ToString(instance.Placement.AvailabilityZone)
	}

	return &types.AWSResource{
		ID:       aws.ToString(instance.InstanceId),
		Type:     "instance",
		Region:   c.cfg.Region,
		State:    string(instance.State.Name),
		Tags:     tags,
		Details:  details,
		LastSeen: time.Now(),
	}
}

// findDefaultSubnet finds a default subnet in the default VPC
func (c *Client) findDefaultSubnet(ctx context.Context) (string, error) {
	// First, find the default VPC
	vpcResult, err := c.ec2.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("isDefault"),
				Values: []string{"true"},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe VPCs: %w", err)
	}

	if len(vpcResult.Vpcs) == 0 {
		return "", fmt.Errorf("no default VPC found")
	}

	defaultVpcID := *vpcResult.Vpcs[0].VpcId

	// Find a subnet in the default VPC
	subnetResult, err := c.ec2.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{defaultVpcID},
			},
			{
				Name:   aws.String("default-for-az"),
				Values: []string{"true"},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe subnets: %w", err)
	}

	if len(subnetResult.Subnets) == 0 {
		return "", fmt.Errorf("no default subnet found in default VPC")
	}

	// Return the first available subnet
	for _, subnet := range subnetResult.Subnets {
		if subnet.State == ec2types.SubnetStateAvailable {
			return *subnet.SubnetId, nil
		}
	}

	return "", fmt.Errorf("no available default subnet found")
}

// DescribeInstances lists EC2 instances
func (c *Client) DescribeInstances(ctx context.Context) ([]*types.AWSResource, error) {
	result, err := c.ec2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe instances: %w", err)
	}

	var resources []*types.AWSResource
	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			resources = append(resources, c.convertEC2Instance(instance))
		}
	}

	return resources, nil
}

// ListEC2Instances is an alias for DescribeInstances for MCP compatibility
func (c *Client) ListEC2Instances(ctx context.Context) ([]*types.AWSResource, error) {
	return c.DescribeInstances(ctx)
}

// GetEC2Instance gets a specific EC2 instance by ID
func (c *Client) GetEC2Instance(ctx context.Context, instanceID string) (*types.AWSResource, error) {
	result, err := c.ec2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe instance %s: %w", instanceID, err)
	}

	if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
		return nil, fmt.Errorf("instance %s not found", instanceID)
	}

	return c.convertEC2Instance(result.Reservations[0].Instances[0]), nil
}

// CreateAMI creates an Amazon Machine Image from an EC2 instance
func (c *Client) CreateAMI(ctx context.Context, instanceID, name, description string) (*types.AWSResource, error) {
	input := &ec2.CreateImageInput{
		InstanceId:  aws.String(instanceID),
		Name:        aws.String(name),
		Description: aws.String(description),
		NoReboot:    aws.Bool(true), // Don't reboot the instance during AMI creation
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeImage,
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String(name)},
					{Key: aws.String("Source"), Value: aws.String(instanceID)},
					{Key: aws.String("Environment"), Value: aws.String("production-ready")},
					{Key: aws.String("CreatedBy"), Value: aws.String("aws-mcp-server")},
				},
			},
		},
	}

	result, err := c.ec2.CreateImage(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to create AMI from instance %s: %w", instanceID, err)
	}

	// Wait for AMI to be available (this can take several minutes)
	logrus.WithFields(logrus.Fields{
		"ami_id":      *result.ImageId,
		"instance_id": instanceID,
	}).Info("AMI creation initiated, waiting for completion...")

	// Create resource object
	resource := &types.AWSResource{
		ID:     *result.ImageId,
		Type:   "ami",
		Region: c.cfg.Region,
		State:  "pending",
		Tags:   make(map[string]string),
		Details: map[string]interface{}{
			"name":               name,
			"description":        description,
			"source_instance_id": instanceID,
		},
		LastSeen: time.Now(),
	}

	return resource, nil
}

// WaitForAMI waits for an AMI to become available
func (c *Client) WaitForAMI(ctx context.Context, amiID string) error {
	maxWaitTime := 30 * time.Minute
	pollInterval := 30 * time.Second

	ctxWithTimeout, cancel := context.WithTimeout(ctx, maxWaitTime)
	defer cancel()

	for {
		select {
		case <-ctxWithTimeout.Done():
			return fmt.Errorf("timeout waiting for AMI %s to become available", amiID)
		default:
			result, err := c.ec2.DescribeImages(ctx, &ec2.DescribeImagesInput{
				ImageIds: []string{amiID},
			})
			if err != nil {
				return fmt.Errorf("failed to describe AMI %s: %w", amiID, err)
			}

			if len(result.Images) == 0 {
				return fmt.Errorf("AMI %s not found", amiID)
			}

			state := result.Images[0].State
			logrus.WithFields(logrus.Fields{
				"ami_id": amiID,
				"state":  state,
			}).Info("AMI status check")

			switch state {
			case ec2types.ImageStateAvailable:
				return nil
			case ec2types.ImageStateFailed:
				return fmt.Errorf("AMI %s creation failed", amiID)
			case ec2types.ImageStatePending:
				time.Sleep(pollInterval)
			default:
				time.Sleep(pollInterval)
			}
		}
	}
}

// GetAvailabilityZones retrieves all available availability zones in the current region
func (c *Client) GetAvailabilityZones(ctx context.Context) ([]string, error) {
	result, err := c.ec2.DescribeAvailabilityZones(ctx, &ec2.DescribeAvailabilityZonesInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("state"),
				Values: []string{"available"},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe availability zones: %w", err)
	}

	var zones []string
	for _, az := range result.AvailabilityZones {
		if az.ZoneName != nil {
			zones = append(zones, *az.ZoneName)
		}
	}

	if len(zones) == 0 {
		// Fallback to common zones for us-east-1 if none found
		logrus.Warn("No availability zones found, using fallback zones")
		return []string{"us-east-1a", "us-east-1b", "us-east-1c"}, nil
	}

	return zones, nil
}

// ========== AMI Listing Methods ==========

// DescribeAMIs lists all AMIs owned by the account
func (c *Client) DescribeAMIs(ctx context.Context) ([]*types.AWSResource, error) {
	result, err := c.ec2.DescribeImages(ctx, &ec2.DescribeImagesInput{
		Owners: []string{"self"}, // Only show AMIs owned by this account
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe AMIs: %w", err)
	}

	var resources []*types.AWSResource
	for _, image := range result.Images {
		resources = append(resources, c.convertAMI(image))
	}

	return resources, nil
}

// DescribePublicAMIs lists public AMIs with optional filters
func (c *Client) DescribePublicAMIs(ctx context.Context, namePattern string) ([]*types.AWSResource, error) {
	input := &ec2.DescribeImagesInput{
		Owners: []string{"amazon"}, // Amazon-owned public AMIs
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("state"),
				Values: []string{"available"},
			},
			{
				Name:   aws.String("image-type"),
				Values: []string{"machine"},
			},
		},
	}

	if namePattern != "" {
		input.Filters = append(input.Filters, ec2types.Filter{
			Name:   aws.String("name"),
			Values: []string{namePattern},
		})
	}

	result, err := c.ec2.DescribeImages(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe public AMIs: %w", err)
	}

	var resources []*types.AWSResource
	for _, image := range result.Images {
		resources = append(resources, c.convertAMI(image))
	}

	return resources, nil
}

// GetAMI gets a specific AMI by ID
func (c *Client) GetAMI(ctx context.Context, amiID string) (*types.AWSResource, error) {
	result, err := c.ec2.DescribeImages(ctx, &ec2.DescribeImagesInput{
		ImageIds: []string{amiID},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe AMI %s: %w", amiID, err)
	}

	if len(result.Images) == 0 {
		return nil, fmt.Errorf("AMI %s not found", amiID)
	}

	return c.convertAMI(result.Images[0]), nil
}

// convertAMI converts an EC2 Image to our internal resource representation
func (c *Client) convertAMI(image ec2types.Image) *types.AWSResource {
	details := map[string]interface{}{
		"name":               aws.ToString(image.Name),
		"description":        aws.ToString(image.Description),
		"imageType":          string(image.ImageType),
		"kernelId":           aws.ToString(image.KernelId),
		"ramdiskId":          aws.ToString(image.RamdiskId),
		"platform":           string(image.Platform),
		"platformDetails":    aws.ToString(image.PlatformDetails),
		"usageOperation":     aws.ToString(image.UsageOperation),
		"architecture":       string(image.Architecture),
		"creationDate":       aws.ToString(image.CreationDate),
		"imageLocation":      aws.ToString(image.ImageLocation),
		"imageOwnerAlias":    aws.ToString(image.ImageOwnerAlias),
		"ownerId":            aws.ToString(image.OwnerId),
		"rootDeviceName":     aws.ToString(image.RootDeviceName),
		"rootDeviceType":     string(image.RootDeviceType),
		"sriovNetSupport":    aws.ToString(image.SriovNetSupport),
		"virtualizationType": string(image.VirtualizationType),
		"hypervisor":         string(image.Hypervisor),
		"public":             aws.ToBool(image.Public),
		"deprecationTime":    aws.ToString(image.DeprecationTime),
	}

	// Add block device mappings
	if len(image.BlockDeviceMappings) > 0 {
		var mappings []map[string]interface{}
		for _, bdm := range image.BlockDeviceMappings {
			mapping := map[string]interface{}{
				"deviceName":  aws.ToString(bdm.DeviceName),
				"virtualName": aws.ToString(bdm.VirtualName),
			}
			if bdm.Ebs != nil {
				mapping["ebs"] = map[string]interface{}{
					"volumeSize":          aws.ToInt32(bdm.Ebs.VolumeSize),
					"volumeType":          string(bdm.Ebs.VolumeType),
					"deleteOnTermination": aws.ToBool(bdm.Ebs.DeleteOnTermination),
					"encrypted":           aws.ToBool(bdm.Ebs.Encrypted),
					"snapshotId":          aws.ToString(bdm.Ebs.SnapshotId),
					"kmsKeyId":            aws.ToString(bdm.Ebs.KmsKeyId),
					"iops":                aws.ToInt32(bdm.Ebs.Iops),
					"throughput":          aws.ToInt32(bdm.Ebs.Throughput),
				}
			}
			mappings = append(mappings, mapping)
		}
		details["blockDeviceMappings"] = mappings
	}

	return &types.AWSResource{
		ID:       aws.ToString(image.ImageId),
		Type:     "ami",
		Region:   c.cfg.Region,
		State:    string(image.State),
		Tags:     make(map[string]string), // Tags need to be fetched separately or converted from image.Tags
		Details:  details,
		LastSeen: time.Now(),
	}
}

// GetLatestAmazonLinux2AMI finds the latest Amazon Linux 2 AMI in the current region
func (c *Client) GetLatestAmazonLinux2AMI(ctx context.Context) (string, error) {
	input := &ec2.DescribeImagesInput{
		Owners: []string{"amazon"},
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("name"),
				Values: []string{"amzn2-ami-hvm-*-x86_64-gp2"},
			},
			{
				Name:   aws.String("state"),
				Values: []string{"available"},
			},
		},
	}

	result, err := c.ec2.DescribeImages(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to describe AMIs: %w", err)
	}

	if len(result.Images) == 0 {
		return "", fmt.Errorf("no Amazon Linux 2 AMIs found")
	}

	// Find the most recent AMI by creation date
	var latestAMI ec2types.Image
	var latestTime time.Time

	for _, image := range result.Images {
		if image.CreationDate == nil {
			continue
		}

		creationTime, err := time.Parse(time.RFC3339, *image.CreationDate)
		if err != nil {
			c.logger.WithError(err).WithField("ami", *image.ImageId).Warn("Failed to parse AMI creation date")
			continue
		}

		if creationTime.After(latestTime) {
			latestTime = creationTime
			latestAMI = image
		}
	}

	if latestAMI.ImageId == nil {
		return "", fmt.Errorf("no valid Amazon Linux 2 AMI found")
	}

	c.logger.WithFields(logrus.Fields{
		"amiId":        *latestAMI.ImageId,
		"name":         aws.ToString(latestAMI.Name),
		"creationDate": aws.ToString(latestAMI.CreationDate),
	}).Info("Found latest Amazon Linux 2 AMI")

	return *latestAMI.ImageId, nil
}
