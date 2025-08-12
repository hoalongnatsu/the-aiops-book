package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"aws-mcp-server/pkg/types"

	"github.com/sirupsen/logrus"
)

type Client struct {
	cfg    aws.Config
	ec2    *ec2.Client
	logger *logrus.Logger
}

func NewClient(region, profile string, logger *logrus.Logger) (*Client, error) {
	cfg, err := config.LoadDefaultConfig(
		context.Background(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &Client{
		cfg:    cfg,
		ec2:    ec2.NewFromConfig(cfg),
		logger: logger,
	}, nil
}

// HealthCheck verifies AWS connectivity
func (c *Client) HealthCheck(ctx context.Context) error {
	_, err := c.ec2.DescribeRegions(ctx, &ec2.DescribeRegionsInput{})
	if err != nil {
		return fmt.Errorf("AWS health check failed: %w", err)
	}
	return nil
}

// ListEC2Instances retrieves all EC2 instances in the region
func (c *Client) ListEC2Instances(ctx context.Context) ([]types.AWSResource, error) {
	start := time.Now()

	result, err := c.ec2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{})
	if err != nil {
		c.logger.WithError(err).Error("Failed to describe EC2 instances")
		return nil, fmt.Errorf("failed to describe instances: %w", err)
	}

	var resources []types.AWSResource
	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			resource := c.convertEC2Instance(instance)
			resources = append(resources, resource)
		}
	}

	c.logger.WithFields(logrus.Fields{
		"count":    len(resources),
		"duration": time.Since(start),
	}).Info("Retrieved EC2 instances")

	return resources, nil
}

// GetEC2Instance retrieves a specific EC2 instance
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

	instance := result.Reservations[0].Instances[0]
	resource := c.convertEC2Instance(instance)

	return &resource, nil
}

// convertEC2Instance converts AWS EC2 instance to our standard format
func (c *Client) convertEC2Instance(instance ec2types.Instance) types.AWSResource {
	tags := make(map[string]string)
	for _, tag := range instance.Tags {
		if tag.Key != nil && tag.Value != nil {
			tags[*tag.Key] = *tag.Value
		}
	}

	details := map[string]interface{}{
		"instanceType": string(instance.InstanceType),
		"placement":    instance.Placement,
		"launchTime":   instance.LaunchTime,
	}

	if instance.PublicIpAddress != nil {
		details["publicIpAddress"] = *instance.PublicIpAddress
	}

	if instance.PrivateIpAddress != nil {
		details["privateIpAddress"] = *instance.PrivateIpAddress
	}

	var instanceID string
	if instance.InstanceId != nil {
		instanceID = *instance.InstanceId
	}

	return types.AWSResource{
		ID:       instanceID,
		Type:     "ec2-instance",
		Region:   c.cfg.Region,
		State:    string(instance.State.Name),
		Tags:     tags,
		Details:  details,
		LastSeen: time.Now(),
	}
}
