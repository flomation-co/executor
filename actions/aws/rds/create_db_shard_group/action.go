// Package aws_rds_create_db_shard_group creates an Aurora Limitless DB shard group.
package aws_rds_create_db_shard_group

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Create DB Shard Group"
	Description  = "Create an Aurora Limitless DB shard group on a DB cluster."
	Website      = "https://www.flomation.co"
	Icon         = "server+plus"
	Date         = "21/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Required: true, Options: []core.ConnectionOption{
		{Name: "Access Keys", Value: "keys"},
		{Name: "Assume Role (cross-account)", Value: "assume_role"},
		{Name: "Managed Role (Credential)", Value: "credential"},
	}},
	{Name: "aws_access_key", Type: core.ConnectionTypeSecret, Label: "AWS Access Key", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "aws_secret_key", Type: core.ConnectionTypeSecret, Label: "AWS Secret Key", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "aws_region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "eu-west-2", Required: true},
	{Name: "aws_session_token", Type: core.ConnectionTypeSecret, Label: "Session Token (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "assume_role_arn", Type: core.ConnectionTypeString, Label: "Role ARN to Assume", Placeholder: "arn:aws:iam::<your-account>:role/FlomationAccess", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"assume_role"}}},
	{Name: "external_id", Type: core.ConnectionTypeString, Label: "Assume Role External ID (optional)", Placeholder: "Must match the External ID in the role's trust policy", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"assume_role"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "AWS Role Credential", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"credential"}}},
	{Name: "db_shard_group_identifier", Type: core.ConnectionTypeString, Label: "DB Shard Group Identifier", Placeholder: "my-shard-group", Required: true},
	{Name: "db_cluster_identifier", Type: core.ConnectionTypeString, Label: "DB Cluster Identifier", Placeholder: "my-limitless-cluster", Required: true},
	{Name: "max_acu", Type: core.ConnectionTypeString, Label: "Maximum Capacity (ACUs)", Placeholder: "e.g. 64", Required: true},
	{Name: "min_acu", Type: core.ConnectionTypeString, Label: "Minimum Capacity (ACUs, optional)", Placeholder: "e.g. 0.5"},
	{Name: "compute_redundancy", Type: core.ConnectionTypeInteger, Label: "Compute Redundancy (optional)", Placeholder: "0, 1 or 2"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "shard_group", Type: core.ConnectionTypeObject, Label: "DB Shard Group"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	shardID := awscommon.InputString("db_shard_group_identifier", inputs)
	clusterID := awscommon.InputString("db_cluster_identifier", inputs)
	if shardID == "" || clusterID == "" {
		return nil, fmt.Errorf("both db shard group identifier and db cluster identifier are required")
	}
	maxACU, ok := awscommon.InputFloat("max_acu", inputs)
	if !ok {
		return nil, fmt.Errorf("maximum capacity (max_acu) is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.CreateDBShardGroupInput{
		DBShardGroupIdentifier: aws.String(shardID),
		DBClusterIdentifier:    aws.String(clusterID),
		MaxACU:                 aws.Float64(maxACU),
	}
	if minACU, ok := awscommon.InputFloat("min_acu", inputs); ok {
		in.MinACU = aws.Float64(minACU)
	}
	if cr, ok := awscommon.InputInt("compute_redundancy", inputs); ok {
		in.ComputeRedundancy = aws.Int32(int32(cr))
	}

	out, err := client.CreateDBShardGroup(ctx, in)
	if err != nil {
		return nil, err
	}

	shardGroup := map[string]interface{}{
		"db_shard_group_identifier": aws.ToString(out.DBShardGroupIdentifier),
		"db_cluster_identifier":     aws.ToString(out.DBClusterIdentifier),
		"status":                    aws.ToString(out.Status),
		"endpoint":                  aws.ToString(out.Endpoint),
		"max_acu":                   aws.ToFloat64(out.MaxACU),
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Creating DB shard group %q on cluster %q (status: %s)", shardID, clusterID, aws.ToString(out.Status)),
		"shard_group": shardGroup,
	}, nil
}
