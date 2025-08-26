package mcp

import (
	"aws-mcp-server/pkg/aws"
	"context"
	"fmt"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"
)

// ========== Interface defines ==========

// RDSToolsInterface defines all available RDS database tools
// Following Single Responsibility Principle - each tool manages one specific RDS resource type
//
// Available Tools:
//   - listDBInstances()           : List all RDS database instances in the region
//   - listDBSnapshots()           : List all RDS database snapshots in the region
//   - createDBSubnetGroup()       : Create a database subnet group (aws_db_subnet_group equivalent)
//   - createDBInstance()          : Create an RDS database instance (aws_db_instance equivalent)
//   - startDBInstance()           : Start a stopped RDS database instance
//   - stopDBInstance()            : Stop a running RDS database instance
//   - deleteDBInstance()          : Delete an RDS database instance (with optional final snapshot)
//   - createDBSnapshot()          : Create a manual snapshot of an RDS database instance
//
// Usage Example (Terraform-like workflow):
//   1. createDBSubnetGroup(name="my-db-subnet-group", subnetIds=["subnet-xxx", "subnet-yyy"])
//   2. createDBInstance(dbInstanceIdentifier="my-database", engine="mysql", dbInstanceClass="db.t3.micro")
//   3. createDBSnapshot(dbInstanceIdentifier="my-database", dbSnapshotIdentifier="my-backup")
//   4. stopDBInstance(dbInstanceIdentifier="my-database")

// ========== RDS Tools ==========

// listDBInstances lists all RDS database instances
func (h *ToolHandler) listDBInstances(ctx context.Context) (*mcp.CallToolResult, error) {
	result, err := h.resourceHandler.ReadResource(ctx, "aws://rds/instances")
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to list RDS instances: %s", err.Error()))
	}

	if len(result.Contents) > 0 {
		if textContent, ok := result.Contents[0].(*mcp.TextResourceContents); ok {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Type: "text",
						Text: textContent.Text,
					},
				},
			}, nil
		}
	}

	return h.createErrorResponse("No data returned from resource")
}

// listDBSnapshots lists all RDS database snapshots
func (h *ToolHandler) listDBSnapshots(ctx context.Context) (*mcp.CallToolResult, error) {
	result, err := h.resourceHandler.ReadResource(ctx, "aws://rds/snapshots")
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to list RDS snapshots: %s", err.Error()))
	}

	if len(result.Contents) > 0 {
		if textContent, ok := result.Contents[0].(*mcp.TextResourceContents); ok {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Type: "text",
						Text: textContent.Text,
					},
				},
			}, nil
		}
	}

	return h.createErrorResponse("No data returned from resource")
}

// createDBSubnetGroup creates a database subnet group
func (h *ToolHandler) createDBSubnetGroup(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// Extract required parameters
	name, ok := arguments["dbSubnetGroupName"].(string)
	if !ok || name == "" {
		return h.createErrorResponse("dbSubnetGroupName is required and must be a string")
	}

	description, ok := arguments["dbSubnetGroupDescription"].(string)
	if !ok || description == "" {
		return h.createErrorResponse("dbSubnetGroupDescription is required and must be a string")
	}

	subnetIDsInterface, ok := arguments["subnetIds"].([]interface{})
	if !ok || len(subnetIDsInterface) == 0 {
		return h.createErrorResponse("subnetIds is required and must be a non-empty array")
	}

	// Convert subnet IDs
	var subnetIDs []string
	for _, id := range subnetIDsInterface {
		if idStr, ok := id.(string); ok {
			subnetIDs = append(subnetIDs, idStr)
		}
	}

	if len(subnetIDs) == 0 {
		return h.createErrorResponse("subnetIds must contain valid subnet ID strings")
	}

	params := aws.CreateDBSubnetGroupParams{
		DBSubnetGroupName:        name,
		DBSubnetGroupDescription: description,
		SubnetIDs:                subnetIDs,
		Tags:                     make(map[string]string),
	}

	// Add optional tags
	if tags, exists := arguments["tags"].(map[string]interface{}); exists {
		for k, v := range tags {
			if strVal, ok := v.(string); ok {
				params.Tags[k] = strVal
			}
		}
	}

	result, err := h.awsClient.CreateDBSubnetGroup(ctx, params)
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to create DB subnet group: %s", err.Error()))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("DB subnet group '%s' created successfully in VPC '%s' with %d subnets",
					*result.DBSubnetGroupName, *result.VpcId, len(result.Subnets)),
			},
		},
	}, nil
}

