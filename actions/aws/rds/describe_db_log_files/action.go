// Package aws_rds_describe_db_log_files lists the log files available for an RDS
// DB instance.
package aws_rds_describe_db_log_files

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
	Name         = "AWS RDS Describe DB Log Files"
	Description  = "List the log files available for an RDS DB instance."
	Website      = "https://www.flomation.co"
	Icon         = "file-lines+magnifying-glass"
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
	{Name: "db_instance_identifier", Type: core.ConnectionTypeString, Label: "DB Instance Identifier", Placeholder: "my-database", Required: true},
	{Name: "filename_contains", Type: core.ConnectionTypeString, Label: "Filename Contains (optional)", Placeholder: "e.g. error"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "log_files", Type: core.ConnectionTypeObject, Label: "Log Files"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	id := awscommon.InputString("db_instance_identifier", inputs)
	if id == "" {
		return nil, fmt.Errorf("db instance identifier is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.DescribeDBLogFilesInput{DBInstanceIdentifier: aws.String(id)}
	if fc := awscommon.InputString("filename_contains", inputs); fc != "" {
		in.FilenameContains = aws.String(fc)
	}

	var logFiles []map[string]interface{}
	paginator := rds.NewDescribeDBLogFilesPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for i := range page.DescribeDBLogFiles {
			f := &page.DescribeDBLogFiles[i]
			logFiles = append(logFiles, map[string]interface{}{
				"log_file_name": aws.ToString(f.LogFileName),
				"size":          aws.ToInt64(f.Size),
				"last_written":  aws.ToInt64(f.LastWritten),
			})
		}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d log file(s) for %q", len(logFiles), id),
		"log_files":   logFiles,
		"count":       len(logFiles),
	}, nil
}
