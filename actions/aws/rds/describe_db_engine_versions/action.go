// Package aws_rds_describe_db_engine_versions lists the RDS database engine
// versions available, optionally filtered.
package aws_rds_describe_db_engine_versions

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
	Name         = "AWS RDS Describe DB Engine Versions"
	Description  = "List available RDS database engine versions, optionally filtered."
	Website      = "https://www.flomation.co"
	Icon         = "layer-group+magnifying-glass"
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
	{Name: "engine", Type: core.ConnectionTypeString, Label: "Engine (optional)", Placeholder: "e.g. postgres"},
	{Name: "engine_version", Type: core.ConnectionTypeString, Label: "Engine Version (optional)", Placeholder: "e.g. 16.3"},
	{Name: "db_parameter_group_family", Type: core.ConnectionTypeString, Label: "DB Parameter Group Family (optional)", Placeholder: "e.g. postgres16"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "engine_versions", Type: core.ConnectionTypeObject, Label: "Engine Versions"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.DescribeDBEngineVersionsInput{}
	if e := awscommon.InputString("engine", inputs); e != "" {
		in.Engine = aws.String(e)
	}
	if v := awscommon.InputString("engine_version", inputs); v != "" {
		in.EngineVersion = aws.String(v)
	}
	if f := awscommon.InputString("db_parameter_group_family", inputs); f != "" {
		in.DBParameterGroupFamily = aws.String(f)
	}

	var versions []map[string]interface{}
	paginator := rds.NewDescribeDBEngineVersionsPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for i := range page.DBEngineVersions {
			v := &page.DBEngineVersions[i]
			versions = append(versions, map[string]interface{}{
				"engine":                    aws.ToString(v.Engine),
				"engine_version":            aws.ToString(v.EngineVersion),
				"db_engine_description":     aws.ToString(v.DBEngineDescription),
				"db_parameter_group_family": aws.ToString(v.DBParameterGroupFamily),
			})
		}
	}

	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Found %d engine version(s)", len(versions)),
		"engine_versions": versions,
		"count":           len(versions),
	}, nil
}
