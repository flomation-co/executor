// Package aws_route53_list_tags_for_resource lists tags on a Route 53 hosted
// zone or health check.
package aws_route53_list_tags_for_resource

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Route 53 List Tags"
	Description  = "List the tags on a Route 53 hosted zone or health check."
	Website      = "https://www.flomation.co"
	Icon         = "tag+magnifying-glass"
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
	{Name: "resource_type", Type: core.ConnectionTypeString, Label: "Resource Type", Required: true, Options: []core.ConnectionOption{
		{Name: "Hosted Zone", Value: "hostedzone"},
		{Name: "Health Check", Value: "healthcheck"},
	}},
	{Name: "resource_id", Type: core.ConnectionTypeString, Label: "Resource ID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Tags (JSON object)"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	resourceType := strings.TrimSpace(awscommon.InputString("resource_type", inputs))
	if resourceType == "" {
		return nil, fmt.Errorf("resource_type is required")
	}
	resourceID := awscommon.InputString("resource_id", inputs)
	if resourceID == "" {
		return nil, fmt.Errorf("resource_id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := route53.NewFromConfig(cfg)

	out, err := client.ListTagsForResource(ctx, &route53.ListTagsForResourceInput{
		ResourceType: r53types.TagResourceType(resourceType),
		ResourceId:   aws.String(resourceID),
	})
	if err != nil {
		return nil, err
	}

	tags := map[string]string{}
	if out.ResourceTagSet != nil {
		for _, t := range out.ResourceTagSet.Tags {
			tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
		}
	}

	jsonBytes, err := json.Marshal(tags)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d tag(s) on %s %s", len(tags), resourceType, resourceID),
		"tags":        string(jsonBytes),
	}, nil
}
