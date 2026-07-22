// Package aws_route53_list_query_logging_configs lists Route 53 DNS query
// logging configurations.
package aws_route53_list_query_logging_configs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Route 53 List Query Logging"
	Description  = "List Route 53 DNS query logging configurations, optionally filtered by zone."
	Website      = "https://www.flomation.co"
	Icon         = "route+list"
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
	{Name: "hosted_zone_id", Type: core.ConnectionTypeString, Label: "Hosted Zone ID (optional filter)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "configs", Type: core.ConnectionTypeString, Label: "Configs (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

type configOut struct {
	ID                        string `json:"id"`
	HostedZoneID              string `json:"hosted_zone_id"`
	CloudWatchLogsLogGroupArn string `json:"cloud_watch_logs_log_group_arn"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	in := &route53.ListQueryLoggingConfigsInput{}
	if v := strings.TrimSpace(awscommon.InputString("hosted_zone_id", inputs)); v != "" {
		in.HostedZoneId = aws.String(v)
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := route53.NewFromConfig(cfg)

	out, err := client.ListQueryLoggingConfigs(ctx, in)
	if err != nil {
		return nil, err
	}

	configs := make([]configOut, 0, len(out.QueryLoggingConfigs))
	for _, c := range out.QueryLoggingConfigs {
		configs = append(configs, configOut{
			ID:                        aws.ToString(c.Id),
			HostedZoneID:              aws.ToString(c.HostedZoneId),
			CloudWatchLogsLogGroupArn: aws.ToString(c.CloudWatchLogsLogGroupArn),
		})
	}

	jsonBytes, err := json.Marshal(configs)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d query logging config(s)", len(configs)),
		"configs":     string(jsonBytes),
		"count":       len(configs),
	}, nil
}
