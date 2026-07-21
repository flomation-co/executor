// Package aws_rds_describe_db_shard_groups lists Aurora Limitless DB shard
// groups, optionally narrowed to a single identifier.
package aws_rds_describe_db_shard_groups

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Describe DB Shard Groups"
	Description  = "List Aurora Limitless DB shard groups, optionally filtered by identifier."
	Website      = "https://www.flomation.co"
	Icon         = "server+magnifying-glass"
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
	{Name: "db_shard_group_identifier", Type: core.ConnectionTypeString, Label: "DB Shard Group Identifier (optional)", Placeholder: "Leave blank to list all"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "shard_groups", Type: core.ConnectionTypeObject, Label: "DB Shard Groups"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.DescribeDBShardGroupsInput{}
	if id := awscommon.InputString("db_shard_group_identifier", inputs); id != "" {
		in.DBShardGroupIdentifier = aws.String(id)
	}

	var shardGroups []map[string]interface{}
	for {
		page, err := client.DescribeDBShardGroups(ctx, in)
		if err != nil {
			return nil, err
		}
		for i := range page.DBShardGroups {
			shardGroups = append(shardGroups, flattenShardGroup(&page.DBShardGroups[i]))
		}
		if page.Marker == nil || aws.ToString(page.Marker) == "" {
			break
		}
		in.Marker = page.Marker
	}

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Found %d DB shard group(s)", len(shardGroups)),
		"shard_groups": shardGroups,
		"count":        len(shardGroups),
	}, nil
}

func flattenShardGroup(in *rdstypes.DBShardGroup) map[string]interface{} {
	return map[string]interface{}{
		"db_shard_group_identifier": aws.ToString(in.DBShardGroupIdentifier),
		"db_cluster_identifier":     aws.ToString(in.DBClusterIdentifier),
		"status":                    aws.ToString(in.Status),
		"endpoint":                  aws.ToString(in.Endpoint),
		"max_acu":                   aws.ToFloat64(in.MaxACU),
	}
}
