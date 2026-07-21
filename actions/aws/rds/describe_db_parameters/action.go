// Package aws_rds_describe_db_parameters lists the parameters in an RDS DB parameter group.
package aws_rds_describe_db_parameters

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
	Name         = "AWS RDS Describe DB Parameters"
	Description  = "List the parameters within an RDS DB parameter group."
	Website      = "https://www.flomation.co"
	Icon         = "gear+list"
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
	{Name: "db_parameter_group_name", Type: core.ConnectionTypeString, Label: "Parameter Group Name", Placeholder: "my-pg-postgres16", Required: true},
	{Name: "source", Type: core.ConnectionTypeString, Label: "Source (optional)", Options: []core.ConnectionOption{
		{Name: "All", Value: ""},
		{Name: "User", Value: "user"},
		{Name: "System", Value: "system"},
		{Name: "Engine Default", Value: "engine-default"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "parameters", Type: core.ConnectionTypeObject, Label: "Parameters"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := awscommon.InputString("db_parameter_group_name", inputs)
	if name == "" {
		return nil, fmt.Errorf("parameter group name is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.DescribeDBParametersInput{DBParameterGroupName: aws.String(name)}
	if source := awscommon.InputString("source", inputs); source != "" {
		in.Source = aws.String(source)
	}

	var params []map[string]interface{}
	paginator := rds.NewDescribeDBParametersPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for i := range page.Parameters {
			p := &page.Parameters[i]
			params = append(params, map[string]interface{}{
				"name":           aws.ToString(p.ParameterName),
				"value":          aws.ToString(p.ParameterValue),
				"apply_type":     aws.ToString(p.ApplyType),
				"data_type":      aws.ToString(p.DataType),
				"allowed_values": aws.ToString(p.AllowedValues),
				"is_modifiable":  aws.ToBool(p.IsModifiable),
			})
		}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d parameter(s) in DB parameter group %q", len(params), name),
		"parameters":  params,
		"count":       len(params),
	}, nil
}
