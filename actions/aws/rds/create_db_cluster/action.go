// Package aws_rds_create_db_cluster provisions a new Aurora/RDS DB cluster.
package aws_rds_create_db_cluster

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	rdscat "flomation.app/automate/executor/actions/aws/rds"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Create DB Cluster"
	Description  = "Provision a new Aurora/RDS DB cluster. Add instances to it afterwards."
	Website      = "https://www.flomation.co"
	Icon         = "circle-nodes+plus"
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
	{Name: "engine", Type: core.ConnectionTypeString, Label: "Engine", Required: true, Options: []core.ConnectionOption{
		{Name: "Aurora MySQL", Value: "aurora-mysql"},
		{Name: "Aurora PostgreSQL", Value: "aurora-postgresql"},
		{Name: "MySQL (Multi-AZ cluster)", Value: "mysql"},
		{Name: "PostgreSQL (Multi-AZ cluster)", Value: "postgres"},
	}},
	{Name: "master_username", Type: core.ConnectionTypeString, Label: "Master Username", Placeholder: "admin", Required: true},
	{Name: "master_password", Type: core.ConnectionTypeSecret, Label: "Master Password", Required: true},
	{Name: "database_name", Type: core.ConnectionTypeString, Label: "Initial Database Name (optional)"},
	{Name: "engine_version", Type: core.ConnectionTypeString, Label: "Engine Version (optional)"},
	{Name: "port", Type: core.ConnectionTypeInteger, Label: "Port (optional)", Placeholder: "Engine default"},
	{Name: "db_subnet_group_name", Type: core.ConnectionTypeString, Label: "DB Subnet Group (optional)", Placeholder: "Places the cluster in a specific VPC"},
	{Name: "vpc_security_group_ids", Type: core.ConnectionTypeString, Label: "VPC Security Group IDs (optional)", Placeholder: "Comma-separated, e.g. sg-0abc,sg-0def"},
	{Name: "storage_encrypted", Type: core.ConnectionTypeBoolean, Label: "Encrypt Storage at Rest"},
	{Name: "kms_key_id", Type: core.ConnectionTypeString, Label: "KMS Key ID/ARN (optional)"},
	{Name: "backup_retention_period", Type: core.ConnectionTypeInteger, Label: "Backup Retention (days, optional)", Placeholder: "1-35"},
	{Name: "preferred_backup_window", Type: core.ConnectionTypeString, Label: "Preferred Backup Window (optional)", Placeholder: "hh24:mi-hh24:mi UTC"},
	{Name: "deletion_protection", Type: core.ConnectionTypeBoolean, Label: "Deletion Protection"},
	{Name: "serverless_v2_min_capacity", Type: core.ConnectionTypeString, Label: "Serverless v2 Min ACU (optional)", Placeholder: "e.g. 0.5"},
	{Name: "serverless_v2_max_capacity", Type: core.ConnectionTypeString, Label: "Serverless v2 Max ACU (optional)", Placeholder: "e.g. 16"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "cluster", Type: core.ConnectionTypeObject, Label: "DB Cluster"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	id := awscommon.InputString("db_cluster_identifier", inputs)
	engine := awscommon.InputString("engine", inputs)
	username := awscommon.InputString("master_username", inputs)
	password := awscommon.InputString("master_password", inputs)
	if id == "" || engine == "" || username == "" || password == "" {
		return nil, fmt.Errorf("cluster identifier, engine, master username and master password are all required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.CreateDBClusterInput{
		DBClusterIdentifier: aws.String(id),
		Engine:              aws.String(engine),
		MasterUsername:      aws.String(username),
		MasterUserPassword:  aws.String(password),
	}
	if v := awscommon.InputString("database_name", inputs); v != "" {
		in.DatabaseName = aws.String(v)
	}
	if v := awscommon.InputString("engine_version", inputs); v != "" {
		in.EngineVersion = aws.String(v)
	}
	if p, ok := awscommon.InputInt("port", inputs); ok {
		in.Port = aws.Int32(int32(p))
	}
	if v := awscommon.InputString("db_subnet_group_name", inputs); v != "" {
		in.DBSubnetGroupName = aws.String(v)
	}
	if ids := awscommon.InputStrings("vpc_security_group_ids", inputs); len(ids) > 0 {
		in.VpcSecurityGroupIds = ids
	}
	if awscommon.InputBool("storage_encrypted", inputs) {
		in.StorageEncrypted = aws.Bool(true)
	}
	if v := awscommon.InputString("kms_key_id", inputs); v != "" {
		in.KmsKeyId = aws.String(v)
	}
	if n, ok := awscommon.InputInt("backup_retention_period", inputs); ok {
		in.BackupRetentionPeriod = aws.Int32(int32(n))
	}
	if v := awscommon.InputString("preferred_backup_window", inputs); v != "" {
		in.PreferredBackupWindow = aws.String(v)
	}
	if awscommon.InputBool("deletion_protection", inputs) {
		in.DeletionProtection = aws.Bool(true)
	}
	if sv2 := rdscat.BuildServerlessV2(inputs); sv2 != nil {
		in.ServerlessV2ScalingConfiguration = sv2
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.Tags = tags
	}

	out, err := client.CreateDBCluster(ctx, in)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Creating DB cluster %q (%s, status: %s)", id, engine, aws.ToString(out.DBCluster.Status)),
		"cluster":     rdscat.SummariseCluster(out.DBCluster),
	}, nil
}

func buildTags(inputs []*core.Connection) []rdstypes.Tag {
	conn := core.FindConnection("tags", inputs)
	if conn == nil {
		return nil
	}
	var tags []rdstypes.Tag
	for _, kv := range conn.KeyValuePairs() {
		k := strings.TrimSpace(kv.Key)
		if k == "" {
			continue
		}
		tags = append(tags, rdstypes.Tag{Key: aws.String(k), Value: aws.String(kv.Value)})
	}
	return tags
}
