// Package aws_route53_get_query_logging_config retrieves a Route 53 DNS query logging configuration.
package aws_route53_get_query_logging_config

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
	Name         = "AWS Route 53 Get Query Logging Config"
	Description  = "Retrieve a Route 53 DNS query logging configuration."
	Website      = "https://www.flomation.co"
	Icon         = "route+magnifying-glass"
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
	{Name: "query_logging_config_id", Type: core.ConnectionTypeString, Label: "Query Logging Config ID", Placeholder: "0123456789abcdef", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "hosted_zone_id", Type: core.ConnectionTypeString, Label: "Hosted Zone ID"},
	{Name: "cloud_watch_logs_log_group_arn", Type: core.ConnectionTypeString, Label: "CloudWatch Logs Log Group ARN"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	configID := awscommon.InputString("query_logging_config_id", inputs)
	if configID == "" {
		return nil, fmt.Errorf("query logging config id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := route53.NewFromConfig(cfg)

	out, err := client.GetQueryLoggingConfig(ctx, &route53.GetQueryLoggingConfigInput{
		Id: aws.String(configID),
	})
	if err != nil {
		return nil, err
	}

	var zoneID, logGroupARN string
	if out.QueryLoggingConfig != nil {
		zoneID = aws.ToString(out.QueryLoggingConfig.HostedZoneId)
		logGroupARN = aws.ToString(out.QueryLoggingConfig.CloudWatchLogsLogGroupArn)
	}

	return map[string]interface{}{
		"tool_result":                    fmt.Sprintf("Query logging config %s logs hosted zone %s to %s", configID, zoneID, logGroupARN),
		"hosted_zone_id":                 zoneID,
		"cloud_watch_logs_log_group_arn": logGroupARN,
	}, nil
}
