// Package aws_rds_describe_option_groups lists RDS option groups, optionally
// narrowed to a single name or engine.
package aws_rds_describe_option_groups

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
	Name         = "AWS RDS Describe Option Groups"
	Description  = "List RDS option groups, optionally filtered by name or engine."
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
	{Name: "option_group_name", Type: core.ConnectionTypeString, Label: "Option Group Name (optional)", Placeholder: "Leave blank to list all"},
	{Name: "engine_name", Type: core.ConnectionTypeString, Label: "Engine Name (optional)", Placeholder: "mysql"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "option_groups", Type: core.ConnectionTypeObject, Label: "Option Groups"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.DescribeOptionGroupsInput{}
	if name := awscommon.InputString("option_group_name", inputs); name != "" {
		in.OptionGroupName = aws.String(name)
	}
	if engine := awscommon.InputString("engine_name", inputs); engine != "" {
		in.EngineName = aws.String(engine)
	}

	var groups []map[string]interface{}
	paginator := rds.NewDescribeOptionGroupsPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for i := range page.OptionGroupsList {
			og := &page.OptionGroupsList[i]
			groups = append(groups, map[string]interface{}{
				"name":                 aws.ToString(og.OptionGroupName),
				"engine":               aws.ToString(og.EngineName),
				"major_engine_version": aws.ToString(og.MajorEngineVersion),
				"description":          aws.ToString(og.OptionGroupDescription),
			})
		}
	}

	return map[string]interface{}{
		"tool_result":   fmt.Sprintf("Found %d option group(s)", len(groups)),
		"option_groups": groups,
		"count":         len(groups),
	}, nil
}
