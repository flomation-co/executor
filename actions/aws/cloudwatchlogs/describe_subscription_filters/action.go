// Package aws_cloudwatchlogs_describe_subscription_filters lists CloudWatch Logs subscription filters.
package aws_cloudwatchlogs_describe_subscription_filters

import (
	"context"
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	cwlogs "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS CloudWatch Logs Describe Subscription Filters"
	Description  = "List the subscription filters attached to a CloudWatch Logs log group."
	Website      = "https://www.flomation.co"
	Icon         = "file-lines+magnifying-glass"
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
	{Name: "filter_name_prefix", Type: core.ConnectionTypeString, Label: "Filter Name Prefix (optional)", Placeholder: "ship-logs"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "subscription_filters", Type: core.ConnectionTypeString, Label: "Subscription Filters (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

type subscriptionFilterInfo struct {
	FilterName     string `json:"filter_name"`
	FilterPattern  string `json:"filter_pattern"`
	DestinationARN string `json:"destination_arn"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	logGroup := awscommon.InputString("log_group_name", inputs)
	if logGroup == "" {
		return nil, fmt.Errorf("log group name is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := cwlogs.NewFromConfig(cfg)

	in := &cwlogs.DescribeSubscriptionFiltersInput{
		LogGroupName: aws.String(logGroup),
	}
	if prefix := awscommon.InputString("filter_name_prefix", inputs); prefix != "" {
		in.FilterNamePrefix = aws.String(prefix)
	}

	var filters []subscriptionFilterInfo

	paginator := cwlogs.NewDescribeSubscriptionFiltersPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, f := range page.SubscriptionFilters {
			filters = append(filters, subscriptionFilterInfo{
				FilterName:     aws.ToString(f.FilterName),
				FilterPattern:  aws.ToString(f.FilterPattern),
				DestinationARN: aws.ToString(f.DestinationArn),
			})
		}
	}

	filtersJSON := "[]"
	if b, mErr := json.Marshal(filters); mErr == nil {
		filtersJSON = string(b)
	}

	return map[string]interface{}{
		"tool_result":          fmt.Sprintf("Found %d subscription filter(s) on %s", len(filters), logGroup),
		"subscription_filters": filtersJSON,
		"count":                len(filters),
	}, nil
}
