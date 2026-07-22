// Package aws_cloudwatchlogs_get_data_protection_policy retrieves the data protection policy of a CloudWatch Logs log group.
package aws_cloudwatchlogs_get_data_protection_policy

import (
	"context"
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS CloudWatch Get Data Protection Policy"
	Description  = "Retrieve the data protection policy attached to a log group."
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
	{Name: "log_group_identifier", Type: core.ConnectionTypeString, Label: "Log Group Name or ARN", Placeholder: "/flomation/app", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "policy_document", Type: core.ConnectionTypeString, Label: "Policy Document"},
	{Name: "last_updated", Type: core.ConnectionTypeString, Label: "Last Updated"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	logGroup := awscommon.InputString("log_group_identifier", inputs)
	if logGroup == "" {
		return nil, fmt.Errorf("log group identifier is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := cloudwatchlogs.NewFromConfig(cfg)

	out, err := client.GetDataProtectionPolicy(ctx, &cloudwatchlogs.GetDataProtectionPolicyInput{
		LogGroupIdentifier: aws.String(logGroup),
	})
	if err != nil {
		return nil, err
	}

	policyDocument := ""
	if out.PolicyDocument != nil {
		policyDocument = *out.PolicyDocument
	}
	lastUpdated := ""
	if out.LastUpdatedTime != nil {
		lastUpdated = time.UnixMilli(*out.LastUpdatedTime).UTC().Format(time.RFC3339)
	}

	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Retrieved data protection policy for %s", logGroup),
		"policy_document": policyDocument,
		"last_updated":    lastUpdated,
	}, nil
}
