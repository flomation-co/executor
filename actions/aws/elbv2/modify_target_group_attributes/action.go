// Package aws_elbv2_modify_target_group_attributes sets ELBv2 target group attributes.
package aws_elbv2_modify_target_group_attributes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Modify Target Group Attributes"
	Description  = "Set attributes on an Elastic Load Balancing target group."
	Website      = "https://www.flomation.co"
	Icon         = "diagram-project+list"
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
	{Name: "target_group_arn", Type: core.ConnectionTypeString, Label: "Target Group ARN", Placeholder: "arn:aws:elasticloadbalancing:...:targetgroup/...", Required: true},
	{Name: "attributes", Type: core.ConnectionTypeString, Label: "Attributes", Placeholder: `[{"Key":"deregistration_delay.timeout_seconds","Value":"30"}]`, Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "target_group_arn", Type: core.ConnectionTypeString, Label: "Target Group ARN"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	arn := awscommon.InputString("target_group_arn", inputs)
	if arn == "" {
		return nil, fmt.Errorf("target group arn is required")
	}

	raw := strings.TrimSpace(awscommon.InputString("attributes", inputs))
	if raw == "" {
		return nil, fmt.Errorf("attributes are required")
	}
	var entries []struct {
		Key   string `json:"Key"`
		Value string `json:"Value"`
	}
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return nil, fmt.Errorf("invalid attributes JSON: %w", err)
	}
	var attrs []elbv2types.TargetGroupAttribute
	for _, e := range entries {
		if strings.TrimSpace(e.Key) == "" {
			continue
		}
		attrs = append(attrs, elbv2types.TargetGroupAttribute{
			Key:   aws.String(e.Key),
			Value: aws.String(e.Value),
		})
	}
	if len(attrs) == 0 {
		return nil, fmt.Errorf("at least one attribute is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := elasticloadbalancingv2.NewFromConfig(cfg)

	_, err = client.ModifyTargetGroupAttributes(ctx, &elasticloadbalancingv2.ModifyTargetGroupAttributesInput{
		TargetGroupArn: aws.String(arn),
		Attributes:     attrs,
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result":      fmt.Sprintf("Set %d attribute(s) on %s", len(attrs), arn),
		"target_group_arn": arn,
	}, nil
}
