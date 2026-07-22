// Package aws_cloudwatch_tag_resource adds tags to a CloudWatch resource.
package aws_cloudwatch_tag_resource

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS CloudWatch Tag Resource"
	Description  = "Add key-value tags to a CloudWatch resource (alarm, etc)."
	Website      = "https://www.flomation.co"
	Icon         = "chart-line+tag"
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
	{Name: "resource_arn", Type: core.ConnectionTypeString, Label: "Resource ARN", Placeholder: "arn:aws:cloudwatch:eu-west-2:123456789012:alarm:high-cpu", Required: true},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags", Placeholder: "Add a Key and Value per tag"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "resource_arn", Type: core.ConnectionTypeString, Label: "Resource ARN"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	resourceARN := strings.TrimSpace(awscommon.InputString("resource_arn", inputs))
	if resourceARN == "" {
		return nil, fmt.Errorf("resource_arn is required")
	}
	tags := buildTags(inputs)
	if len(tags) == 0 {
		return nil, fmt.Errorf("at least one tag is required")
	}

	in := &cloudwatch.TagResourceInput{
		ResourceARN: aws.String(resourceARN),
		Tags:        tags,
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := cloudwatch.NewFromConfig(cfg)

	if _, err := client.TagResource(ctx, in); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Added %d tag(s) to %s", len(tags), resourceARN),
		"resource_arn": resourceARN,
	}, nil
}

func buildTags(inputs []*core.Connection) []cwtypes.Tag {
	conn := core.FindConnection("tags", inputs)
	if conn == nil {
		return nil
	}
	var tags []cwtypes.Tag
	for _, kv := range conn.KeyValuePairs() {
		key := strings.TrimSpace(kv.Key)
		if key == "" {
			continue
		}
		tags = append(tags, cwtypes.Tag{Key: aws.String(key), Value: aws.String(strings.TrimSpace(kv.Value))})
	}
	return tags
}
