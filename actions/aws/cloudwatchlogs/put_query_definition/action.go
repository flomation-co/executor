// Package aws_cloudwatchlogs_put_query_definition creates or updates a CloudWatch Logs Insights saved query definition.
package aws_cloudwatchlogs_put_query_definition

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS CloudWatch Put Query Definition"
	Description  = "Create or update a saved CloudWatch Logs Insights query definition."
	Website      = "https://www.flomation.co"
	Icon         = "file-lines+plus"
	Date         = "22/07/2026"
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
	{Name: "name", Type: core.ConnectionTypeString, Label: "Query Name", Placeholder: "Errors by service", Required: true},
	{Name: "query_string", Type: core.ConnectionTypeString, Label: "Query String", Placeholder: "fields @timestamp, @message | filter @message like /ERROR/", Required: true},
	{Name: "log_group_names", Type: core.ConnectionTypeString, Label: "Log Group Names (comma-separated, optional)", Placeholder: "/flomation/app,/flomation/api"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "query_definition_id", Type: core.ConnectionTypeString, Label: "Query Definition ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := awscommon.InputString("name", inputs)
	if name == "" {
		return nil, fmt.Errorf("query name is required")
	}
	queryString := awscommon.InputString("query_string", inputs)
	if strings.TrimSpace(queryString) == "" {
		return nil, fmt.Errorf("query string is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := cloudwatchlogs.NewFromConfig(cfg)

	in := &cloudwatchlogs.PutQueryDefinitionInput{
		Name:        aws.String(name),
		QueryString: aws.String(queryString),
	}
	if raw := awscommon.InputString("log_group_names", inputs); strings.TrimSpace(raw) != "" {
		var groups []string
		for _, g := range strings.Split(raw, ",") {
			if trimmed := strings.TrimSpace(g); trimmed != "" {
				groups = append(groups, trimmed)
			}
		}
		if len(groups) > 0 {
			in.LogGroupNames = groups
		}
	}

	out, err := client.PutQueryDefinition(ctx, in)
	if err != nil {
		return nil, err
	}

	queryID := ""
	if out.QueryDefinitionId != nil {
		queryID = *out.QueryDefinitionId
	}

	return map[string]interface{}{
		"tool_result":         fmt.Sprintf("Put query definition %s (%s)", name, queryID),
		"query_definition_id": queryID,
	}, nil
}
