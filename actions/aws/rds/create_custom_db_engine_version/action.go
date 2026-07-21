// Package aws_rds_create_custom_db_engine_version creates a custom RDS DB
// engine version (e.g. RDS Custom for Oracle) from installation media.
package aws_rds_create_custom_db_engine_version

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
	Name         = "AWS RDS Create Custom DB Engine Version"
	Description  = "Create a custom RDS DB engine version from installation media."
	Website      = "https://www.flomation.co"
	Icon         = "code-branch+plus"
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
	{Name: "engine", Type: core.ConnectionTypeString, Label: "Engine", Placeholder: "custom-oracle-ee", Required: true},
	{Name: "engine_version", Type: core.ConnectionTypeString, Label: "Engine Version", Placeholder: "19.cdb_cev1", Required: true},
	{Name: "database_installation_files_s3_bucket_name", Type: core.ConnectionTypeString, Label: "Installation Files S3 Bucket (optional)"},
	{Name: "database_installation_files_s3_prefix", Type: core.ConnectionTypeString, Label: "Installation Files S3 Prefix (optional)"},
	{Name: "kms_key_id", Type: core.ConnectionTypeString, Label: "KMS Key ID (optional)"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description (optional)"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "engine_version", Type: core.ConnectionTypeObject, Label: "Engine Version"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	engine := awscommon.InputString("engine", inputs)
	version := awscommon.InputString("engine_version", inputs)
	if engine == "" || version == "" {
		return nil, fmt.Errorf("both engine and engine version are required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.CreateCustomDBEngineVersionInput{
		Engine:        aws.String(engine),
		EngineVersion: aws.String(version),
	}
	if v := awscommon.InputString("database_installation_files_s3_bucket_name", inputs); v != "" {
		in.DatabaseInstallationFilesS3BucketName = aws.String(v)
	}
	if v := awscommon.InputString("database_installation_files_s3_prefix", inputs); v != "" {
		in.DatabaseInstallationFilesS3Prefix = aws.String(v)
	}
	if v := awscommon.InputString("kms_key_id", inputs); v != "" {
		in.KMSKeyId = aws.String(v)
	}
	if v := awscommon.InputString("description", inputs); v != "" {
		in.Description = aws.String(v)
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.Tags = tags
	}

	out, err := client.CreateCustomDBEngineVersion(ctx, in)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Creating custom engine version %s %s (status: %s)", aws.ToString(out.Engine), aws.ToString(out.EngineVersion), aws.ToString(out.Status)),
		"engine_version": map[string]interface{}{
			"engine":                aws.ToString(out.Engine),
			"engine_version":        aws.ToString(out.EngineVersion),
			"status":                aws.ToString(out.Status),
			"db_engine_version_arn": aws.ToString(out.DBEngineVersionArn),
		},
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
