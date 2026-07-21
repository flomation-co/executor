// Package aws_rds_modify_db_cluster modifies settings of an Aurora/RDS DB cluster.
package aws_rds_modify_db_cluster

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	rdscat "flomation.app/automate/executor/actions/aws/rds"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Modify DB Cluster"
	Description  = "Modify an Aurora/RDS DB cluster (version, port, password or backup retention)."
	Website      = "https://www.flomation.co"
	Icon         = "circle-nodes+pen"
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
	{Name: "db_cluster_identifier", Type: core.ConnectionTypeString, Label: "DB Cluster Identifier", Placeholder: "my-aurora-cluster", Required: true},
	{Name: "master_password", Type: core.ConnectionTypeSecret, Label: "New Master Password (optional)"},
	{Name: "engine_version", Type: core.ConnectionTypeString, Label: "New Engine Version (optional)"},
	{Name: "port", Type: core.ConnectionTypeInteger, Label: "New Port (optional)"},
	{Name: "backup_retention_period", Type: core.ConnectionTypeInteger, Label: "Backup Retention (days, optional)"},
	{Name: "vpc_security_group_ids", Type: core.ConnectionTypeString, Label: "New VPC Security Group IDs (optional)", Placeholder: "Comma-separated; replaces the current set"},
	{Name: "deletion_protection", Type: core.ConnectionTypeString, Label: "Deletion Protection", Options: []core.ConnectionOption{
		{Name: "No change", Value: ""},
		{Name: "Enable", Value: "true"},
		{Name: "Disable", Value: "false"},
	}},
	{Name: "serverless_v2_min_capacity", Type: core.ConnectionTypeString, Label: "Serverless v2 Min ACU (optional)", Placeholder: "e.g. 0.5"},
	{Name: "serverless_v2_max_capacity", Type: core.ConnectionTypeString, Label: "Serverless v2 Max ACU (optional)", Placeholder: "e.g. 16"},
	{Name: "apply_immediately", Type: core.ConnectionTypeBoolean, Label: "Apply Immediately"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "cluster", Type: core.ConnectionTypeObject, Label: "DB Cluster"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	id := awscommon.InputString("db_cluster_identifier", inputs)
	if id == "" {
		return nil, fmt.Errorf("db cluster identifier is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.ModifyDBClusterInput{
		DBClusterIdentifier: aws.String(id),
		ApplyImmediately:    aws.Bool(awscommon.InputBool("apply_immediately", inputs)),
	}
	changed := 0
	if v := awscommon.InputString("master_password", inputs); v != "" {
		in.MasterUserPassword = aws.String(v)
		changed++
	}
	if v := awscommon.InputString("engine_version", inputs); v != "" {
		in.EngineVersion = aws.String(v)
		changed++
	}
	if p, ok := awscommon.InputInt("port", inputs); ok {
		in.Port = aws.Int32(int32(p))
		changed++
	}
	if d, ok := awscommon.InputInt("backup_retention_period", inputs); ok {
		in.BackupRetentionPeriod = aws.Int32(int32(d))
		changed++
	}
	if ids := awscommon.InputStrings("vpc_security_group_ids", inputs); len(ids) > 0 {
		in.VpcSecurityGroupIds = ids
		changed++
	}
	if v := awscommon.InputString("deletion_protection", inputs); v != "" {
		in.DeletionProtection = aws.Bool(v == "true")
		changed++
	}
	if sv2 := rdscat.BuildServerlessV2(inputs); sv2 != nil {
		in.ServerlessV2ScalingConfiguration = sv2
		changed++
	}
	if changed == 0 {
		return nil, fmt.Errorf("provide at least one setting to modify (password, engine version, port or backup retention)")
	}

	out, err := client.ModifyDBCluster(ctx, in)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Modifying DB cluster %q (%d change(s), status: %s)", id, changed, aws.ToString(out.DBCluster.Status)),
		"cluster":     rdscat.SummariseCluster(out.DBCluster),
	}, nil
}
