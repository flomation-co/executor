// Package aws_elbv2_modify_load_balancer_attributes updates the attributes of an
// Elastic Load Balancing v2 load balancer.
package aws_elbv2_modify_load_balancer_attributes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Modify Load Balancer Attributes"
	Description  = "Update the attributes of a load balancer from a JSON array of key/value pairs."
	Website      = "https://www.flomation.co"
	Icon         = "arrows-split-up-and-left+pen"
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
	{Name: "load_balancer_arn", Type: core.ConnectionTypeString, Label: "Load Balancer ARN", Placeholder: "arn:aws:elasticloadbalancing:...", Required: true},
	{Name: "attributes", Type: core.ConnectionTypeString, Label: "Attributes (JSON)", Placeholder: `[{"Key":"deletion_protection.enabled","Value":"true"}]`, Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "attributes", Type: core.ConnectionTypeObject, Label: "Attributes"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	arn := strings.TrimSpace(awscommon.InputString("load_balancer_arn", inputs))
	if arn == "" {
		return nil, fmt.Errorf("load balancer arn is required")
	}
	rawAttrs := strings.TrimSpace(awscommon.InputString("attributes", inputs))
	if rawAttrs == "" {
		return nil, fmt.Errorf("attributes is required")
	}

	var parsed []struct {
		Key   string `json:"Key"`
		Value string `json:"Value"`
	}
	if err := json.Unmarshal([]byte(rawAttrs), &parsed); err != nil {
		return nil, fmt.Errorf("attributes must be a JSON array of {Key, Value}: %w", err)
	}

	attrs := make([]elbv2types.LoadBalancerAttribute, 0, len(parsed))
	for _, a := range parsed {
		if strings.TrimSpace(a.Key) == "" {
			continue
		}
		attrs = append(attrs, elbv2types.LoadBalancerAttribute{
			Key:   aws.String(a.Key),
			Value: aws.String(a.Value),
		})
	}
	if len(attrs) == 0 {
		return nil, fmt.Errorf("no valid attributes supplied")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := elbv2.NewFromConfig(cfg)

	out, err := client.ModifyLoadBalancerAttributes(ctx, &elbv2.ModifyLoadBalancerAttributesInput{
		LoadBalancerArn: aws.String(arn),
		Attributes:      attrs,
	})
	if err != nil {
		return nil, err
	}

	attributes := map[string]string{}
	for _, a := range out.Attributes {
		attributes[aws.ToString(a.Key)] = aws.ToString(a.Value)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Updated %d attribute(s) for %s", len(attrs), arn),
		"attributes":  attributes,
	}, nil
}
