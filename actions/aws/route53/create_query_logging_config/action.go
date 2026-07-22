// Package aws_route53_create_query_logging_config enables DNS query logging for
// a Route 53 public hosted zone.
package aws_route53_create_query_logging_config

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Route 53 Create Query Logging"
	Description  = "Enable DNS query logging to CloudWatch for a public hosted zone."
	Website      = "https://www.flomation.co"
	Icon         = "route+plus"
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
	{Name: "hosted_zone_id", Type: core.ConnectionTypeString, Label: "Hosted Zone ID", Placeholder: "Z1234567890ABC", Required: true},
	{Name: "cloud_watch_logs_log_group_arn", Type: core.ConnectionTypeString, Label: "CloudWatch Logs Log Group ARN", Placeholder: "arn:aws:logs:us-east-1:123456789012:log-group:/route53/example", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "query_logging_config_id", Type: core.ConnectionTypeString, Label: "Query Logging Config ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	zoneID := awscommon.InputString("hosted_zone_id", inputs)
	if zoneID == "" {
		return nil, fmt.Errorf("hosted zone id is required")
	}
	logGroupARN := awscommon.InputString("cloud_watch_logs_log_group_arn", inputs)
	if logGroupARN == "" {
		return nil, fmt.Errorf("cloud watch logs log group arn is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := route53.NewFromConfig(cfg)

	out, err := client.CreateQueryLoggingConfig(ctx, &route53.CreateQueryLoggingConfigInput{
		HostedZoneId:              aws.String(zoneID),
		CloudWatchLogsLogGroupArn: aws.String(logGroupARN),
	})
	if err != nil {
		return nil, err
	}

	configID := ""
	if out.QueryLoggingConfig != nil {
		configID = aws.ToString(out.QueryLoggingConfig.Id)
	}

	return map[string]interface{}{
		"tool_result":             fmt.Sprintf("Enabled query logging for %s (config %s)", zoneID, configID),
		"query_logging_config_id": configID,
	}, nil
}