// createDBInstance creates a new RDS database instance
func (h *ToolHandler) createDBInstance(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// Extract required parameters
	dbInstanceIdentifier, ok := arguments["dbInstanceIdentifier"].(string)
	if !ok || dbInstanceIdentifier == "" {
		return h.createErrorResponse("dbInstanceIdentifier is required and must be a string")
	}

	dbInstanceClass, ok := arguments["dbInstanceClass"].(string)
	if !ok || dbInstanceClass == "" {
		return h.createErrorResponse("dbInstanceClass is required and must be a string")
	}

	engine, ok := arguments["engine"].(string)
	if !ok || engine == "" {
		return h.createErrorResponse("engine is required and must be a string")
	}

	masterUsername, ok := arguments["masterUsername"].(string)
	if !ok || masterUsername == "" {
		return h.createErrorResponse("masterUsername is required and must be a string")
	}

	masterUserPassword, ok := arguments["masterUserPassword"].(string)
	if !ok || masterUserPassword == "" {
		return h.createErrorResponse("masterUserPassword is required and must be a string")
	}

	allocatedStorageInterface, ok := arguments["allocatedStorage"]
	if !ok {
		return h.createErrorResponse("allocatedStorage is required")
	}

	// Handle allocatedStorage conversion
	var allocatedStorage int32
	switch v := allocatedStorageInterface.(type) {
	case float64:
		allocatedStorage = int32(v)
	case int:
		allocatedStorage = int32(v)
	case int32:
		allocatedStorage = v
	case string:
		if parsed, err := strconv.ParseInt(v, 10, 32); err == nil {
			allocatedStorage = int32(parsed)
		} else {
			return h.createErrorResponse("allocatedStorage must be a valid integer")
		}
	default:
		return h.createErrorResponse("allocatedStorage must be a number")
	}

	params := aws.CreateDBInstanceParams{
		DBInstanceIdentifier: dbInstanceIdentifier,
		DBInstanceClass:      dbInstanceClass,
		Engine:               engine,
		MasterUsername:       masterUsername,
		MasterUserPassword:   masterUserPassword,
		AllocatedStorage:     allocatedStorage,
		Tags:                 make(map[string]string),
	}

	// Extract optional parameters
	if engineVersion, ok := arguments["engineVersion"].(string); ok {
		params.EngineVersion = engineVersion
	}
	if storageType, ok := arguments["storageType"].(string); ok {
		params.StorageType = storageType
	}
	if storageEncrypted, ok := arguments["storageEncrypted"].(bool); ok {
		params.StorageEncrypted = storageEncrypted
	}
	if dbSubnetGroupName, ok := arguments["dbSubnetGroupName"].(string); ok {
		params.DBSubnetGroupName = dbSubnetGroupName
	}
	if multiAZ, ok := arguments["multiAZ"].(bool); ok {
		params.MultiAZ = multiAZ
	}
	if publiclyAccessible, ok := arguments["publiclyAccessible"].(bool); ok {
		params.PubliclyAccessible = publiclyAccessible
	}

	// Handle VPC security group IDs
	if vpcSecurityGroupIDsInterface, ok := arguments["vpcSecurityGroupIds"].([]interface{}); ok {
		var vpcSecurityGroupIDs []string
		for _, id := range vpcSecurityGroupIDsInterface {
			if idStr, ok := id.(string); ok {
				vpcSecurityGroupIDs = append(vpcSecurityGroupIDs, idStr)
			}
		}
		params.VpcSecurityGroupIDs = vpcSecurityGroupIDs
	}

	// Handle backup retention period
	if backupRetentionInterface, ok := arguments["backupRetentionPeriod"]; ok {
		switch v := backupRetentionInterface.(type) {
		case float64:
			params.BackupRetentionPeriod = int32(v)
		case int:
			params.BackupRetentionPeriod = int32(v)
		case int32:
			params.BackupRetentionPeriod = v
		}
	}

	if preferredBackupWindow, ok := arguments["preferredBackupWindow"].(string); ok {
		params.PreferredBackupWindow = preferredBackupWindow
	}
	if preferredMaintenanceWindow, ok := arguments["preferredMaintenanceWindow"].(string); ok {
		params.PreferredMaintenanceWindow = preferredMaintenanceWindow
	}

	// Add optional tags
	if tags, exists := arguments["tags"].(map[string]interface{}); exists {
		for k, v := range tags {
			if strVal, ok := v.(string); ok {
				params.Tags[k] = strVal
			}
		}
	}

	result, err := h.awsClient.CreateDBInstance(ctx, params)
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to create DB instance: %s", err.Error()))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("DB instance '%s' creation initiated. Engine: %s, Class: %s, Storage: %dGB",
					*result.DBInstanceIdentifier, *result.Engine, *result.DBInstanceClass, *result.AllocatedStorage),
			},
		},
	}, nil
}

