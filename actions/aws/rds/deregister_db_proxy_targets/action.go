// Package aws_rds_deregister_db_proxy_targets removes DB instances and/or DB
// clusters from an RDS Proxy target group.
package aws_rds_deregister_db_proxy_targets

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
	Name         = "AWS RDS Deregister DB Proxy Targets"
	Description  = "Remove DB instances or clusters from an RDS Proxy target group."
	Website      = "https://www.flomation.co"
	Icon         = "route+minus"
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
	{Name: "db_proxy_name", Type: core.ConnectionTypeString, Label: "DB Proxy Name", Placeholder: "my-proxy", Required: true},
	{Name: "target_group_name", Type: core.ConnectionTypeString, Label: "Target Group Name", Placeholder: "default"},
	{Name: "db_instance_identifiers", Type: core.ConnectionTypeString, Label: "DB Instance Identifiers (optional)", Placeholder: "Comma-separated"},
	{Name: "db_cluster_identifiers", Type: core.ConnectionTypeString, Label: "DB Cluster Identifiers (optional)", Placeholder: "Comma-separated"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := awscommon.InputString("db_proxy_name", inputs)
	if name == "" {
		return nil, fmt.Errorf("db proxy name is required")
	}

	instanceIDs := awscommon.InputStrings("db_instance_identifiers", inputs)
	clusterIDs := awscommon.InputStrings("db_cluster_identifiers", inputs)
	if len(instanceIDs) == 0 && len(clusterIDs) == 0 {
		return nil, fmt.Errorf("at least one DB instance identifier or DB cluster identifier is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.DeregisterDBProxyTargetsInput{DBProxyName: aws.String(name)}
	if group := awscommon.InputString("target_group_name", inputs); group != "" {
		in.TargetGroupName = aws.String(group)
	}
	if len(instanceIDs) > 0 {
		in.DBInstanceIdentifiers = instanceIDs
	}
	if len(clusterIDs) > 0 {
		in.DBClusterIdentifiers = clusterIDs
	}

	if _, err := client.DeregisterDBProxyTargets(ctx, in); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Deregistered %d target(s) from DB proxy %q", len(instanceIDs)+len(clusterIDs), name),
	}, nil
}
