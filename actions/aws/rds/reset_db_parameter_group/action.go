// Package aws_rds_reset_db_parameter_group resets parameters in an RDS DB parameter group to defaults.
package aws_rds_reset_db_parameter_group

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
	Name         = "AWS RDS Reset DB Parameter Group"
	Description  = "Reset parameters in an RDS DB parameter group to their default values."
	Website      = "https://www.flomation.co"
	Icon         = "gear+rotate"
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
	{Name: "reset_all_parameters", Type: core.ConnectionTypeBoolean, Label: "Reset All Parameters"},
	{Name: "parameters", Type: core.ConnectionTypeKeyValueArray, Label: "Parameters to Reset (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
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

	in := &rds.ResetDBParameterGroupInput{DBParameterGroupName: aws.String(name)}

	resetAll := awscommon.InputBool("reset_all_parameters", inputs)
	if resetAll {
		in.ResetAllParameters = aws.Bool(true)
	} else {
		params := buildParameters(inputs)
		if len(params) == 0 {
			return nil, fmt.Errorf("either enable reset all parameters or supply at least one parameter to reset")
		}
		in.Parameters = params
	}

	_, err = client.ResetDBParameterGroup(ctx, in)
	if err != nil {
		return nil, err
	}

	summary := fmt.Sprintf("Reset selected parameters in DB parameter group %q", name)
	if resetAll {
		summary = fmt.Sprintf("Reset all parameters in DB parameter group %q to defaults", name)
	}

	return map[string]interface{}{
		"tool_result": summary,
	}, nil
}

func buildParameters(inputs []*core.Connection) []rdstypes.Parameter {
	conn := core.FindConnection("parameters", inputs)
	if conn == nil {
		return nil
	}
	var params []rdstypes.Parameter
	for _, kv := range conn.KeyValuePairs() {
		k := strings.TrimSpace(kv.Key)
		if k == "" {
			continue
		}
		params = append(params, rdstypes.Parameter{
			ParameterName:  aws.String(k),
			ParameterValue: aws.String(kv.Value),
			ApplyMethod:    rdstypes.ApplyMethodImmediate,
		})
	}
	return params
}
