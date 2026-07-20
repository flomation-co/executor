// Package aws_rds_modify_global_cluster modifies an Aurora global database cluster.
package aws_rds_modify_global_cluster

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Modify Global Cluster"
	Description  = "Modify an Aurora global database cluster (rename, engine version, deletion protection)."
	Website      = "https://www.flomation.co"
	Icon         = "globe+pen"
	Date         = "20/07/2026"
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
	{Name: "global_cluster_identifier", Type: core.ConnectionTypeString, Label: "Global Cluster Identifier", Placeholder: "my-global-database", Required: true},
	{Name: "new_global_cluster_identifier", Type: core.ConnectionTypeString, Label: "New Global Cluster Identifier (optional)", Placeholder: "Rename the global cluster"},
	{Name: "engine_version", Type: core.ConnectionTypeString, Label: "Engine Version (optional)"},
	{Name: "deletion_protection", Type: core.ConnectionTypeString, Label: "Deletion Protection (optional)", Options: []core.ConnectionOption{
		{Name: "Leave unchanged", Value: ""},
		{Name: "Enabled", Value: "true"},
		{Name: "Disabled", Value: "false"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "global_cluster", Type: core.ConnectionTypeObject, Label: "Global Cluster"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	id := awscommon.InputString("global_cluster_identifier", inputs)
	if id == "" {
		return nil, fmt.Errorf("global cluster identifier is required")
	}

	in := &rds.ModifyGlobalClusterInput{GlobalClusterIdentifier: aws.String(id)}
	changed := false
	if v := awscommon.InputString("new_global_cluster_identifier", inputs); v != "" {
		in.NewGlobalClusterIdentifier = aws.String(v)
		changed = true
	}
	if v := awscommon.InputString("engine_version", inputs); v != "" {
		in.EngineVersion = aws.String(v)
		changed = true
	}
	switch strings.ToLower(strings.TrimSpace(awscommon.InputString("deletion_protection", inputs))) {
	case "true":
		in.DeletionProtection = aws.Bool(true)
		changed = true
	case "false":
		in.DeletionProtection = aws.Bool(false)
		changed = true
	}
	if !changed {
		return nil, fmt.Errorf("at least one change must be specified")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	out, err := client.ModifyGlobalCluster(ctx, in)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result":    fmt.Sprintf("Modified global cluster %q (status: %s)", id, aws.ToString(out.GlobalCluster.Status)),
		"global_cluster": flattenGlobalCluster(out.GlobalCluster),
	}, nil
}

func flattenGlobalCluster(gc *rdstypes.GlobalCluster) map[string]interface{} {
	if gc == nil {
		return nil
	}
	var members []map[string]interface{}
	for _, m := range gc.GlobalClusterMembers {
		members = append(members, map[string]interface{}{
			"db_cluster_arn": aws.ToString(m.DBClusterArn),
			"is_writer":      aws.ToBool(m.IsWriter),
		})
	}
	return map[string]interface{}{
		"global_cluster_identifier": aws.ToString(gc.GlobalClusterIdentifier),
		"status":                    aws.ToString(gc.Status),
		"engine":                    aws.ToString(gc.Engine),
		"engine_version":            aws.ToString(gc.EngineVersion),
		"members":                   members,
	}
}
