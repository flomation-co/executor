// Package aws_cloudwatchlogs_put_subscription_filter creates or updates a CloudWatch Logs subscription filter.
package aws_cloudwatchlogs_put_subscription_filter

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS CloudWatch Put Subscription Filter"
	Description  = "Stream matching log events from a log group to a destination (Lambda, Kinesis, Firehose)."
	Website      = "https://www.flomation.co"
	Icon         = "file-lines+link"
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
	{Name: "log_group_name", Type: core.ConnectionTypeString, Label: "Log Group Name", Placeholder: "/flomation/app", Required: true},
	{Name: "filter_name", Type: core.ConnectionTypeString, Label: "Filter Name", Placeholder: "ForwardErrors", Required: true},
	{Name: "filter_pattern", Type: core.ConnectionTypeString, Label: "Filter Pattern", Placeholder: "ERROR (empty matches all)", Required: true},
	{Name: "destination_arn", Type: core.ConnectionTypeString, Label: "Destination ARN", Placeholder: "arn:aws:lambda:eu-west-2:...:function:...", Required: true},
	{Name: "role_arn", Type: core.ConnectionTypeString, Label: "Role ARN (optional)", Placeholder: "arn:aws:iam::...:role/..."},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "filter_name", Type: core.ConnectionTypeString, Label: "Filter Name"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	logGroup := awscommon.InputString("log_group_name", inputs)
	if logGroup == "" {
		return nil, fmt.Errorf("log group name is required")
	}
	filterName := awscommon.InputString("filter_name", inputs)
	if filterName == "" {
		return nil, fmt.Errorf("filter name is required")
	}
	filterPattern := awscommon.InputString("filter_pattern", inputs)
	if filterPattern == "" {
		return nil, fmt.Errorf("filter pattern is required")
	}
	destinationARN := awscommon.InputString("destination_arn", inputs)
	if destinationARN == "" {
		return nil, fmt.Errorf("destination ARN is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := cloudwatchlogs.NewFromConfig(cfg)

	in := &cloudwatchlogs.PutSubscriptionFilterInput{
		LogGroupName:   aws.String(logGroup),
		FilterName:     aws.String(filterName),
		FilterPattern:  aws.String(filterPattern),
		DestinationArn: aws.String(destinationARN),
	}
	if r := awscommon.InputString("role_arn", inputs); r != "" {
		in.RoleArn = aws.String(r)
	}

	_, err = client.PutSubscriptionFilter(ctx, in)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Put subscription filter %s on %s → %s", filterName, logGroup, destinationARN),
		"filter_name": filterName,
	}, nil
}
