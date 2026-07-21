// Package aws_elbv2_remove_tags removes tags from ELBv2 resources.
package aws_elbv2_remove_tags

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS ELBv2 Remove Tags"
	Description  = "Remove tags from Elastic Load Balancing v2 resources by key."
	Website      = "https://www.flomation.co"
	Icon         = "tag+trash"
	Date         = "21/07/2026"
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
	{Name: "resource_arns", Type: core.ConnectionTypeString, Label: "Resource ARNs", Placeholder: "Comma-separated ELBv2 ARNs", Required: true},
	{Name: "tag_keys", Type: core.ConnectionTypeString, Label: "Tag Keys", Placeholder: "Comma-separated tag keys to remove", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	resourceARNs := awscommon.InputStrings("resource_arns", inputs)
	if len(resourceARNs) == 0 {
		return nil, fmt.Errorf("at least one resource arn is required")
	}
	tagKeys := awscommon.InputStrings("tag_keys", inputs)
	if len(tagKeys) == 0 {
		return nil, fmt.Errorf("at least one tag key is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := elasticloadbalancingv2.NewFromConfig(cfg)

	_, err = client.RemoveTags(ctx, &elasticloadbalancingv2.RemoveTagsInput{
		ResourceArns: resourceARNs,
		TagKeys:      tagKeys,
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Removed %d tag key(s) from %d resource(s)", len(tagKeys), len(resourceARNs)),
	}, nil
}
