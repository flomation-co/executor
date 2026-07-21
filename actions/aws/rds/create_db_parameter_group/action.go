// Package aws_rds_create_db_parameter_group creates an RDS DB parameter group.
package aws_rds_create_db_parameter_group

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
	Name         = "AWS RDS Create DB Parameter Group"
	Description  = "Create an RDS DB parameter group for a database engine family."
	Website      = "https://www.flomation.co"
	Icon         = "gear+plus"
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
	{Name: "db_parameter_group_family", Type: core.ConnectionTypeString, Label: "Parameter Group Family", Placeholder: "postgres16", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "Custom parameters for my database", Required: true},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "parameter_group", Type: core.ConnectionTypeObject, Label: "DB Parameter Group"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := awscommon.InputString("db_parameter_group_name", inputs)
	family := awscommon.InputString("db_parameter_group_family", inputs)
	description := awscommon.InputString("description", inputs)
	if name == "" || family == "" || description == "" {
		return nil, fmt.Errorf("parameter group name, family and description are all required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.CreateDBParameterGroupInput{
		DBParameterGroupName:   aws.String(name),
		DBParameterGroupFamily: aws.String(family),
		Description:            aws.String(description),
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.Tags = tags
	}

	out, err := client.CreateDBParameterGroup(ctx, in)
	if err != nil {
		return nil, err
	}

	pg := out.DBParameterGroup
	group := map[string]interface{}{
		"name":        aws.ToString(pg.DBParameterGroupName),
		"family":      aws.ToString(pg.DBParameterGroupFamily),
		"description": aws.ToString(pg.Description),
		"arn":         aws.ToString(pg.DBParameterGroupArn),
	}

	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Created DB parameter group %q (family: %s)", name, family),
		"parameter_group": group,
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