// startDBInstance starts a stopped RDS database instance
func (h *ToolHandler) startDBInstance(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	dbInstanceIdentifier, ok := arguments["dbInstanceIdentifier"].(string)
	if !ok || dbInstanceIdentifier == "" {
		return h.createErrorResponse("dbInstanceIdentifier is required and must be a string")
	}

	err := h.awsClient.StartDBInstance(ctx, dbInstanceIdentifier)
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to start DB instance: %s", err.Error()))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("DB instance '%s' start initiated successfully", dbInstanceIdentifier),
			},
		},
	}, nil
}

// stopDBInstance stops a running RDS database instance
func (h *ToolHandler) stopDBInstance(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	dbInstanceIdentifier, ok := arguments["dbInstanceIdentifier"].(string)
	if !ok || dbInstanceIdentifier == "" {
		return h.createErrorResponse("dbInstanceIdentifier is required and must be a string")
	}

	err := h.awsClient.StopDBInstance(ctx, dbInstanceIdentifier)
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to stop DB instance: %s", err.Error()))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("DB instance '%s' stop initiated successfully", dbInstanceIdentifier),
			},
		},
	}, nil
}

// deleteDBInstance deletes an RDS database instance
func (h *ToolHandler) deleteDBInstance(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	dbInstanceIdentifier, ok := arguments["dbInstanceIdentifier"].(string)
	if !ok || dbInstanceIdentifier == "" {
		return h.createErrorResponse("dbInstanceIdentifier is required and must be a string")
	}

	// Default to skip final snapshot unless specified
	skipFinalSnapshot := true
	if skip, ok := arguments["skipFinalSnapshot"].(bool); ok {
		skipFinalSnapshot = skip
	}

	finalSnapshotIdentifier := ""
	if !skipFinalSnapshot {
		if snapshot, ok := arguments["finalDBSnapshotIdentifier"].(string); ok {
			finalSnapshotIdentifier = snapshot
		} else {
			return h.createErrorResponse("finalDBSnapshotIdentifier is required when skipFinalSnapshot is false")
		}
	}

	err := h.awsClient.DeleteDBInstance(ctx, dbInstanceIdentifier, skipFinalSnapshot, finalSnapshotIdentifier)
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to delete DB instance: %s", err.Error()))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("DB instance '%s' deletion initiated successfully", dbInstanceIdentifier),
			},
		},
	}, nil
}

// createDBSnapshot creates a snapshot of an RDS database instance
func (h *ToolHandler) createDBSnapshot(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	dbInstanceIdentifier, ok := arguments["dbInstanceIdentifier"].(string)
	if !ok || dbInstanceIdentifier == "" {
		return h.createErrorResponse("dbInstanceIdentifier is required and must be a string")
	}

	dbSnapshotIdentifier, ok := arguments["dbSnapshotIdentifier"].(string)
	if !ok || dbSnapshotIdentifier == "" {
		return h.createErrorResponse("dbSnapshotIdentifier is required and must be a string")
	}

	params := aws.CreateDBSnapshotParams{
		DBInstanceIdentifier: dbInstanceIdentifier,
		DBSnapshotIdentifier: dbSnapshotIdentifier,
		Tags:                 make(map[string]string),
	}

	// Add optional tags
	if tags, exists := arguments["tags"].(map[string]interface{}); exists {
		for k, v := range tags {
			if strVal, ok := v.(string); ok {
				params.Tags[k] = strVal
			}
		}
	}

	result, err := h.awsClient.CreateDBSnapshot(ctx, params)
	if err != nil {
		return h.createErrorResponse(fmt.Sprintf("Failed to create DB snapshot: %s", err.Error()))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("DB snapshot '%s' creation initiated for instance '%s'",
					*result.DBSnapshotIdentifier, *result.DBInstanceIdentifier),
			},
		},
	}, nil
}
