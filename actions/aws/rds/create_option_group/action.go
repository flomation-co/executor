// Package aws_rds_create_option_group creates an RDS option group for a database
// engine and major version.
package aws_rds_create_option_group

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
	Name         = "AWS RDS Create Option Group"
	Description  = "Create an RDS option group for a database engine and major version."
	Website      = "https://www.flomation.co"
	Icon         = "layer-group+plus"
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
	{Name: "option_group_name", Type: core.ConnectionTypeString, Label: "Option Group Name", Placeholder: "my-option-group", Required: true},
	{Name: "engine_name", Type: core.ConnectionTypeString, Label: "Engine Name", Placeholder: "mysql", Required: true},
	{Name: "major_engine_version", Type: core.ConnectionTypeString, Label: "Major Engine Version", Placeholder: "8.0", Required: true},
	{Name: "option_group_description", Type: core.ConnectionTypeString, Label: "Description", Required: true},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "option_group", Type: core.ConnectionTypeObject, Label: "Option Group"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := awscommon.InputString("option_group_name", inputs)
	engine := awscommon.InputString("engine_name", inputs)
	version := awscommon.InputString("major_engine_version", inputs)
	description := awscommon.InputString("option_group_description", inputs)
	if name == "" || engine == "" || version == "" || description == "" {
		return nil, fmt.Errorf("option group name, engine name, major engine version and description are required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.CreateOptionGroupInput{
		OptionGroupName:        aws.String(name),
		EngineName:             aws.String(engine),
		MajorEngineVersion:     aws.String(version),
		OptionGroupDescription: aws.String(description),
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.Tags = tags
	}

	out, err := client.CreateOptionGroup(ctx, in)
	if err != nil {
		return nil, err
	}

	og := out.OptionGroup
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created option group %q for %s %s", aws.ToString(og.OptionGroupName), aws.ToString(og.EngineName), aws.ToString(og.MajorEngineVersion)),
		"option_group": map[string]interface{}{
			"name":                 aws.ToString(og.OptionGroupName),
			"engine":               aws.ToString(og.EngineName),
			"major_engine_version": aws.ToString(og.MajorEngineVersion),
			"description":          aws.ToString(og.OptionGroupDescription),
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
