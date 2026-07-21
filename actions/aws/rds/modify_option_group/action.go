// Package aws_rds_modify_option_group adds or removes options in an RDS option
// group.
package aws_rds_modify_option_group

import (
	"context"
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Modify Option Group"
	Description  = "Add or remove options in an RDS option group."
	Website      = "https://www.flomation.co"
	Icon         = "layer-group+pen"
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
	{Name: "option_group_name", Type: core.ConnectionTypeString, Label: "Option Group Name", Placeholder: "my-option-group", Required: true},
	{Name: "options_to_include", Type: core.ConnectionTypeString, Label: "Options to Include (JSON)", Placeholder: `[{"option_name":"OEM"}]`},
	{Name: "options_to_remove", Type: core.ConnectionTypeString, Label: "Options to Remove", Placeholder: "Comma-separated option names"},
	{Name: "apply_immediately", Type: core.ConnectionTypeBoolean, Label: "Apply Immediately"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "option_group", Type: core.ConnectionTypeObject, Label: "Option Group"},
}

type optionInclude struct {
	OptionName string `json:"option_name"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := awscommon.InputString("option_group_name", inputs)
	if name == "" {
		return nil, fmt.Errorf("option group name is required")
	}

	var include []rdstypes.OptionConfiguration
	if raw := awscommon.InputString("options_to_include", inputs); raw != "" {
		var parsed []optionInclude
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			return nil, fmt.Errorf("options to include must be a JSON array of objects: %w", err)
		}
		for _, p := range parsed {
			if p.OptionName == "" {
				return nil, fmt.Errorf("each option to include must have an option_name")
			}
			include = append(include, rdstypes.OptionConfiguration{OptionName: aws.String(p.OptionName)})
		}
	}

	remove := awscommon.InputStrings("options_to_remove", inputs)

	if len(include) == 0 && len(remove) == 0 {
		return nil, fmt.Errorf("at least one option to include or remove is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.ModifyOptionGroupInput{
		OptionGroupName:  aws.String(name),
		ApplyImmediately: aws.Bool(awscommon.InputBool("apply_immediately", inputs)),
	}
	if len(include) > 0 {
		in.OptionsToInclude = include
	}
	if len(remove) > 0 {
		in.OptionsToRemove = remove
	}

	out, err := client.ModifyOptionGroup(ctx, in)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Modified option group %q (+%d/-%d options)", name, len(include), len(remove)),
		"option_group": flattenOptionGroup(out.OptionGroup),
	}, nil
}

func flattenOptionGroup(g *rdstypes.OptionGroup) map[string]interface{} {
	if g == nil {
		return nil
	}
	var options []map[string]interface{}
	for _, o := range g.Options {
		options = append(options, map[string]interface{}{
			"option_name": aws.ToString(o.OptionName),
		})
	}
	return map[string]interface{}{
		"option_group_name":    aws.ToString(g.OptionGroupName),
		"engine_name":          aws.ToString(g.EngineName),
		"major_engine_version": aws.ToString(g.MajorEngineVersion),
		"options":              options,
	}
}
